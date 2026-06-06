// mbcontrol — Go replacement for mb_control.ps1.
//
// Serves the config-panel HTML, owns the moonbridge child process, and exposes
// the same JSON API that the panel uses to switch models / restart / regen Codex.
//
// Build:  go build -o mbcontrol.exe ./control
// Run:    mbcontrol.exe -root <backend-dir> [-no-browser]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ---- constants --------------------------------------------------------------

const (
	controlPort = 38450
	bridgeAddr  = "127.0.0.1:38440"
)

// ---- config types -----------------------------------------------------------

type mbConfig struct {
	CodexHome        string            `json:"codexHome"`
	GlobalCodexHome  string            `json:"globalCodexHome"`
	NativeCodexModel string            `json:"nativeCodexModel"`
	CodexAppPath     string            `json:"codexAppPath"`
	CodexWorkdir     string            `json:"codexWorkdir"`
	Active           activeField       `json:"active"`
	Presets          map[string]preset `json:"presets"`
}

type activeField struct {
	Preset string `json:"preset"`
}

type preset struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl"`
}

// ---- globals ----------------------------------------------------------------

var (
	root      string
	noBrowser bool

	bridgeMu sync.Mutex
	bridgeP  *os.Process

	prevBalMu sync.Mutex
	prevBal   *float64
)

// ---- main -------------------------------------------------------------------

func main() {
	flag.StringVar(&root, "root", "", "backend root directory (contains config.yml and mb_config.json)")
	flag.BoolVar(&noBrowser, "no-browser", false, "do not open the panel in a browser on start")
	flag.Parse()

	if root == "" {
		exe, _ := os.Executable()
		root = filepath.Dir(exe)
	}
	var err error
	if root, err = filepath.Abs(root); err != nil {
		log.Fatalf("[control] cannot resolve root: %v", err)
	}

	if os.Getenv("MB_NO_BROWSER") != "" {
		noBrowser = true
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", controlPort))
	if err != nil {
		log.Fatalf("[control] FAILED to bind port %d — already in use? %v", controlPort, err)
	}

	if err := startBridge(); err != nil {
		log.Printf("[control] WARNING: could not start bridge: %v", err)
	}

	panelURL := fmt.Sprintf("http://127.0.0.1:%d/", controlPort)
	fmt.Println("============================================")
	fmt.Printf("  Moon Bridge control panel: %s\n", panelURL)
	fmt.Printf("  Bridge target:             %s\n", bridgeAddr)
	fmt.Println("  KEEP THIS RUNNING. Ctrl+C stops everything.")
	fmt.Println("============================================")

	if !noBrowser {
		_ = exec.Command("cmd", "/c", "start", panelURL).Start()
	}

	go handleSignals(ln)

	mux := http.NewServeMux()
	mux.HandleFunc("/", routeRoot)
	mux.HandleFunc("/api/state", routeState)
	mux.HandleFunc("/api/status", routeStatus)
	mux.HandleFunc("/api/save", routeSave)
	mux.HandleFunc("/api/restart", routeRestart)
	mux.HandleFunc("/api/regen-codex", routeRegenCodex)
	mux.HandleFunc("/api/codex-mode", routeCodexMode)
	mux.HandleFunc("/api/codex-native", routeCodexNative)
	mux.HandleFunc("/api/codex-bridge", routeCodexBridge)
	mux.HandleFunc("/api/deepseek-balance", routeDeepseekBalance)
	mux.HandleFunc("/api/open-codex", routeOpenCodex)

	log.Fatal((&http.Server{Handler: mux}).Serve(ln))
}

func handleSignals(ln net.Listener) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("[control] shutting down...")
	stopBridge()
	ln.Close()
	os.Exit(0)
}

// ---- bridge process management ----------------------------------------------

func bridgeExe() string {
	return filepath.Join(root, "moonbridge.exe")
}

