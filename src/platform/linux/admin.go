//go:build linux

package linux

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"potatonet/src/platform/base"
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

// preservedEnv — переменные графической сессии, которые нужно прокинуть в
// elevated-процесс при запуске через sudo (pkexec получает их через env).
const preservedEnv = "DISPLAY,WAYLAND_DISPLAY,XAUTHORITY,XDG_RUNTIME_DIR,XDG_SESSION_TYPE,DBUS_SESSION_BUS_ADDRESS"

var graphEnvKeys = []string{
	"DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY",
	"XDG_RUNTIME_DIR", "XDG_SESSION_TYPE", "DBUS_SESSION_BUS_ADDRESS",
}

// askpassCandidates — графические программы запроса пароля для sudo -A. Нужны,
// когда pkexec недоступен (например, в AppImage без зависимости).
var askpassCandidates = []string{
	"/usr/bin/ksshaskpass",
	"/usr/bin/ssh-askpass",
	"/usr/lib/ssh/x11-ssh-askpass",
	"/usr/bin/lxqt-openssh-askpass",
}

// RelaunchElevated перезапускает бэкенд с правами root. В AppImage исполняемый
// файл лежит в FUSE-монтировании, недоступном root, поэтому копируем бинарник
// в /tmp и запускаем копию, явно передавая каталог данных пользователя.
//
// Повышение прав из графической сессии возможно только через polkit-агент
// (pkexec) или askpass: обычный sudo требует терминала и из меню не работает.
// Не-root процесс остаётся жив до закрытия elevated-окна (cmd.Wait) — иначе
// desktop-окружение сносит запущенный процесс вместе с окном.
func RelaunchElevated(dataDir string) error {
	launch := base.CurrentExe()
	if copy, err := copyToTemp(launch); err == nil {
		launch = copy
	}

	logFile := openElevationLog(dataDir)
	if logFile != nil {
		defer logFile.Close()
	}
	logf := func(format string, a ...any) {
		line := fmt.Sprintf(format, a...)
		if logFile != nil {
			fmt.Fprintln(logFile, line)
		}
		fmt.Fprintln(os.Stderr, line)
	}

	args := []string{"--data-dir", dataDir}
	displayEnv := graphEnv()

	var cmd *exec.Cmd
	graphical := false
	switch askpass := findAskpass(); {
	case hasPkexec():
		graphical = true
		logf("elevation: pkexec (display=%q wayland=%q runtime=%q)",
			os.Getenv("DISPLAY"), os.Getenv("WAYLAND_DISPLAY"), os.Getenv("XDG_RUNTIME_DIR"))
		// pkexec очищает окружение; прокидываем переменные графической сессии.
		cmd = exec.Command("pkexec",
			append(append([]string{"env"}, displayEnv...), append([]string{launch}, args...)...)...)
	case askpass != "":
		graphical = true
		logf("elevation: sudo -A (askpass=%s)", askpass)
		cmd = exec.Command("sudo", append([]string{
			"-A", "--preserve-env=" + preservedEnv, launch,
		}, args...)...)
		cmd.Env = append(os.Environ(), "SUDO_ASKPASS="+askpass)
	default:
		logf("elevation: sudo (pkexec/askpass не найдены, нужен терминал)")
		cmd = exec.Command("sudo", append([]string{
			"-E", "--preserve-env=" + preservedEnv, launch,
		}, args...)...)
	}

	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		logf("elevation failed: %v", err)
		notifyElevationError(err)
		return fmt.Errorf("elevation failed: %v", err)
	}
	// Держим не-root процесс живым, пока работает elevated-окно: иначе при
	// запуске из меню (systemd-скоуп) окно сносится вместе с нами, а из
	// терминала sudo теряет управляющий терминал и не может прочитать пароль.
	if err := cmd.Wait(); err != nil {
		logf("elevation: process exited: %v", err)
		if !graphical && !stdinIsTTY() {
			notifyElevationError(err)
		}
	}
	return nil
}

func graphEnv() []string {
	var out []string
	for _, k := range graphEnvKeys {
		if v := os.Getenv(k); v != "" {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func hasPkexec() bool {
	_, err := exec.LookPath("pkexec")
	return err == nil
}

func findAskpass() string {
	if v := os.Getenv("SUDO_ASKPASS"); v != "" {
		if _, err := exec.LookPath(v); err == nil {
			return v
		}
	}
	for _, c := range askpassCandidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// openElevationLog открывает лог elevated-процесса в каталоге данных. Ошибки
// запуска из меню иначе некуда выводить — stdio запустившего процесса закрыт.
func openElevationLog(dir string) *os.File {
	f, err := os.OpenFile(filepath.Join(dir, "potatonet.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil
	}
	return f
}

// notifyElevationError пытается показать графическое сообщение об ошибке
// повышения прав, если доступен kdialog/zenity/xmessage.
func notifyElevationError(err error) {
	msg := "Не удалось запустить PotatoNet с правами администратора.\n\n" +
		err.Error() + "\n\nУстановите пакет pkexec (policykit-1 / polkit)."
	for _, name := range []string{"kdialog", "zenity", "xmessage"} {
		p, lookErr := exec.LookPath(name)
		if lookErr != nil {
			continue
		}
		var cmdArgs []string
		switch name {
		case "kdialog":
			cmdArgs = []string{"--error", msg}
		case "zenity":
			cmdArgs = []string{"--error", "--text=" + msg}
		default:
			cmdArgs = []string{msg}
		}
		if exec.Command(p, cmdArgs...).Run() == nil {
			return
		}
	}
}

// copyToTemp копирует исполняемый файл в /tmp (доступный root) и возвращает
// путь к копии. Запись идёт через временный файл + rename, чтобы не поймать
// ETXTBSY, если копия уже запущена.
func copyToTemp(src string) (string, error) {
	dst := filepath.Join(os.TempDir(), fmt.Sprintf("potatonet-core-%d", os.Getuid()))
	tmp, err := os.CreateTemp(os.TempDir(), "potatonet-core-*.tmp")
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
