//go:build windows

package platform

import (
	"os/exec"

	"potatonet/src/platform/base"
	windows "potatonet/src/platform/windows"
)

func IsAdmin() bool                     { return windows.IsAdmin() }
func RelaunchElevated(dir string) error { return windows.RelaunchElevated(dir) }
func TaskExists() bool                  { return windows.TaskExists() }
func RunElevatedTask() error            { return windows.RunElevatedTask() }
func EnsureElevatedTask(dir string) error {
	return windows.EnsureElevatedTask(dir)
}
func ChownToUser(path string)        { windows.ChownToUser(path) }
func SetupProcAttrs(c *exec.Cmd)     { windows.SetupProcAttrs(c) }
func KillProcessGroup(pid int)       { windows.KillProcessGroup(pid) }
func KillAllByName()                 { windows.KillAllByName() }
func CoreName() string               { return windows.CoreName() }
func CLIName() string                { return windows.CLIName() }
func LocalEasyTierDir() string       { return windows.LocalEasyTierDir() }
func InstallURL() string             { return windows.InstallURL() }
func DataDir() string                { return windows.DataDir() }
func IsWritable(dir string) bool     { return base.IsWritable(dir) }
func IsAutostartEnabled() bool       { return windows.IsAutostartEnabled() }
func SetAutostart(enable bool) error { return windows.SetAutostart(enable) }
func SetupWebviewEnv()               { windows.SetupWebviewEnv() }
