//go:build linux

package linux

import (
	"os"
	"path/filepath"
)

// DataDir — пользовательский каталог для config.yml и EasyTier, когда папка
// с исполняемым файлом доступна только для чтения (например, AppImage).
func DataDir() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "pnet")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "pnet")
	}
	return filepath.Join(home, ".local", "share", "pnet")
}

func CoreName() string         { return "easytier-core" }
func CLIName() string          { return "easytier-cli" }
func LocalEasyTierDir() string { return "easytier-linux-x86_64" }

func InstallURL() string {
	return "https://github.com/EasyTier/EasyTier/releases/download/v2.4.5/easytier-linux-x86_64-v2.4.5.zip"
}
