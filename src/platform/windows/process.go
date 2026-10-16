//go:build windows

package windows

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"pnet/src/platform/base"
)

func SetupProcAttrs(cmd *exec.Cmd) {
	// CREATE_NEW_PROCESS_GROUP + CREATE_NO_WINDOW (скрыть консоль)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200 | 0x08000000}
}

func KillProcessGroup(pid int) {
	_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(pid)).Run()
}

func KillAllByName() {
	_ = exec.Command("taskkill", "/F", "/IM", "easytier-core.exe").Run()
	_ = os.RemoveAll(base.PidDir())
}
