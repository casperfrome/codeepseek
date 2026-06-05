package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// newControlListener binds the control server to localhost only.
func newControlListener(port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
}

// isListening reports whether something accepts TCP connections at addr.
func isListening(addr string, timeout time.Duration) bool {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// startBridge launches the moonbridge child process. When noBrowser is set the
// window is hidden and stdout/stderr are redirected to log files in the root.
// Returns the new process PID.
func (s *server) startBridge() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	args := append(append([]string{}, s.mbPre...), "-config", "config.yml")
	cmd := exec.Command(s.mbCmd, args...)
	cmd.Dir = s.root

	var logs []*os.File
	if s.noBrowser {
		hideWindow(cmd)
		if out, err := os.Create(filepath.Join(s.root, "bridge.log")); err == nil {
			cmd.Stdout = out
			logs = append(logs, out)
		}
		if errf, err := os.Create(filepath.Join(s.root, "bridge.err.log")); err == nil {
			cmd.Stderr = errf
			logs = append(logs, errf)
		}
	} else {
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	}

	fmt.Printf("[control] starting bridge: %s %v\n", s.mbCmd, args)
	if err := cmd.Start(); err != nil {
		for _, f := range logs {
			_ = f.Close()
		}
		return 0, err
	}
	s.bridge = cmd

	// Reap the process and clear state when it exits.
	go func() {
		_ = cmd.Wait()
		for _, f := range logs {
			_ = f.Close()
		}
		s.mu.Lock()
		if s.bridge == cmd {
			s.bridge = nil
		}
		s.mu.Unlock()
	}()

	return cmd.Process.Pid, nil
}

// stopBridge terminates the bridge process tree, if running.
func (s *server) stopBridge() {
	s.mu.Lock()
	cmd := s.bridge
	s.bridge = nil
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}
	fmt.Printf("[control] stopping bridge pid %d\n", cmd.Process.Pid)
	killProcessTree(cmd)
}

// bridgeAlive reports whether the bridge process is currently running.
func (s *server) bridgeAlive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bridge != nil
}
