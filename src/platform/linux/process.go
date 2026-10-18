//go:build linux

package linux

import (
	"os"
	"os/exec"
	"syscall"

	"potatonet/src/platform/base"
)

func SetupProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func KillProcessGroup(pid int) {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		pgid = pid
	}
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
}

func KillAllByName() {
	_ = exec.Command("pkill", "-e", "easytier-core").Run()
	_ = os.RemoveAll(base.PidDir())
}
