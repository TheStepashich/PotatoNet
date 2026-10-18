//go:build linux

package linux

import (
	"fmt"
	"os"
	"path/filepath"

	"potatonet/src/platform/base"
)

// desktopFile — пользовательский автозапуск (без root), как это делают
// обычные GUI-приложения: ~/.config/autostart/PotatoNet.desktop.
func desktopFile() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.TempDir()
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", "PotatoNet.desktop")
}

// launchCommand — команда автозапуска. Для AppImage нужен сам файл образа,
// потому что точка монтирования временная.
func launchCommand() string {
	if v := os.Getenv("APPIMAGE"); v != "" {
		return fmt.Sprintf("%q", v)
	}
	return fmt.Sprintf("%q", base.CurrentExe())
}

func IsAutostartEnabled() bool {
	_, err := os.Stat(desktopFile())
	return err == nil
}

func SetAutostart(enable bool) error {
	file := desktopFile()
	if !enable {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=PotatoNet\n" +
		"Comment=PotatoNet VPN\n" +
		"Exec=" + launchCommand() + "\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
	return os.WriteFile(file, []byte(content), 0o644)
}
