package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"pnet/src/platform"
)

const defaultPanelAddr = "127.0.0.1:19090"

// gui — локальный HTTP-API, к которому обращается окно NW.js.
type gui struct {
	paths platform.Paths
	cfg   *Config
	mu    sync.Mutex
}

// runServer запускает бэкенд без браузера: только локальный API.
func runServer(paths platform.Paths) {
	// Готовим EasyTier до повышения прав: в AppImage бинарники лежат в
	// read-only FUSE-монтировании, недоступном root.
	if err := installEasyTier(paths); err != nil {
		logf("EasyTier: %v", err)
	}

	if !platform.IsAdmin() && os.Getenv("PNET_SKIP_ADMIN") == "" {
		fmt.Println("Backend requires administrator privileges. Relaunching...")
		if err := platform.RelaunchElevated(paths.LibDir); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to elevate: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg, err := loadConfig(paths)
	if err != nil {
		cfg = defaultConfig()
	}
	g := &gui{paths: paths, cfg: cfg}

	addr := defaultPanelAddr
	if v := strings.TrimSpace(os.Getenv("PNET_ADDR")); v != "" {
		addr = v
	} else if strings.TrimSpace(cfg.WebPanelAdress) != "" {
		addr = strings.TrimSpace(cfg.WebPanelAdress)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logf("Failed to listen on %s: %v", addr, err)
		os.Exit(1)
	}
	logf("pnet backend listening on %s", addr)

	srv := &http.Server{Handler: g.mux()}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		logf("HTTP server error: %v", err)
	}
}

func (g *gui) mux() *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("/api/state", g.handleState)
	m.HandleFunc("/api/networks", g.handleSaveNetworks)
	m.HandleFunc("/api/connect", g.handleConnect)
	m.HandleFunc("/api/disconnect", g.handleDisconnect)
	m.HandleFunc("/api/peers", g.handlePeers)
	m.HandleFunc("/api/shutdown", g.handleShutdown)
	return m
}

func (g *gui) handleState(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	cfg := g.cfg
	networks := append([]Network(nil), cfg.Networks...)
	g.mu.Unlock()

	if networks == nil {
		networks = []Network{}
	}
	writeJSON(w, map[string]any{
		"ok":         true,
		"networks":   networks,
		"connected":  runningNames(),
		"configFile": g.paths.ConfigFile,
	})
}

// handleSaveNetworks сохраняет минимальный список сетей и останавливает те,
// что были удалены или изменены.
func (g *gui) handleSaveNetworks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Networks []Network `json:"networks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	seen := map[string]bool{}
	for i, n := range req.Networks {
		name := strings.TrimSpace(n.Name)
		if name == "" {
			writeErr(w, http.StatusBadRequest, "network name is required")
			return
		}
		if seen[name] {
			writeErr(w, http.StatusBadRequest, "duplicate network: "+name)
			return
		}
		seen[name] = true
		req.Networks[i].Name = name
		req.Networks[i].Secret = strings.TrimSpace(n.Secret)
		req.Networks[i].Hostname = strings.TrimSpace(n.Hostname)
	}

	g.mu.Lock()
	old := g.cfg.Networks
	g.cfg.Networks = req.Networks
	cfg := g.cfg
	g.mu.Unlock()

	if err := saveConfig(cfg, g.paths); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}
	platform.ChownToUser(g.paths.LibDir)
	logf("Config saved: %s", g.paths.ConfigFile)

	// Останавливаем сети, которых больше нет или которые изменились.
	for _, o := range old {
		if !containsNetwork(req.Networks, o) {
			stopNetwork(o.Name)
		}
	}
	writeOK(w, "Config saved")
}

func (g *gui) handleConnect(w http.ResponseWriter, r *http.Request) {
	name, ok := decodeName(w, r)
	if !ok {
		return
	}
	g.mu.Lock()
	cfg := g.cfg
	_, net, found := findNetwork(cfg.Networks, name)
	g.mu.Unlock()
	if !found {
		writeErr(w, http.StatusNotFound, "network not found: "+name)
		return
	}
	markStarting(net.Name)
	go func() {
		err := startNetwork(cfg, g.paths, net)
		if err != nil {
			logf("Error: %v", err)
		}
		markFinished(net.Name, err)
	}()
	writeOK(w, "connecting")
}

func (g *gui) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	name, ok := decodeName(w, r)
	if !ok {
		return
	}
	stopNetwork(name)
	writeOK(w, "disconnected")
}

func (g *gui) handlePeers(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	networks := append([]Network(nil), g.cfg.Networks...)
	g.mu.Unlock()

	res := map[string]networkPeers{}
	for _, n := range networks {
		np := peersForNetwork(g.paths, n.Name)
		np.Connected = isRunning(n.Name)
		st := getStatus(n.Name)
		np.Starting = st.Starting
		np.Error = st.Error
		res[n.Name] = np
	}
	writeJSON(w, map[string]any{"ok": true, "networks": res})
}

func (g *gui) handleShutdown(w http.ResponseWriter, r *http.Request) {
	writeOK(w, "shutting down")
	go func() {
		time.Sleep(150 * time.Millisecond)
		stopAll()
		os.Exit(0)
	}()
}

func decodeName(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return "", false
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return "", false
	}
	return name, true
}

func findNetwork(networks []Network, name string) (int, Network, bool) {
	for i, n := range networks {
		if n.Name == name {
			return i, n, true
		}
	}
	return 0, Network{}, false
}

func containsNetwork(networks []Network, n Network) bool {
	for _, x := range networks {
		if x.Name == n.Name && x.Secret == n.Secret && x.Hostname == n.Hostname {
			return true
		}
	}
	return false
}
