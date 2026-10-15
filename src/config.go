package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"pnet/src/platform"
)

// Network — минимальная настройка одной VPN-сети, доступная пользователю.
type Network struct {
	Name     string `yaml:"name" json:"name"`
	Secret   string `yaml:"secret" json:"secret"`
	Hostname string `yaml:"hostname,omitempty" json:"hostname,omitempty"`
}

type Config struct {
	Bootstrap []string  `yaml:"bootstrap" json:"-"`
	Networks  []Network `yaml:"networks" json:"networks"`

	// Служебные и legacy-поля. Больше не редактируются из интерфейса,
	// но читаются со старых конфигов для миграции.
	ListenPort     int        `yaml:"listen_port,omitempty" json:"-"`
	WebPanelAdress string     `yaml:"web_panel_adress,omitempty" json:"-"`
	DNS            string     `yaml:"dns,omitempty" json:"-"`
	IP             string     `yaml:"ip,omitempty" json:"-"`
	Net            *LegacyNet `yaml:"net,omitempty" json:"-"`
}

type LegacyNet struct {
	Name   string `yaml:"name" json:"name"`
	Secret string `yaml:"secret" json:"secret"`
}

// defaultBootstrap — встроенные bootstrap-сервера. Пользователь их не видит.
func defaultBootstrap() []string {
	return []string{
		"udp://45.112.194.131:11010",
		"udp://92.63.103.86:11010",
		"udp://104.239.87.24:11010",
	}
}

func defaultConfig() *Config {
	return &Config{
		Bootstrap: defaultBootstrap(),
		Networks:  []Network{},
	}
}

func loadConfig(paths platform.Paths) (*Config, error) {
	data, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	migrateConfig(&cfg)
	if len(cfg.Bootstrap) == 0 {
		cfg.Bootstrap = defaultBootstrap()
	}
	if cfg.Networks == nil {
		cfg.Networks = []Network{}
	}
	return &cfg, nil
}

func migrateConfig(cfg *Config) {
	if len(cfg.Networks) == 0 && cfg.Net != nil {
		cfg.Networks = []Network{{Name: cfg.Net.Name, Secret: cfg.Net.Secret}}
		cfg.Net = nil
		cfg.IP = ""
	}
}

func saveConfig(cfg *Config, paths platform.Paths) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(paths.ConfigFile, data, 0o644)
}
