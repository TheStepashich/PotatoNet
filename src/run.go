package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"potatonet/src/platform"
)

// cappedBuffer хранит первые max байт вывода процесса, чтобы показать причину
// ошибки, не накапливая весь лог долгоживущего процесса в памяти.
type cappedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
	max int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	c.mu.Lock()
	defer c.mu.Unlock()
	if remain := c.max - c.buf.Len(); remain > 0 {
		if len(p) > remain {
			p = p[:remain]
		}
		c.buf.Write(p)
	}
	return n, nil
}

func (c *cappedBuffer) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.TrimSpace(c.buf.String())
}

const (
	defaultRPCPort      = 15888
	defaultListenerPort = 11010
)

var (
	runningMu sync.Mutex
	running   = map[string]int{} // имя сети -> pid процесса easytier-core

	statusMu sync.Mutex
	statuses = map[string]netStatus{} // имя сети -> состояние подключения
)

// netStatus — текущее состояние попытки запуска сети, чтобы UI мог показать
// «подключаюсь...» и текст ошибки вместо молчаливого бездействия.
type netStatus struct {
	Starting bool   `json:"starting"`
	Error    string `json:"error"`
}

func markStarting(name string) {
	statusMu.Lock()
	statuses[name] = netStatus{Starting: true}
	statusMu.Unlock()
}

func markFinished(name string, err error) {
	statusMu.Lock()
	defer statusMu.Unlock()
	if err != nil {
		statuses[name] = netStatus{Error: err.Error()}
		return
	}
	delete(statuses, name)
}

func clearStatus(name string) {
	statusMu.Lock()
	delete(statuses, name)
	statusMu.Unlock()
}

func getStatus(name string) netStatus {
	statusMu.Lock()
	defer statusMu.Unlock()
	return statuses[name]
}

// netPorts — пара портов, выделенных сети на время её работы.
type netPorts struct {
	Listener int
	RPC      int
}

var (
	portsMu  sync.Mutex
	portsMap = map[string]netPorts{} // имя сети -> занятые порты
)

func setPorts(name string, p netPorts) {
	portsMu.Lock()
	portsMap[name] = p
	portsMu.Unlock()
}

func getPorts(name string) netPorts {
	portsMu.Lock()
	defer portsMu.Unlock()
	return portsMap[name]
}

func removePorts(name string) {
	portsMu.Lock()
	delete(portsMap, name)
	portsMu.Unlock()
}

// freeUDPPort ищет свободный UDP-порт, начиная с base. Позволяет новому
// приложению не конфликтовать с классическим PotatoNet (тот занимает 11010+i).
func freeUDPPort(base int) int {
	for p := base; p < base+1000; p++ {
		pc, err := net.ListenPacket("udp", fmt.Sprintf("0.0.0.0:%d", p))
		if err == nil {
			_ = pc.Close()
			return p
		}
	}
	return 0
}

func freeTCPPort(base int) int {
	for p := base; p < base+1000; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			_ = ln.Close()
			return p
		}
	}
	return 0
}

func writePid(name string, pid int) {
	_ = os.WriteFile(filepath.Join(platform.PidDir(), name+".pid"), []byte(strconv.Itoa(pid)), 0o644)
}

func removePid(name string) {
	_ = os.Remove(filepath.Join(platform.PidDir(), name+".pid"))
}

func rpcFile(name string) string {
	return filepath.Join(platform.PidDir(), name+".rpc")
}

func writeRPC(name string, port int) {
	_ = os.WriteFile(rpcFile(name), []byte(strconv.Itoa(port)), 0o644)
}

func removeRPC(name string) {
	_ = os.Remove(rpcFile(name))
}

// readRPC нужен автономной команде `peers`: динамический порт сохраняется на
// диск, чтобы отдельный процесс мог найти RPC-портал работающей сети.
func readRPC(name string) int {
	b, err := os.ReadFile(rpcFile(name))
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return p
}

func isRunning(name string) bool {
	runningMu.Lock()
	defer runningMu.Unlock()
	_, ok := running[name]
	return ok
}

func runningNames() []string {
	runningMu.Lock()
	defer runningMu.Unlock()
	out := make([]string, 0, len(running))
	for n := range running {
		out = append(out, n)
	}
	return out
}

