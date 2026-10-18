//go:build windows

package windows

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"potatonet/src/platform/base"
)

func SetupProcAttrs(cmd *exec.Cmd) {
	// DETACHED_PROCESS (0x00000008) + CREATE_NEW_PROCESS_GROUP (0x00000200)
	// + CREATE_NO_WINDOW (0x08000000) + HideWindow. Нужен, чтобы консольные
	// easytier-core/easytier-cli/reg/taskkill не открывали окно терминала:
	// DETACHED_PROCESS отвязывает от консоли родителя (иначе CREATE_NO_WINDOW
	// игнорируется), CREATE_NO_WINDOW запрещает новую, HideWindow — подстраховка.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x00000200 | 0x08000000,
	}
}

func KillProcessGroup(pid int) {
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(pid))
	SetupProcAttrs(cmd)
	_ = cmd.Run()
}

func KillAllByName() {
	cmd := exec.Command("taskkill", "/F", "/IM", "easytier-core.exe")
	SetupProcAttrs(cmd)
	_ = cmd.Run()
	_ = os.RemoveAll(base.PidDir())
}
