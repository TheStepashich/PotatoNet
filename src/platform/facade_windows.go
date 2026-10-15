//go:build windows

package platform

import (
	"os/exec"

	"pnet/src/platform/base"
	windows "pnet/src/platform/windows"
)

func IsAdmin() bool                     { return windows.IsAdmin() }
func RelaunchElevated(dir string) error { return windows.RelaunchElevated(dir) }
func ChownToUser(path string)           { windows.ChownToUser(path) }
func SetupProcAttrs(c *exec.Cmd)        { windows.SetupProcAttrs(c) }
func KillProcessGroup(pid int)          { windows.KillProcessGroup(pid) }
func KillAllByName()                    { windows.KillAllByName() }
func CoreName() string                  { return windows.CoreName() }
func CLIName() string                   { return windows.CLIName() }
func LocalEasyTierDir() string          { return windows.LocalEasyTierDir() }
func InstallURL() string                { return windows.InstallURL() }
func DataDir() string                   { return windows.DataDir() }
func IsWritable(dir string) bool        { return base.IsWritable(dir) }
