package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// deepseekBalance reads the active preset's key + base URL, calls DeepSeek's
// GET /user/balance, and reports the balance plus spend since the last poll.
func (s *server) deepseekBalance() (map[string]any, error) {
	cfg, err := loadMBConfig(s.root)
	if err != nil {
		return nil, err
	}
	key, base, ok := cfg.activePreset()
	if !ok {
		return nil, fmt.Errorf("no active preset in mb_config.json")
	}
	if key == "" {
		return nil, fmt.Errorf("当前模型未填写 API Key")
	}
	if base == "" {
		base = "https://api.deepseek.com"
	}

	// Derive the API origin from base_url, then append /user/balance.
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	balURL := strings.TrimRight(u.Scheme+"://"+u.Host, "/") + "/user/balance"

	req, _ := http.NewRequest(http.MethodGet, balURL, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed struct {
		IsAvailable  bool `json:"is_available"`
		BalanceInfos []struct {
			Currency     string `json:"currency"`
			TotalBalance string `json:"total_balance"`
		} `json:"balance_infos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.BalanceInfos) == 0 {
		return nil, fmt.Errorf("balance response had no balance_infos (HTTP %d)", resp.StatusCode)
	}
	info := parsed.BalanceInfos[0]
	balance, _ := strconv.ParseFloat(info.TotalBalance, 64)

	hasPrev := s.prevBal != nil
	lastSpend := 0.0
	if hasPrev {
		lastSpend = math.Round((*s.prevBal-balance)*10000) / 10000
	}
	s.prevBal = &balance

	return map[string]any{
		"ok":        true,
		"balance":   balance,
		"currency":  info.Currency,
		"available": parsed.IsAvailable,
		"hasPrev":   hasPrev,
		"lastSpend": lastSpend,
	}, nil
}

// openCodex launches `codex app <workdir>` with CODEX_HOME pointed at the
// repo-local .codex so it routes through Moon Bridge regardless of the global toggle.
func (s *server) openCodex() (map[string]any, error) {
	appPath, workdir := s.codexAppPath, s.codexWorkdir
	if cfg, err := loadMBConfig(s.root); err == nil {
		if cfg.CodexAppPath != "" {
			appPath = cfg.CodexAppPath
		}
		if cfg.CodexWorkdir != "" {
			workdir = cfg.CodexWorkdir
		}
	}
	if appPath == "" {
		appPath = "codex"
	}
	if workdir == "" {
		workdir = os.Getenv("USERPROFILE")
	}
	if _, err := os.Stat(workdir); err != nil {
		return nil, fmt.Errorf("工作目录不存在: %s", workdir)
	}

	cmd := exec.Command(appPath, "app", workdir)
	cmd.Env = append(os.Environ(), "CODEX_HOME="+s.codexHome)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":        true,
		"path":      appPath,
		"workdir":   workdir,
		"codexHome": s.codexHome,
		"pid":       cmd.Process.Pid,
	}, nil
}
