package backend

import (
	"fmt"
	"strings"
	"sync"

	"potatonet/src/platform"
)

// logBuffer — кольцевой буфер последних строк лога для отображения в UI.
type logBuffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func newLogBuffer(max int) *logBuffer {
	return &logBuffer{max: max}
}

func (b *logBuffer) add(msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, msg)
	if len(b.lines) > b.max {
		b.lines = b.lines[len(b.lines)-b.max:]
	}
}

func (b *logBuffer) snapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}

// Service — фасад над логикой VPN, доступный из UI (Wails-биндинги).
type Service struct {
	paths platform.Paths
	g     *gui
	logs  *logBuffer
}

// NewService загружает конфиг и подготоваливает сервис для GUI.
func NewService(paths platform.Paths) *Service {
	cfg, err := loadConfig(paths)
	if err != nil {
		cfg = defaultConfig()
	}
	s := &Service{
		paths: paths,
		g:     &gui{paths: paths, cfg: cfg},
		logs:  newLogBuffer(1000),
	}
	logf = func(format string, a ...any) {
		msg := fmt.Sprintf(format, a...)
		s.logs.add(msg)
		fmt.Println(msg)
	}
	return s
}

// State — снимок текущего состояния для UI.
type State struct {
	Networks   []Network `json:"networks"`
	Connected  []string  `json:"connected"`
	ConfigFile string    `json:"configFile"`
}

func (s *Service) GetState() State {
	s.g.mu.Lock()
	nets := append([]Network(nil), s.g.cfg.Networks...)
	s.g.mu.Unlock()
	if nets == nil {
		nets = []Network{}
	}
	return State{
		Networks:   nets,
		Connected:  runningNames(),
		ConfigFile: s.paths.ConfigFile,
	}
}

// SaveNetworks сохраняет список сетей и останавливает удалённые/изменённые.
func (s *Service) SaveNetworks(nets []Network) error {
	seen := map[string]bool{}
	for i, n := range nets {
		name := strings.TrimSpace(n.Name)
		if name == "" {
			return fmt.Errorf("имя сети обязательно")
		}
		if seen[name] {
			return fmt.Errorf("сеть с именем «%s» уже есть", name)
		}
		seen[name] = true
		nets[i].Name = name
		nets[i].Secret = strings.TrimSpace(n.Secret)
		nets[i].Hostname = strings.TrimSpace(n.Hostname)
	}

	s.g.mu.Lock()
	old := s.g.cfg.Networks
	s.g.cfg.Networks = nets
	cfg := s.g.cfg
	s.g.mu.Unlock()

	if err := saveConfig(cfg, s.paths); err != nil {
		return fmt.Errorf("не удалось сохранить конфиг: %w", err)
	}
	platform.ChownToUser(s.paths.LibDir)
	logf("Конфиг сохранён: %s", s.paths.ConfigFile)

	for _, o := range old {
		if !containsNetwork(nets, o) {
			stopNetwork(o.Name)
		}
	}
	return nil
}

// Connect запускает сеть в фоне (может занять время на скачивание EasyTier).
func (s *Service) Connect(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("имя сети обязательно")
	}
	s.g.mu.Lock()
	cfg := s.g.cfg
	_, net, found := findNetwork(cfg.Networks, name)
	s.g.mu.Unlock()
	if !found {
		return fmt.Errorf("сеть не найдена: %s", name)
	}
	markStarting(net.Name)
	go func() {
		err := startNetwork(cfg, s.paths, net)
		if err != nil {
			logf("Ошибка: %v", err)
		}
		markFinished(net.Name, err)
	}()
	return nil
}

// Disconnect останавливает сеть.
func (s *Service) Disconnect(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("имя сети обязательно")
	}
	stopNetwork(name)
	return nil
}

// GetPeers возвращает состояние и список устройств по каждой сети.
func (s *Service) GetPeers() map[string]NetworkPeers {
	s.g.mu.Lock()
	networks := append([]Network(nil), s.g.cfg.Networks...)
	s.g.mu.Unlock()

	res := map[string]NetworkPeers{}
	for _, n := range networks {
		np := peersForNetwork(s.paths, n.Name)
		np.Connected = isRunning(n.Name)
		st := getStatus(n.Name)
		np.Starting = st.Starting
		np.Error = st.Error
		res[n.Name] = np
	}
	return res
}

// GetLogs возвращает последние строки лога.
func (s *Service) GetLogs() []string {
	return s.logs.snapshot()
}

// AutoConnect подключает все сохранённые сети, если включён автозапуск. Нужно,
// чтобы при старте из автозапуска приложение сразу поднимало VPN, а не просто
// открывало окно.
func (s *Service) AutoConnect() {
	if !platform.IsAutostartEnabled() {
		return
	}
	s.g.mu.Lock()
	nets := append([]Network(nil), s.g.cfg.Networks...)
	s.g.mu.Unlock()
	if len(nets) == 0 {
		return
	}
	logf("Автозапуск: подключение к сетям (%d)", len(nets))
	for _, n := range nets {
		if isRunning(n.Name) {
			continue
		}
		if err := s.Connect(n.Name); err != nil {
			logf("Автоподключение «%s»: %v", n.Name, err)
		}
	}
}

// IsAutostartEnabled сообщает, включён ли автозапуск.
func (s *Service) IsAutostartEnabled() bool {
	return platform.IsAutostartEnabled()
}

// SetAutostart включает/выключает автозапуск при входе в систему.
func (s *Service) SetAutostart(enable bool) error {
	if err := platform.SetAutostart(enable); err != nil {
		return err
	}
	if enable {
		logf("Автозапуск включён")
	} else {
		logf("Автозапуск выключен")
	}
	return nil
}

// Shutdown останавливает все сети.
func (s *Service) Shutdown() {
	stopAll()
}
