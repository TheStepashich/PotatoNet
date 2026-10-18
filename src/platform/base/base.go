package base

import (
	"os"
	"path/filepath"
)

type Paths struct {
	ConfigFile   string
	LibDir       string
	BundleDir    string
	EasyTierCore string
	EasyTierCLI  string
}

func CurrentExe() string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	if p, err := filepath.Abs(exe); err == nil {
		return p
	}
	return exe
}

func PidDir() string {
	dir := filepath.Join(os.TempDir(), "PotatoNet")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// IsWritable сообщает, можно ли писать в каталог. Нужно, чтобы отличить
// портативный запуск (папка рядом с exe) от установленного (AppImage,
// Program Files), где данные надо складывать в пользовательский каталог.
func IsWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".potatonet-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
