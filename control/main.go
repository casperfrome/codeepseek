// Command mbcontrol is the codeepseek control server: a small HTTP server that
// serves the config panel, owns the moonbridge bridge child process, and exposes
// the JSON API the panel uses to switch models / restart / regen Codex config.
//
// It is a Go port of mb_control.ps1 and is normally launched by MoonBridgeSwitcher
// (which passes -root <backend dir> -no-browser); it can also be run standalone.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	bridgeHost = "127.0.0.1"
	bridgePort = 38440 // the moonbridge bridge listens here
)

// server holds the resolved configuration and live process state.
type server struct {
	root      string // codeepseek backend directory
	port      int    // control port (default 38450)
	noBrowser bool

	mbCmd string   // moonbridge.exe path, or "go"
	mbPre []string // prefix args before -config (e.g. run ./cmd/moonbridge)

	// Codex settings, read from mb_config.json at startup (with fallbacks).
	codexHome        string
	globalCodexHome  string
	nativeCodexModel string
	codexAppPath     string
	codexWorkdir     string

	mu      sync.Mutex
	bridge  *exec.Cmd // current bridge process (nil when stopped)
	prevBal *float64  // last DeepSeek balance, for spend-since-last-poll
}

func (s *server) configYml() string  { return filepath.Join(s.root, "config.yml") }
func (s *server) configJson() string { return filepath.Join(s.root, "mb_config.json") }
func (s *server) bridgeAddr() string { return fmt.Sprintf("%s:%d", bridgeHost, bridgePort) }
func (s *server) globalCodexToml() string {
	return filepath.Join(s.globalCodexHome, "config.toml")
}

func main() {
	var (
		root      = flag.String("root", "", "codeepseek backend directory (default: current working directory)")
		port      = flag.Int("port", 38450, "control server port")
		noBrowser = flag.Bool("no-browser", false, "do not open the panel in a browser; hide the bridge window")
	)
	flag.Parse()

	r := *root
	if r == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("[control] cannot determine working directory: %v", err)
		}
		r = cwd
	}
	r, err := filepath.Abs(r)
	if err != nil {
		log.Fatalf("[control] invalid root: %v", err)
	}

	s := &server{
		root:      r,
		port:      *port,
		noBrowser: *noBrowser || os.Getenv("MB_NO_BROWSER") != "",
		// defaults; overridden below from mb_config.json
		codexHome:        filepath.Join(r, ".codex"),
		globalCodexHome:  filepath.Join(os.Getenv("USERPROFILE"), ".codex"),
		nativeCodexModel: "gpt-5.5",
		codexAppPath:     "codex",
	}

	// Required backend files.
	for _, f := range []string{s.configYml(), s.configJson()} {
		if _, err := os.Stat(f); err != nil {
			log.Fatalf("[control] missing %s (is -root the codeepseek folder?)", f)
		}
	}

	// Codex settings come from mb_config.json (best-effort).
	if cfg, err := loadMBConfig(s.root); err == nil {
		if cfg.CodexHome != "" {
			s.codexHome = cfg.CodexHome
		}
		if cfg.GlobalCodexHome != "" {
			s.globalCodexHome = cfg.GlobalCodexHome
		}
		if cfg.NativeCodexModel != "" {
			s.nativeCodexModel = cfg.NativeCodexModel
		}
		if cfg.CodexAppPath != "" {
			s.codexAppPath = cfg.CodexAppPath
		}
		if cfg.CodexWorkdir != "" {
			s.codexWorkdir = cfg.CodexWorkdir
		}
	}

	// Bridge invocation: prefer the compiled exe, else fall back to `go run`.
	if _, err := os.Stat(filepath.Join(s.root, "moonbridge.exe")); err == nil {
		s.mbCmd, s.mbPre = filepath.Join(s.root, "moonbridge.exe"), nil
	} else {
		s.mbCmd, s.mbPre = "go", []string{"run", "./cmd/moonbridge"}
	}

	prefix := fmt.Sprintf("http://127.0.0.1:%d/", s.port)
	ln, err := newControlListener(s.port)
	if err != nil {
		log.Fatalf("[control] FAILED to bind %s - port %d in use?\n%v", prefix, s.port, err)
	}

	s.startBridge()
	fmt.Println("============================================")
	fmt.Printf("  codeepseek control panel: %s\n", prefix)
	fmt.Printf("  Bridge target: %s\n", s.bridgeAddr())
	fmt.Println("  Press Ctrl+C to stop the bridge + panel.")
	fmt.Println("============================================")
	if !s.noBrowser {
		openBrowser(prefix)
	}

	// Stop the bridge on Ctrl+C / termination.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("[control] shutting down...")
		s.stopBridge()
		os.Exit(0)
	}()

	if err := http.Serve(ln, s.routes()); err != nil {
		s.stopBridge()
		log.Fatalf("[control] server error: %v", err)
	}
}

// routes wires the panel + JSON API. Paths and behavior mirror mb_control.ps1.
func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(panelHTML)
		default:
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "message": "not found: " + r.URL.Path})
		}
	})

	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/save", s.handleSave)
	mux.HandleFunc("/api/restart", s.handleRestart)
	mux.HandleFunc("/api/regen-codex", s.handleRegenCodex)
	mux.HandleFunc("/api/codex-mode", s.handleCodexMode)
	mux.HandleFunc("/api/codex-native", s.handleCodexNative)
	mux.HandleFunc("/api/codex-bridge", s.handleCodexBridge)
	mux.HandleFunc("/api/deepseek-balance", s.handleDeepseekBalance)
	mux.HandleFunc("/api/open-codex", s.handleOpenCodex)

	return logRequests(mux)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[control] %s %s -> PANIC: %v", r.Method, r.URL.Path, rec)
				writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": fmt.Sprint(rec)})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// brief pause used by /api/restart between stop and start.
const restartGap = 400 * time.Millisecond
