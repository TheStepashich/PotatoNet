//go:build windows

package windows

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"potatonet/src/platform/base"
)

func IsAdmin() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	var elevated uint32
	var sz uint32
	err := windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevated)), 4, &sz)
	return err == nil && elevated != 0
}

// ChownToUser — no-op: elevation на Windows не меняет владельца файлов.
func ChownToUser(string) {}

// RelaunchElevated перезапускает GUI с правами администратора (UAC), передавая
// каталог данных, чтобы elevated-окно работало с тем же config.yml/EasyTier.
func RelaunchElevated(dataDir string) error {
	exe := base.CurrentExe()
	verb := syscall.StringToUTF16Ptr("runas")
	file := syscall.StringToUTF16Ptr(exe)
	arg := syscall.StringToUTF16Ptr(fmt.Sprintf(`--data-dir "%s"`, dataDir))
	dir := syscall.StringToUTF16Ptr(os.Getenv("WINDIR"))
	// SW_SHOWNORMAL (1): показать окно elevated-приложения.
	err := windows.ShellExecute(0, verb, file, arg, dir, 1)
	if err != nil {
		return fmt.Errorf("ShellExecute: %v", err)
	}
	return nil
}
