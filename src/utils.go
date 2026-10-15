package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func CheckErr(err error) {
	if err != nil {
		fmt.Printf("err: %s\n", err.Error())
		os.Exit(1)
	}
}

type LogFunc func(format string, a ...any)

var logf LogFunc = func(format string, a ...any) {
	fmt.Printf(format+"\n", a...)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, msg string) {
	writeJSON(w, map[string]any{"ok": true, "message": msg})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	writeJSON(w, map[string]any{"ok": false, "error": msg})
}

func background(fn func() error, doneMsg string) {
	go func() {
		if err := fn(); err != nil {
			logf("Error: %v", err)
			return
		}
		logf("%s", doneMsg)
	}()
}
