package backend

import "potatonet/src/platform"

// CmdAutostart включает/выключает автозапуск приложения при входе в систему.
func CmdAutostart(enable bool) error {
	if err := platform.SetAutostart(enable); err != nil {
		return err
	}
	if enable {
		logf("Autostart enabled")
	} else {
		logf("Autostart disabled")
	}
	return nil
}