func startBridge() error {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()

	exe := bridgeExe()
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("moonbridge.exe not found in %s", root)
	}

	var cmd *exec.Cmd
	if noBrowser {
		cmd = exec.Command(exe, "-config", "config.yml")
		logOut, _ := os.OpenFile(filepath.Join(root, "bridge.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		logErr, _ := os.OpenFile(filepath.Join(root, "bridge.err.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		cmd.Stdout = logOut
		cmd.Stderr = logErr
	} else {
		cmd = exec.Command(exe, "-config", "config.yml")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	cmd.Dir = root
	// Windows: CREATE_NO_WINDOW so no console box appears when hosted.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}

	log.Printf("[control] starting bridge: %s -config config.yml", exe)
	if err := cmd.Start(); err != nil {
		return err
	}
	bridgeP = cmd.Process
	return nil
}

func stopBridge() {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	if bridgeP == nil {
		return
	}
	log.Printf("[control] stopping bridge pid %d", bridgeP.Pid)
	// Kill the whole process tree so `go run`-style child exes are also cleaned up.
	_ = exec.Command("taskkill", "/PID", fmt.Sprint(bridgeP.Pid), "/T", "/F").Run()
	bridgeP = nil
}

func bridgeAlive() bool {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	if bridgeP == nil {
		return false
	}
	p, err := os.FindProcess(bridgeP.Pid)
	if err != nil {
		return false
	}
	// On Windows FindProcess always succeeds; send signal 0 equivalent doesn't exist,
	// so we use a non-blocking wait.
	state, err := p.Wait()
	if err != nil || state.Exited() {
		bridgeP = nil
		return false
	}
	return true
}

func isListening() bool {
	conn, err := net.DialTimeout("tcp", bridgeAddr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ---- config helpers ---------------------------------------------------------

func configYmlPath() string  { return filepath.Join(root, "config.yml") }
func configJsonPath() string { return filepath.Join(root, "mb_config.json") }
func htmlPath() string       { return filepath.Join(root, "config_tool.html") }

func readMBConfig() (mbConfig, error) {
	var cfg mbConfig
	data, err := os.ReadFile(configJsonPath())
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func defaults(cfg *mbConfig) {
	home, _ := os.UserHomeDir()
	if cfg.CodexHome == "" {
		cfg.CodexHome = filepath.Join(home, ".codex")
	}
	if cfg.GlobalCodexHome == "" {
		cfg.GlobalCodexHome = filepath.Join(home, ".codex")
	}
	if cfg.NativeCodexModel == "" {
		cfg.NativeCodexModel = "gpt-4o"
	}
	if cfg.CodexAppPath == "" {
		cfg.CodexAppPath = "codex"
	}
	if cfg.CodexWorkdir == "" {
		cfg.CodexWorkdir = home
	}
}

// ---- HTTP helpers -----------------------------------------------------------

func sendJSON(w http.ResponseWriter, code int, v any) {
	b, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(b)
}

func readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

func methodOnly(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		sendJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "message": method + " only"})
		return false
	}
	return true
}

// ---- route handlers ---------------------------------------------------------

func routeRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		sendJSON(w, http.StatusNotFound, map[string]any{"ok": false, "message": "not found: " + r.URL.Path})
		return
	}
	data, err := os.ReadFile(htmlPath())
	if err != nil {
		http.Error(w, "config_tool.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func routeState(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(configJsonPath())
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)
}

func routeStatus(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"processAlive": bridgeAlive(),
		"listening":    isListening(),
		"addr":         bridgeAddr,
	})
}

