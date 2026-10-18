//go:build windows

package windows

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"

	"potatonet/src/platform/base"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const runValue = "PotatoNet"

func IsAutostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runValue)
	return err == nil
}

func SetAutostart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		k, _, err = registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
	}
	defer k.Close()

	if !enable {
		if err := k.DeleteValue(runValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	return k.SetStringValue(runValue, fmt.Sprintf(`"%s"`, base.CurrentExe()))
}
