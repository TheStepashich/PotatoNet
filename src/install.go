package backend

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"potatonet/src/platform"
)

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(tmp, mode)
	}
	return os.Rename(tmp, dst)
}

// Prepare копирует бинарники EasyTier из комплекта в каталог данных. Вызывать
// до повышения прав: в AppImage комплект лежит в read-only FUSE-монтировании,
// недоступном root.
func Prepare(paths platform.Paths) error {
	return installEasyTier(paths)
}

// installEasyTier кладёт бинарники EasyTier (core + cli) из комплекта
// приложения в каталог данных. В AppImage комплект лежит в read-only
// FUSE-монтировании, откуда root-бэкенд не может их запускать, поэтому файлы
// копируются пользовательским процессом ещё до запуска с правами root.
// Никакого скачивания из сети нет.
func installEasyTier(paths platform.Paths) error {
	destDir := filepath.Join(paths.LibDir, platform.LocalEasyTierDir())
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{platform.CoreName(), platform.CLIName(), "Packet.dll", "wintun.dll"} {
		src := filepath.Join(paths.BundleDir, name)
		dst := filepath.Join(destDir, name)

		srcInfo, serr := os.Stat(src)
		dstInfo, derr := os.Stat(dst)

		switch {
		case serr == nil && (derr != nil || dstInfo.Size() != srcInfo.Size()):
			if err := copyFile(src, dst, 0o755); err != nil {
				return fmt.Errorf("не удалось установить EasyTier: %v", err)
			}
		case derr != nil && name != "Packet.dll" && name != "wintun.dll":
			return fmt.Errorf("EasyTier не найден (комплект: %s, данные: %s)", src, dst)
		}
	}
	platform.ChownToUser(paths.LibDir)
	return nil
}
