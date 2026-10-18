//go:build windows

package windows

import "os"

// SetupWebviewEnv — настройки окружения WebView2. Приложение запускается с
// правами администратора; в elevated-контексте песочница рендерера WebView2
// может не инициализироваться, из-за чего браузерный процесс завершается
// (белое окно / «WebView2 crashed»). Отключаем песочницу, если пользователь не
// задал аргументы сам.
func SetupWebviewEnv() {
	setDefault("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--no-sandbox")
}

func setDefault(key, value string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, value)
	}
}
