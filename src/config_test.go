package backend

import (
	"os"
	"path/filepath"
	"potatonet/src/platform"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	if len(cfg.Bootstrap) != 1 {
		t.Fatalf("expected 1 bootstrap peer, got %+v", cfg.Bootstrap)
	}
	if cfg.Bootstrap[0] != "udp://bootstrap.potatom.net:11010" {
		t.Fatalf("unexpected bootstrap peers: %+v", cfg.Bootstrap)
	}
	if len(cfg.Networks) != 0 {
		t.Fatalf("expected no default networks, got %+v", cfg.Networks)
	}
}

func TestDefaultBootstrapEnvOverride(t *testing.T) {
	t.Setenv("POTATONET_BOOTSTRAP", "udp://a.example:11010, udp://b.example:11010")
	got := defaultBootstrap()
	if len(got) != 2 || got[0] != "udp://a.example:11010" || got[1] != "udp://b.example:11010" {
		t.Fatalf("env bootstrap mismatch: %+v", got)
	}
}

func TestDefaultConfigTemplateLoads(t *testing.T) {
	dir := t.TempDir()
	paths := platform.Paths{ConfigFile: filepath.Join(dir, "config.yml")}
	if err := saveConfig(defaultConfig(), paths); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Bootstrap) != 1 || got.Bootstrap[0] != "udp://bootstrap.potatom.net:11010" {
		t.Fatalf("bootstrap mismatch: %+v", got.Bootstrap)
	}
	if len(got.Networks) != 0 {
		t.Fatalf("networks mismatch: %+v", got.Networks)
	}
}

func TestHostnameRoundtrip(t *testing.T) {
	dir := t.TempDir()
	paths := platform.Paths{ConfigFile: filepath.Join(dir, "config.yml")}

	cfg := &Config{
		Bootstrap: []string{"udp://1.2.3.4:11010"},
		Networks: []Network{
			{Name: "party", Secret: "", Hostname: "mybox"},
		},
	}
	if err := saveConfig(cfg, paths); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Networks) != 1 || got.Networks[0].Hostname != "mybox" {
		t.Fatalf("hostname mismatch: %+v", got.Networks)
	}
}

func TestLoadAndSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()
	paths := platform.Paths{ConfigFile: filepath.Join(dir, "config.yml")}

	cfg := &Config{
		Bootstrap: []string{"udp://1.2.3.4:11010"},
		Networks: []Network{
			{Name: "home", Secret: "s3cret", Hostname: "laptop"},
			{Name: "work", Secret: ""},
		},
	}
	if err := saveConfig(cfg, paths); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Bootstrap) != 1 || got.Bootstrap[0] != "udp://1.2.3.4:11010" {
		t.Fatalf("bootstrap mismatch: %+v", got.Bootstrap)
	}
	if len(got.Networks) != 2 {
		t.Fatalf("networks mismatch: %+v", got.Networks)
	}
	if got.Networks[0].Hostname != "laptop" || got.Networks[1].Secret != "" {
		t.Fatalf("network mismatch: %+v", got.Networks)
	}
}

func TestMigrateLegacyConfig(t *testing.T) {
	dir := t.TempDir()
	paths := platform.Paths{ConfigFile: filepath.Join(dir, "config.yml")}

	legacy := "bootstrap:\n  - udp://1.2.3.4:11010\nip: 10.144.0.5\nnet:\n  name: default\n  secret: pw\n"
	if err := os.WriteFile(paths.ConfigFile, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Networks) != 1 {
		t.Fatalf("expected 1 network, got %+v", got.Networks)
	}
	n := got.Networks[0]
	if n.Name != "default" || n.Secret != "pw" {
		t.Fatalf("migrated network mismatch: %+v", n)
	}
	if got.IP != "" || got.Net != nil {
		t.Fatalf("legacy fields not cleared: ip=%q net=%+v", got.IP, got.Net)
	}
}
