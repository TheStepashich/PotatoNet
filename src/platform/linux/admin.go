//go:build linux

package linux

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"pnet/src/platform/base"
)

func IsAdmin() bool {
	return os.Geteuid() == 0
}

// invokingUser возвращает uid/gid пользователя, который запустил pkexec/sudo.
// Нужен, чтобы root-бэкенд не плодил root-овые файлы в домашнем каталоге.
func invokingUser() (int, int, bool) {
	if os.Geteuid() != 0 {
		return 0, 0, false
	}
	uid, err := strconv.Atoi(os.Getenv("PKEXEC_UID"))
	if err != nil {
		uid, err = strconv.Atoi(os.Getenv("SUDO_UID"))
	}
	if err != nil {
		return 0, 0, false
	}
	gid, _ := strconv.Atoi(os.Getenv("PKEXEC_GID"))
	if gid == 0 {
		gid, _ = strconv.Atoi(os.Getenv("SUDO_GID"))
	}
	if gid == 0 {
		gid = uid
	}
	return uid, gid, true
}

// ChownToUser переводит файлы в каталоге во владение вызывающего пользователя,
// если бэкенд запущен под root через pkexec/sudo. Иначе ничего не делает.
func ChownToUser(path string) {
	uid, gid, ok := invokingUser()
	if !ok {
		return
	}
	_ = filepath.Walk(path, func(p string, _ os.FileInfo, err error) error {
		if err == nil {
			_ = os.Lchown(p, uid, gid)
		}
		return nil
	})
}

// RelaunchElevated перезапускает бэкенд с правами root. В AppImage исполняемый
// файл лежит в FUSE-монтировании, недоступном root, поэтому копируем бинарник
// в /tmp и запускаем копию, явно передавая каталог данных пользователя.
func RelaunchElevated(dataDir string) error {
	launch := base.CurrentExe()
	if copy, err := copyToTemp(launch); err == nil {
		launch = copy
	}

	args := []string{"--data-dir", dataDir, "server"}

	var cmd *exec.Cmd
	if _, err := exec.LookPath("pkexec"); err == nil {
		cmd = exec.Command("pkexec", append([]string{launch}, args...)...)
	} else {
		cmd = exec.Command("sudo", append([]string{
			"-E",
			"--preserve-env=DISPLAY,WAYLAND_DISPLAY,XAUTHORITY,DBUS_SESSION_BUS_ADDRESS",
			launch,
		}, args...)...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("elevation failed: %v", err)
	}
	return nil
}

// copyToTemp копирует исполняемый файл в /tmp (доступный root) и возвращает
// путь к копии. Запись идёт через временный файл + rename, чтобы не поймать
// ETXTBSY, если копия уже запущена.
func copyToTemp(src string) (string, error) {
	dst := filepath.Join(os.TempDir(), fmt.Sprintf("pnet-core-%d", os.Getuid()))
	tmp, err := os.CreateTemp(os.TempDir(), "pnet-core-*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())

	in, err := os.Open(src)
	if err != nil {
		tmp.Close()
		return "", err
	}
	defer in.Close()

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return "", err
	}
	return dst, nil
}