func routeSave(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, "POST") {
		return
	}
	body, err := readBody(r)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	var payload struct {
		YAML string          `json:"yaml"`
		JSON json.RawMessage `json:"json"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.YAML == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "missing yaml"})
		return
	}
	// Backup config.yml before overwriting.
	if _, err := os.Stat(configYmlPath()); err == nil {
		_ = copyFile(configYmlPath(), configYmlPath()+".bak")
	}
	if err := os.WriteFile(configYmlPath(), []byte(payload.YAML), 0644); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	jsonBytes, _ := json.MarshalIndent(payload.JSON, "", "  ")
	if err := os.WriteFile(configJsonPath(), jsonBytes, 0644); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	sendJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "saved"})
}

func routeRestart(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, "POST") {
		return
	}
	stopBridge()
	time.Sleep(400 * time.Millisecond)
	if err := startBridge(); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	bridgeMu.Lock()
	pid := 0
	if bridgeP != nil {
		pid = bridgeP.Pid
	}
	bridgeMu.Unlock()
	sendJSON(w, http.StatusOK, map[string]any{"ok": true, "pid": pid})
}

func routeRegenCodex(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, "POST") {
		return
	}
	cfg, err := readMBConfig()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	defaults(&cfg)

	exe := bridgeExe()
	// Get model alias.
	modelOut, err := exec.Command(exe, "-print-codex-model", "-config", "config.yml").Output()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "moonbridge -print-codex-model: " + err.Error()})
		return
	}
	model := strings.TrimSpace(string(modelOut))
	if model == "" {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "could not read codex model alias from config.yml"})
		return
	}

	if err := os.MkdirAll(cfg.CodexHome, 0755); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	tomlOut, err := exec.Command(exe,
		"-print-codex-config", model,
		"-codex-base-url", "http://127.0.0.1:38440/v1",
		"-codex-home", cfg.CodexHome,
		"-config", "config.yml",
	).Output()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "moonbridge -print-codex-config: " + err.Error()})
		return
	}

	tomlPath := filepath.Join(cfg.CodexHome, "config.toml")
	if err := os.WriteFile(tomlPath, tomlOut, 0644); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	sendJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"model":    model,
		"output":   fmt.Sprintf("config.toml + models_catalog.json -> %s", cfg.CodexHome),
	})
}

func routeCodexMode(w http.ResponseWriter, r *http.Request) {
	cfg, err := readMBConfig()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	defaults(&cfg)
	mode := getCodexMode(cfg.GlobalCodexHome)
	sendJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"mode":        mode,
		"home":        cfg.GlobalCodexHome,
		"nativeModel": cfg.NativeCodexModel,
	})
}

func routeCodexNative(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, "POST") {
		return
	}
	cfg, err := readMBConfig()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	defaults(&cfg)
	if err := setCodexNative(cfg.GlobalCodexHome, cfg.NativeCodexModel); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	sendJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"mode":  "native",
		"model": cfg.NativeCodexModel,
		"home":  cfg.GlobalCodexHome,
	})
}

func routeCodexBridge(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, "POST") {
		return
	}
	cfg, err := readMBConfig()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	defaults(&cfg)
	if err := setCodexBridge(cfg.GlobalCodexHome, cfg.CodexHome); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	sendJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"mode": "bridge",
		"home": cfg.GlobalCodexHome,
	})
}

func routeDeepseekBalance(w http.ResponseWriter, r *http.Request) {
	cfg, err := readMBConfig()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	p, ok := cfg.Presets[cfg.Active.Preset]
	if !ok || p.APIKey == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "当前模型未填写 API Key"})
		return
	}

	origin := "https://api.deepseek.com"
	if p.BaseURL != "" {
		if u, err := url.Parse(p.BaseURL); err == nil {
			origin = u.Scheme + "://" + u.Host
		}
	}
	balURL := strings.TrimRight(origin, "/") + "/user/balance"

	req, _ := http.NewRequest("GET", balURL, nil)
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		sendJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	defer resp.Body.Close()
	var body struct {
		IsAvailable  bool `json:"is_available"`
		BalanceInfos []struct {
			TotalBalance float64 `json:"total_balance"`
			Currency     string  `json:"currency"`
		} `json:"balance_infos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		sendJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	if len(body.BalanceInfos) == 0 {
		sendJSON(w, http.StatusOK, map[string]any{"ok": true, "balance": 0, "available": body.IsAvailable})
		return
	}
	info := body.BalanceInfos[0]

	prevBalMu.Lock()
	hasPrev := prevBal != nil
	lastSpend := 0.0
	if hasPrev {
		lastSpend = math2Round(*prevBal-info.TotalBalance, 4)
	}
	bal := info.TotalBalance
	prevBal = &bal
	prevBalMu.Unlock()

	sendJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"balance":   info.TotalBalance,
		"currency":  info.Currency,
		"available": body.IsAvailable,
		"hasPrev":   hasPrev,
		"lastSpend": lastSpend,
	})
}

func routeOpenCodex(w http.ResponseWriter, r *http.Request) {
	if !methodOnly(w, r, "POST") {
		return
	}
	cfg, err := readMBConfig()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	defaults(&cfg)

	if _, err := os.Stat(cfg.CodexWorkdir); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "工作目录不存在: " + cfg.CodexWorkdir})
		return
	}

	cmd := exec.Command(cfg.CodexAppPath, "app", cfg.CodexWorkdir)
	cmd.Env = append(os.Environ(), "CODEX_HOME="+cfg.CodexHome)
	if err := cmd.Start(); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	sendJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"path":      cfg.CodexAppPath,
		"workdir":   cfg.CodexWorkdir,
		"codexHome": cfg.CodexHome,
		"pid":       cmd.Process.Pid,
	})
}

// ---- Codex TOML manipulation ------------------------------------------------
// Text-based; we do not import a TOML library to keep the binary small and avoid
// adding a new dependency that isn't already in go.mod.

