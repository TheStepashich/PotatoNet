//go:build linux

package linux

import "os"

// SetupWebviewEnv подготавливает окружение WebKitGTK. Приложение работает с
// правами root, а песочница WebKit не умеет запускать процессы-помощники под
// root — окно остаётся белым. Также отключаем DMABUF-рендерер, который на части
// драйверов/в Wayland даёт пустое окно.
func SetupWebviewEnv() {
	if os.Geteuid() == 0 {
		setDefault("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "1")
	}
	setDefault("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
}

func setDefault(key, value string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, value)
	}
}