// startNetwork поднимает EasyTier для одной сети. Пользовательские настройки
// сведены к минимуму: имя, пароль, hostname. IP всегда выдаётся по DHCP,
// порты и bootstrap выбираются автоматически.
func startNetwork(cfg *Config, paths platform.Paths, net Network) error {
	if isRunning(net.Name) {
		return nil
	}
	if err := installEasyTier(paths); err != nil {
		return err
	}
	bootstrap := cfg.Bootstrap
	if len(bootstrap) == 0 {
		bootstrap = defaultBootstrap()
	}

	listenPort := freeUDPPort(defaultListenerPort)
	if listenPort == 0 {
		return fmt.Errorf("не найден свободный UDP-порт для сети")
	}
	rpcPort := freeTCPPort(defaultRPCPort)
	if rpcPort == 0 {
		return fmt.Errorf("не найден свободный RPC-порт для сети")
	}
	setPorts(net.Name, netPorts{Listener: listenPort, RPC: rpcPort})
	logf("[%s] ports: udp=%d rpc=%d", net.Name, listenPort, rpcPort)

	args := []string{
		"-d",
		"--listeners", fmt.Sprintf("udp://0.0.0.0:%d", listenPort),
	}
	for _, p := range bootstrap {
		args = append(args, "-p", p)
	}
	if net.Hostname != "" {
		args = append(args, "--hostname", net.Hostname)
	}
	args = append(args,
		"--network-name", net.Name,
		"--network-secret", net.Secret,
		"--rpc-portal", fmt.Sprintf("127.0.0.1:%d", rpcPort),
	)

	out := &cappedBuffer{max: 4096}
	cmd := exec.Command(paths.EasyTierCore, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	platform.SetupProcAttrs(cmd)
	if err := cmd.Start(); err != nil {
		removePorts(net.Name)
		return fmt.Errorf("не удалось запустить EasyTier: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.Wait() }()

	// Если процесс умирает сразу, показываем причину вместо тишины.
	select {
	case werr := <-errCh:
		removePid(net.Name)
		removePorts(net.Name)
		msg := strings.TrimSpace(out.String())
		if msg == "" {
			msg = werr.Error()
		}
		return fmt.Errorf("EasyTier не смог подключиться:\n%s", msg)
	case <-time.After(2 * time.Second):
	}

	runningMu.Lock()
	running[net.Name] = cmd.Process.Pid
	runningMu.Unlock()
	writePid(net.Name, cmd.Process.Pid)
	writeRPC(net.Name, rpcPort)
	logf("[%s] started (pid %d)", net.Name, cmd.Process.Pid)

	go func() {
		<-errCh
		runningMu.Lock()
		delete(running, net.Name)
		runningMu.Unlock()
		removePid(net.Name)
		removeRPC(net.Name)
		removePorts(net.Name)
	}()
	return nil
}

func startNetworks(cfg *Config, paths platform.Paths) error {
	for _, net := range cfg.Networks {
		if err := startNetwork(cfg, paths, net); err != nil {
			return err
		}
		time.Sleep(time.Second)
	}
	return nil
}

func stopNetwork(name string) {
	runningMu.Lock()
	pid, ok := running[name]
	runningMu.Unlock()
	if ok {
		platform.KillProcessGroup(pid)
		logf("[%s] stopped (pid %d)", name, pid)
	}
	removePid(name)
	removeRPC(name)
	clearStatus(name)
	removePorts(name)
}

func stopAll() {
	runningMu.Lock()
	names := make([]string, 0, len(running))
	for n := range running {
		names = append(names, n)
	}
	runningMu.Unlock()
	for _, n := range names {
		stopNetwork(n)
	}
	platform.KillAllByName()
	runningMu.Lock()
	running = map[string]int{}
	runningMu.Unlock()
}

func cmdStop(paths platform.Paths) {
	stopAll()
	logf("All networks stopped")
}

// Peer — запись об устройстве в сети (из easytier-cli -o json peer).
type Peer struct {
	IPv4     string `json:"ipv4"`
	Hostname string `json:"hostname"`
	Cost     string `json:"cost"`
	Latency  string `json:"latency"`
	Loss     string `json:"loss"`
	Rx       string `json:"rx"`
	Tx       string `json:"tx"`
	Tunnel   string `json:"tunnel"`
	NAT      string `json:"nat"`
}

type NetworkPeers struct {
	Connected bool   `json:"connected"`
	Starting  bool   `json:"starting"`
	Error     string `json:"error,omitempty"`
	Peers     []Peer `json:"peers"`
}

func peersForNetwork(paths platform.Paths, name string) NetworkPeers {
	res := NetworkPeers{Peers: []Peer{}}
	if !fileExists(paths.EasyTierCLI) {
		return res
	}
	rpc := getPorts(name).RPC
	if rpc == 0 {
		rpc = readRPC(name)
	}
	if rpc == 0 {
		return res
	}
	cmd := exec.Command(
		paths.EasyTierCLI,
		"-o", "json",
		"-p", fmt.Sprintf("127.0.0.1:%d", rpc),
		"peer",
	)
	platform.SetupProcAttrs(cmd)
	out, err := cmd.Output()
	if err != nil {
		return res
	}
	var raw []struct {
		IPv4     string `json:"ipv4"`
		Hostname string `json:"hostname"`
		Cost     string `json:"cost"`
		LatMs    string `json:"lat_ms"`
		LossRate string `json:"loss_rate"`
		RxBytes  string `json:"rx_bytes"`
		TxBytes  string `json:"tx_bytes"`
		Tunnel   string `json:"tunnel_proto"`
		NAT      string `json:"nat_type"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return res
	}
	for _, p := range raw {
		res.Peers = append(res.Peers, Peer{
			IPv4:     p.IPv4,
			Hostname: p.Hostname,
			Cost:     p.Cost,
			Latency:  p.LatMs,
			Loss:     p.LossRate,
			Rx:       p.RxBytes,
			Tx:       p.TxBytes,
			Tunnel:   p.Tunnel,
			NAT:      p.NAT,
		})
	}
	return res
}

func queryPeers(cfg *Config, paths platform.Paths) {
	for _, net := range cfg.Networks {
		np := peersForNetwork(paths, net.Name)
		logf("=== Peers for network '%s' ===", net.Name)
		for _, p := range np.Peers {
			logf("  %s\t%s\t%s\t%s", p.IPv4, p.Hostname, p.Cost, p.Latency)
		}
	}
}

func cmdPeers(paths platform.Paths) {
	cfg, err := loadConfig(paths)
	if err != nil {
		logf("Error: %v", err)
		return
	}
	queryPeers(cfg, paths)
}

func cmdRun(paths platform.Paths) error {
	if err := installEasyTier(paths); err != nil {
		return err
	}
	cfg, err := loadConfig(paths)
	if err != nil {
		return err
	}
	if err := startNetworks(cfg, paths); err != nil {
		return err
	}
	logf("All networks started. Press Ctrl+C to stop.")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	logf("Shutting down...")
	stopAll()
	return nil
}
