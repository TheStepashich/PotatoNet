package backend

import (
	"flag"
	"fmt"
	"os"
)

// Main — точка входа CLI-режима (headless): запуск/остановка сетей,
// локальный API-сервер, автозапуск. GUI-окно (Wails) живёт в корневом пакете.
func Main(args []string) {
	fs := flag.NewFlagSet("PotatoNet", flag.ExitOnError)
	versionF := fs.Bool("version", false, "Print version and exit")
	runF := fs.Bool("run", false, "Start all networks (downloads EasyTier on first run)")
	stopF := fs.Bool("stop", false, "Stop all networks")
	autostartF := fs.Bool("autostart", false, "Enable autostart")
	noAutostartF := fs.Bool("no-autostart", false, "Disable autostart")
	peersF := fs.Bool("peers", false, "Show connected peers")
	serverF := fs.Bool("server", false, "Run local API server")
	dataDirF := fs.String("data-dir", "", "Data directory (config.yml, EasyTier)")
	_ = fs.Parse(args)

	// совместимость со старым синтаксисом: `PotatoNet run` эквивалентно `PotatoNet -run`
	cmd := ""
	if fs.NArg() > 0 {
		cmd = fs.Arg(0)
	}

	paths := DetectPaths(*dataDirF)

	switch {
	case *versionF || cmd == "version":
		fmt.Println(Version)
	case *runF || cmd == "run":
		CheckErr(cmdRun(paths))
	case *stopF || cmd == "stop":
		cmdStop(paths)
	case *autostartF || cmd == "autostart":
		CheckErr(CmdAutostart(true))
	case *noAutostartF || cmd == "no-autostart":
		CheckErr(CmdAutostart(false))
	case *peersF || cmd == "peers":
		cmdPeers(paths)
	case *serverF || cmd == "server":
		runServer(paths)
	default:
		fs.PrintDefaults()
		os.Exit(1)
	}
}
