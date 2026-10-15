//go:build linux

package platform

import (
	"os/exec"

	"pnet/src/platform/base"
	linux "pnet/src/platform/linux"
)

func IsAdmin() bool                     { return linux.IsAdmin() }
func RelaunchElevated(dir string) error { return linux.RelaunchElevated(dir) }
func ChownToUser(path string)           { linux.ChownToUser(path) }
func SetupProcAttrs(c *exec.Cmd)        { linux.SetupProcAttrs(c) }
func KillProcessGroup(pid int)          { linux.KillProcessGroup(pid) }
func KillAllByName()                    { linux.KillAllByName() }
func CoreName() string                  { return linux.CoreName() }
func CLIName() string                   { return linux.CLIName() }
func LocalEasyTierDir() string          { return linux.LocalEasyTierDir() }
func InstallURL() string                { return linux.InstallURL() }
func DataDir() string                   { return linux.DataDir() }
func IsWritable(dir string) bool        { return base.IsWritable(dir) }