var reModelLine             = regexp.MustCompile(`(?m)^\s*model\s*=.*$`)
var reModelProvider         = regexp.MustCompile(`(?m)^\s*model_provider\s*=.*$`)
var reModelContextWindow    = regexp.MustCompile(`(?m)^\s*model_context_window\s*=.*$`)
var reModelMaxOutputTokens  = regexp.MustCompile(`(?m)^\s*model_max_output_tokens\s*=.*$`)
var reModelCatalogJSON      = regexp.MustCompile(`(?m)^\s*model_catalog_json\s*=.*$`)
var reMoonbridgeTable       = regexp.MustCompile(`(?m)^\s*\[model_providers\.moonbridge\]`)
var reAnyTable              = regexp.MustCompile(`(?m)^\s*\[`)

func globalTomlPath(globalHome string) string {
	return filepath.Join(globalHome, "config.toml")
}

func getCodexMode(globalHome string) string {
	tomlPath := globalTomlPath(globalHome)
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return "unknown"
	}
	if regexp.MustCompile(`(?m)^\s*model_provider\s*=\s*"moonbridge"`).MatchString(string(data)) {
		return "bridge"
	}
	return "native"
}

func backupToml(tomlPath string) {
	if _, err := os.Stat(tomlPath); err == nil {
		stamp := time.Now().Format("20060102-150405")
		_ = copyFile(tomlPath, tomlPath+".mb-bak-"+stamp)
	}
}

// removeMoonbridgeTable removes the [model_providers.moonbridge] section.
func removeMoonbridgeTable(text string) string {
	loc := reMoonbridgeTable.FindStringIndex(text)
	if loc == nil {
		return text
	}
	start := loc[0]
	// Find the start of the next [table] after this one.
	rest := text[loc[1]:]
	nextTable := reAnyTable.FindStringIndex(rest)
	if nextTable == nil {
		// No next table — drop to end of file.
		return strings.TrimRight(text[:start], "\r\n")
	}
	return text[:start] + rest[nextTable[0]:]
}

// dropRoutingKeys removes all model-routing key lines from text.
func dropRoutingKeys(text string) string {
	text = reModelProvider.ReplaceAllString(text, "")
	text = reModelContextWindow.ReplaceAllString(text, "")
	text = reModelMaxOutputTokens.ReplaceAllString(text, "")
	text = reModelCatalogJSON.ReplaceAllString(text, "")
	return text
}

func setCodexNative(globalHome, nativeModel string) error {
	tomlPath := globalTomlPath(globalHome)
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return fmt.Errorf("global codex config not found: %s", tomlPath)
	}
	backupToml(tomlPath)
	text := removeMoonbridgeTable(string(data))
	text = dropRoutingKeys(text)
	// Replace or add the model line.
	if reModelLine.MatchString(text) {
		text = reModelLine.ReplaceAllString(text, `model = "`+nativeModel+`"`)
	} else {
		text = `model = "` + nativeModel + `"` + "\r\n" + text
	}
	return os.WriteFile(tomlPath, []byte(text), 0644)
}

func setCodexBridge(globalHome, codexHome string) error {
	tomlPath := globalTomlPath(globalHome)
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return fmt.Errorf("global codex config not found: %s", tomlPath)
	}
	backupToml(tomlPath)
	catalogPath := filepath.Join(codexHome, "models_catalog.json")
	catalogJSON := strings.ReplaceAll(catalogPath, `\`, `\\`)

	text := removeMoonbridgeTable(string(data))
	// Drop existing routing keys and model line — we re-emit them at the top.
	text = reModelLine.ReplaceAllString(text, "")
	text = dropRoutingKeys(text)

	head := strings.Join([]string{
		`model = "moonbridge"`,
		`model_provider = "moonbridge"`,
		`model_context_window = 1000000`,
		`model_max_output_tokens = 384000`,
		`model_catalog_json = "` + catalogJSON + `"`,
	}, "\r\n")
	providerBlock := strings.Join([]string{
		"",
		"[model_providers.moonbridge]",
		`name = "Moon Bridge"`,
		`base_url = "http://127.0.0.1:38440/v1"`,
		`wire_api = "responses"`,
	}, "\r\n")

	body := strings.TrimRight(head+"\r\n"+text, "\r\n")
	return os.WriteFile(tomlPath, []byte(body+"\r\n"+providerBlock+"\r\n"), 0644)
}

// ---- utilities --------------------------------------------------------------

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func math2Round(x float64, prec int) float64 {
	p := 1.0
	for i := 0; i < prec; i++ {
		p *= 10
	}
	return float64(int64(x*p+0.5)) / p
}
