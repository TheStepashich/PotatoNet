//go:build linux

package platform

import (
	"errors"
	"os/exec"

	"potatonet/src/platform/base"
	linux "potatonet/src/platform/linux"
)

func IsAdmin() bool                     { return linux.IsAdmin() }
func RelaunchElevated(dir string) error { return linux.RelaunchElevated(dir) }

// Планировщик заданий — механизм Windows; на Linux повышение прав всегда идёт
// через pkexec/sudo, поэтому задания не используются.
func TaskExists() bool                { return false }
func RunElevatedTask() error          { return errors.New("scheduled task is not supported on linux") }
func EnsureElevatedTask(string) error { return nil }
func ChownToUser(path string)         { linux.ChownToUser(path) }
func SetupProcAttrs(c *exec.Cmd)      { linux.SetupProcAttrs(c) }
func KillProcessGroup(pid int)        { linux.KillProcessGroup(pid) }
func KillAllByName()                  { linux.KillAllByName() }
func CoreName() string                { return linux.CoreName() }
func CLIName() string                 { return linux.CLIName() }
func LocalEasyTierDir() string        { return linux.LocalEasyTierDir() }
func InstallURL() string              { return linux.InstallURL() }
func DataDir() string                 { return linux.DataDir() }
func IsWritable(dir string) bool      { return base.IsWritable(dir) }
func IsAutostartEnabled() bool        { return linux.IsAutostartEnabled() }
func SetAutostart(enable bool) error  { return linux.SetAutostart(enable) }
func SetupWebviewEnv()                { linux.SetupWebviewEnv() }
