package main

import (
	"flag"
	"os"
)

func main() {
	runF := flag.Bool("run", false, "Start all networks (downloads EasyTier on first run)")
	stopF := flag.Bool("stop", false, "Stop all networks")
	peersF := flag.Bool("peers", false, "Show connected peers")
	serverF := flag.Bool("server", false, "Run local API server (default)")
	dataDirF := flag.String("data-dir", "", "Data directory (config.yml, EasyTier)")

	flag.Parse()

	// совместимость со старым синтаксисом: `pnet run` эквивалентно `pnet -run`
	cmd := ""
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
	}

	paths := detectPaths(*dataDirF)
	if len(os.Args[1:]) == 0 {
		runServer(paths)
		return
	}

	if *runF || cmd == "run" {
		CheckErr(cmdRun(paths))
	} else if *stopF || cmd == "stop" {
		cmdStop(paths)
	} else if *peersF || cmd == "peers" {
		cmdPeers(paths)
	} else if *serverF || cmd == "server" || cmd == "gui" {
		runServer(paths)
	} else {
		flag.PrintDefaults()
		os.Exit(1)
	}
}
