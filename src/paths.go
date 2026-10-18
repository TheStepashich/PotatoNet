package backend

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"potatonet/src/platform"
)

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// DetectPaths по умолчанию хранит config.yml и EasyTier в пользовательском
// каталоге (DataDir). Портативный режим (рядом с исполняемым файлом) включается
// переменной POTATONET_PORTABLE=1 или файлом "portable" рядом с приложением.
func DetectPaths(override string) platform.Paths {
	if runtime.GOARCH != "amd64" {
		logf("Warning: unsupported architecture %s — EasyTier x86_64 required", runtime.GOARCH)
	}
	exeDir := filepath.Dir(platform.CurrentExe())

	baseDir := platform.DataDir()
	if v := strings.TrimSpace(os.Getenv("POTATONET_DATA_DIR")); v != "" {
		baseDir = v
	}
	if strings.TrimSpace(override) != "" {
		baseDir = strings.TrimSpace(override)
	}
	if os.Getenv("POTATONET_PORTABLE") != "" || fileExists(filepath.Join(exeDir, "portable")) {
		baseDir = exeDir
	}
	_ = os.MkdirAll(baseDir, 0o755)
	platform.ChownToUser(baseDir)
	etDir := filepath.Join(baseDir, platform.LocalEasyTierDir())
	return platform.Paths{
		ConfigFile:   filepath.Join(baseDir, "config.yml"),
		LibDir:       baseDir,
		BundleDir:    filepath.Join(exeDir, platform.LocalEasyTierDir()),
		EasyTierCore: filepath.Join(etDir, platform.CoreName()),
		EasyTierCLI:  filepath.Join(etDir, platform.CLIName()),
	}
}
