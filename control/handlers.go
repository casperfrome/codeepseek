package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

// writeJSON encodes obj as compact JSON with the given status code.
func writeJSON(w http.ResponseWriter, code int, obj any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(obj)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"ok": false, "message": msg})
}

// requirePOST writes a 405 and returns false if the method is not POST.
func requirePOST(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return false
	}
	return true
}

// handleState serves mb_config.json verbatim (the panel's source of truth).
func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	b, err := os.ReadFile(s.configJson())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(b)
}

// handleStatus reports whether the bridge process is alive and its port is listening.
func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"processAlive": s.bridgeAlive(),
		"listening":    isListening(s.bridgeAddr(), 300*time.Millisecond),
		"addr":         s.bridgeAddr(),
	})
}

// handleSave persists config.yml (backed up first) and mb_config.json from the
// panel payload {yaml, json}. All files are written UTF-8 without a BOM.
func (s *server) handleSave(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var payload struct {
		Yaml string          `json:"yaml"`
		JSON json.RawMessage `json:"json"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if payload.Yaml == "" {
		writeError(w, http.StatusBadRequest, "missing yaml")
		return
	}

	// Back up the existing config.yml, then write the new one.
	if cur, err := os.ReadFile(s.configYml()); err == nil {
		_ = os.WriteFile(s.configYml()+".bak", cur, 0o644)
	}
	if err := os.WriteFile(s.configYml(), []byte(payload.Yaml), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Re-indent the JSON sub-document (preserves key order) and persist it.
	if len(payload.JSON) > 0 {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, payload.JSON, "", "  "); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json field: "+err.Error())
			return
		}
		if err := os.WriteFile(s.configJson(), pretty.Bytes(), 0o644); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "saved"})
}

// handleRestart stops and restarts the bridge child process.
func (s *server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	s.stopBridge()
	time.Sleep(restartGap)
	pid, err := s.startBridge()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pid": pid})
}

// handleRegenCodex regenerates the repo-local Codex config.toml via moonbridge.
func (s *server) handleRegenCodex(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	model, err := s.regenCodex()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"model":  model,
		"output": "config.toml + models_catalog.json -> " + s.codexHome,
	})
}

// handleCodexMode reports whether the global Codex routes to native GPT or the bridge.
func (s *server) handleCodexMode(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"mode":        s.codexMode(),
		"home":        s.globalCodexHome,
		"nativeModel": s.nativeCodexModel,
	})
}

func (s *server) handleCodexNative(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	if err := s.setCodexNative(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "mode": "native", "model": s.nativeCodexModel, "home": s.globalCodexHome,
	})
}

func (s *server) handleCodexBridge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	if err := s.setCodexBridge(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mode": "bridge", "home": s.globalCodexHome})
}

func (s *server) handleDeepseekBalance(w http.ResponseWriter, r *http.Request) {
	res, err := s.deepseekBalance()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *server) handleOpenCodex(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	res, err := s.openCodex()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
