//go:build !windows

package main

import "os/exec"

// The control server targets Windows (moonbridge.exe, taskkill, ~/.codex paths).
// These stubs exist only so the module compiles and `go vet` runs on other OSes.

func hideWindow(cmd *exec.Cmd) {}

func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func openBrowser(url string) {
	_ = exec.Command("xdg-open", url).Start()
}
