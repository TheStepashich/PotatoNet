package main

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	windowsopts "github.com/wailsapp/wails/v2/pkg/options/windows"

	backend "potatonet/src"
	"potatonet/src/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

// dataDirArg достаёт значение флага --data-dir без полного разбора аргументов
// (нужно до запуска Wails, чтобы elevated-копия использовала тот же каталог).
func dataDirArg(args []string) string {
	for i, a := range args {
		if a == "--data-dir" && i+1 < len(args) {
			return args[i+1]
		}
		if v, ok := strings.CutPrefix(a, "--data-dir="); ok {
			return v
		}
	}
	return ""
}

// isCLI сообщает, запущен ли бинарник в консольном режиме (headless-команды).
func isCLI(args []string) bool {
	for _, a := range args {
		switch a {
		case "run", "stop", "server", "peers", "autostart", "no-autostart", "version",
			"-run", "-stop", "-server", "-peers", "-autostart", "-no-autostart", "-version":
			return true
		}
	}
	return false
}

// hasArg сообщает, есть ли среди аргументов нужный флаг (без разбора значений).
func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func main() {
	args := os.Args[1:]

	if isCLI(args) {
		backend.Main(args)
		return
	}

	paths := backend.DetectPaths(dataDirArg(args))

	// Копируем EasyTier до повышения прав: в AppImage комплект лежит в
	// read-only FUSE-монтировании, недоступном root.
	if !bindingsMode {
		if err := backend.Prepare(paths); err != nil {
			println("EasyTier:", err.Error())
		}
	}

	// Создание VPN-интерфейса требует прав root. Если их нет — перезапускаемся
	// с повышением (UAC на Windows, pkexec/sudo на Linux) и передаём каталог
	// данных, чтобы elevated-окно работало с тем же конфигом. В режиме генерации
	// биндингов повышение пропускаем — иначе wails.Run не будет достигнут.
	if !bindingsMode && !platform.IsAdmin() &&
		os.Getenv("POTATONET_SKIP_ADMIN") == "" && !hasArg(args, "--no-relaunch") {
		// На Windows UAC-запрос показывается только один раз: при первом запуске
		// создаётся задание планировщика, и дальше запуск идёт через него — без
		// UAC. Запущенный заданием процесс несёт флаг --no-relaunch, поэтому
		// сюда не попадает.
		if platform.TaskExists() {
			if err := platform.RunElevatedTask(); err == nil {
				return
			}
		}
		if err := platform.RelaunchElevated(paths.LibDir); err != nil {
			println("elevation failed:", err.Error())
		}
		return
	}

	svc := backend.NewService(paths)

	// Один раз создаём задание планировщика (Windows), чтобы последующие запуски
	// обходились без UAC. Требует прав администратора.
	if !bindingsMode && platform.IsAdmin() {
		if err := platform.EnsureElevatedTask(paths.LibDir); err != nil {
			println("scheduled task:", err.Error())
		}
	}

	platform.SetupWebviewEnv()

	// Если включён автозапуск — сразу подключаемся к сохранённым сетям, а не
	// просто открываем окно.
	if !bindingsMode {
		svc.AutoConnect()
	}

	err := wails.Run(&options.App{
		Title:     "PotatoNet",
		Width:     900,
		Height:    680,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 13, B: 13, A: 1},
		OnShutdown:       func(ctx context.Context) { svc.Shutdown() },
		Linux: &linux.Options{
			ProgramName:      "PotatoNet",
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
		Windows: &windowsopts.Options{
			// Elevated-процесс + WebView2: задаём явный каталог данных и
			// отключаем GPU и RendererCodeIntegrity — иначе браузерный процесс
			// может падать (белое окно / «WebView2 crashed»), особенно с
			// антивирусами и средствами endpoint-защиты.
			WebviewUserDataPath:                 filepath.Join(paths.LibDir, "webview2"),
			WebviewGpuIsDisabled:                true,
			WebviewDisableRendererCodeIntegrity: true,
		},
		Bind: []interface{}{
			svc,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
