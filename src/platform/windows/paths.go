//go:build windows

package windows

import (
	"os"
	"path/filepath"
)

// DataDir — пользовательский каталог для config.yml и EasyTier, когда папка
// с установленным приложением (Program Files) доступна только для чтения.
func DataDir() string {
	if v := os.Getenv("APPDATA"); v != "" {
		return filepath.Join(v, "pnet")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "pnet")
	}
	return filepath.Join(home, "pnet")
}

func CoreName() string         { return "easytier-core.exe" }
func CLIName() string          { return "easytier-cli.exe" }
func LocalEasyTierDir() string { return "easytier-windows-x86_64" }

func InstallURL() string {
	return "https://github.com/EasyTier/EasyTier/releases/download/v2.4.5/easytier-windows-x86_64-v2.4.5.zip"
}
