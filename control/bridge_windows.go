//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW prevents a console window from popping up for the child.
const createNoWindow = 0x08000000

// hideWindow configures the child process to run without a visible window.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

// killProcessTree kills the process and all of its descendants. /T covers the
// temp child exe spawned by `go run`, mirroring mb_control.ps1's taskkill.
func killProcessTree(cmd *exec.Cmd) {
	pid := cmd.Process.Pid
	_ = exec.Command("taskkill", "/PID", fmt.Sprint(pid), "/T", "/F").Run()
}

// openBrowser opens url in the default browser via the shell file handler.
func openBrowser(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
