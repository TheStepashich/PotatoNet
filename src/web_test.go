package backend

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"potatonet/src/platform"
	"testing"
)

func TestStateReturnsNetworks(t *testing.T) {
	paths := emptyConfigPaths(t)
	g := newTestGUI(paths)
	g.cfg.Networks = []Network{{Name: "home", Secret: "pw", Hostname: "laptop"}}

	rec := httptest.NewRecorder()
	g.mux().ServeHTTP(rec, httptest.NewRequest("GET", "/api/state", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct {
		Ok        bool      `json:"ok"`
		Connected []string  `json:"connected"`
		Networks  []Network `json:"networks"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Ok || len(resp.Networks) != 1 {
		t.Fatalf("bad state: %+v", resp)
	}
	if resp.Networks[0].Name != "home" || resp.Networks[0].Secret != "pw" {
		t.Fatalf("network mismatch: %+v", resp.Networks)
	}
}

func TestSaveWritesConfig(t *testing.T) {
	paths := emptyConfigPaths(t)
	g := newTestGUI(paths)

	body := `{"networks":[{"name":"home","secret":"pw","hostname":"laptop"}]}`
	rec := httptest.NewRecorder()
	g.mux().ServeHTTP(rec, httptest.NewRequest("POST", "/api/networks", bytes.NewBufferString(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	cfg, err := loadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Networks) != 1 || cfg.Networks[0].Name != "home" || cfg.Networks[0].Hostname != "laptop" {
		t.Fatalf("saved config mismatch: %+v", cfg.Networks)
	}
}

func TestSaveRejectsDuplicateNetwork(t *testing.T) {
	paths := emptyConfigPaths(t)
	g := newTestGUI(paths)

	body := `{"networks":[{"name":"net1","secret":"a"},{"name":"net1","secret":"b"}]}`
	rec := httptest.NewRecorder()
	g.mux().ServeHTTP(rec, httptest.NewRequest("POST", "/api/networks", bytes.NewBufferString(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestSaveRejectsEmptyName(t *testing.T) {
	paths := emptyConfigPaths(t)
	g := newTestGUI(paths)

	body := `{"networks":[{"name":"  ","secret":"a"}]}`
	rec := httptest.NewRecorder()
	g.mux().ServeHTTP(rec, httptest.NewRequest("POST", "/api/networks", bytes.NewBufferString(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestConnectUnknownNetwork(t *testing.T) {
	paths := emptyConfigPaths(t)
	g := newTestGUI(paths)

	rec := httptest.NewRecorder()
	g.mux().ServeHTTP(rec, httptest.NewRequest("POST", "/api/connect", bytes.NewBufferString(`{"name":"nope"}`)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestPeersEmptyWithoutCLI(t *testing.T) {
	paths := emptyConfigPaths(t)
	g := newTestGUI(paths)
	g.cfg.Networks = []Network{{Name: "home"}}

	rec := httptest.NewRecorder()
	g.mux().ServeHTTP(rec, httptest.NewRequest("GET", "/api/peers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct {
		Ok       bool                    `json:"ok"`
		Networks map[string]NetworkPeers `json:"networks"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Ok || len(resp.Networks["home"].Peers) != 0 {
		t.Fatalf("bad peers response: %+v", resp)
	}
}

func newTestGUI(paths platform.Paths) *gui {
	cfg, err := loadConfig(paths)
	if err != nil {
		cfg = defaultConfig()
	}
	return &gui{paths: paths, cfg: cfg}
}

func emptyConfigPaths(t *testing.T) platform.Paths {
	t.Helper()
	dir := t.TempDir()
	paths := platform.Paths{ConfigFile: filepath.Join(dir, "config.yml")}
	cfg := &Config{Bootstrap: []string{}, Networks: []Network{}}
	if err := saveConfig(cfg, paths); err != nil {
		t.Fatal(err)
	}
	return paths
}
