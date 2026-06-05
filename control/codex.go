package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Line patterns matching mb_control.ps1's codex TOML editing rules.
var (
	reModel          = regexp.MustCompile(`^\s*model\s*=`)
	reModelProvider  = regexp.MustCompile(`^\s*model_provider\s*=`)
	reCtxWindow      = regexp.MustCompile(`^\s*model_context_window\s*=`)
	reMaxOutput      = regexp.MustCompile(`^\s*model_max_output_tokens\s*=`)
	reCatalog        = regexp.MustCompile(`^\s*model_catalog_json\s*=`)
	reProviderHeader = regexp.MustCompile(`^\s*\[model_providers\.moonbridge\]\s*$`)
	reAnyTable       = regexp.MustCompile(`^\s*\[`)
	reProviderActive = regexp.MustCompile(`(?m)^\s*model_provider\s*=\s*"moonbridge"`)
	reLineSplit      = regexp.MustCompile(`\r?\n`)
)

// runMoonbridge runs the bridge CLI with the given args (cwd = root) and returns stdout.
func (s *server) runMoonbridge(args ...string) (string, error) {
	full := append(append([]string{}, s.mbPre...), args...)
	cmd := exec.Command(s.mbCmd, full...)
	cmd.Dir = s.root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("%s %v failed: %v: %s", s.mbCmd, full, err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// regenCodex mirrors 3_gen_codex_config.bat: read the model alias, render the
// Codex config.toml against the local bridge, and write it into codexHome.
func (s *server) regenCodex() (string, error) {
	if err := os.MkdirAll(s.codexHome, 0o755); err != nil {
		return "", err
	}
	modelOut, err := s.runMoonbridge("-print-codex-model", "-config", "config.yml")
	if err != nil {
		return "", err
	}
	model := strings.TrimSpace(firstLine(modelOut))
	if model == "" {
		return "", fmt.Errorf("could not read codex model alias from config.yml")
	}
	toml, err := s.runMoonbridge("-print-codex-config", model,
		"-codex-base-url", "http://"+s.bridgeAddr()+"/v1",
		"-codex-home", s.codexHome, "-config", "config.yml")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(s.codexHome, "config.toml"), []byte(toml), 0o644); err != nil {
		return "", err
	}
	return model, nil
}

// codexMode reports the routing of the user's GLOBAL ~/.codex/config.toml.
func (s *server) codexMode() string {
	b, err := os.ReadFile(s.globalCodexToml())
	if err != nil {
		return "unknown"
	}
	if reProviderActive.Match(b) {
		return "bridge"
	}
	return "native"
}

// backupGlobalCodexToml saves a timestamped copy before any edit.
func (s *server) backupGlobalCodexToml() error {
	b, err := os.ReadFile(s.globalCodexToml())
	if err != nil {
		return nil // nothing to back up
	}
	stamp := time.Now().Format("20060102-150405")
	return os.WriteFile(s.globalCodexToml()+".mb-bak-"+stamp, b, 0o644)
}

// removeMoonbridgeProviderTable drops the whole [model_providers.moonbridge]
// table (header until the next [table] or EOF).
func removeMoonbridgeProviderTable(lines []string) []string {
	out := make([]string, 0, len(lines))
	skip := false
	for _, ln := range lines {
		if reProviderHeader.MatchString(ln) {
			skip = true
			continue
		}
		if skip {
			if reAnyTable.MatchString(ln) {
				skip = false // next table ends the skip
			} else {
				continue
			}
		}
		out = append(out, ln)
	}
	return out
}

// setCodexNative points the global Codex at the native GPT model.
func (s *server) setCodexNative() error {
	b, err := os.ReadFile(s.globalCodexToml())
	if err != nil {
		return fmt.Errorf("global codex config not found: %s", s.globalCodexToml())
	}
	if err := s.backupGlobalCodexToml(); err != nil {
		return err
	}
	lines := removeMoonbridgeProviderTable(reLineSplit.Split(string(b), -1))
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		switch {
		case reModel.MatchString(ln):
			out = append(out, `model = "`+s.nativeCodexModel+`"`)
		case reModelProvider.MatchString(ln), reCtxWindow.MatchString(ln),
			reMaxOutput.MatchString(ln), reCatalog.MatchString(ln):
			// drop
		default:
			out = append(out, ln)
		}
	}
	return os.WriteFile(s.globalCodexToml(), []byte(strings.Join(out, "\r\n")), 0o644)
}

// setCodexBridge routes the global Codex through Moon Bridge, rebuilding the
// routing keys and [model_providers.moonbridge] table cleanly.
func (s *server) setCodexBridge() error {
	b, err := os.ReadFile(s.globalCodexToml())
	if err != nil {
		return fmt.Errorf("global codex config not found: %s", s.globalCodexToml())
	}
	if err := s.backupGlobalCodexToml(); err != nil {
		return err
	}
	catalog := filepath.Join(s.codexHome, "models_catalog.json")
	lines := removeMoonbridgeProviderTable(reLineSplit.Split(string(b), -1))

	// Drop existing routing keys so we re-emit them in a known-good block.
	rest := make([]string, 0, len(lines))
	for _, ln := range lines {
		if reModel.MatchString(ln) || reModelProvider.MatchString(ln) ||
			reCtxWindow.MatchString(ln) || reMaxOutput.MatchString(ln) || reCatalog.MatchString(ln) {
			continue
		}
		rest = append(rest, ln)
	}

	// Routing keys must sit at the top, before any [table] header.
	head := []string{
		`model = "moonbridge"`,
		`model_provider = "moonbridge"`,
		`model_context_window = 1000000`,
		`model_max_output_tokens = 384000`,
		`model_catalog_json = "` + strings.ReplaceAll(catalog, `\`, `\\`) + `"`,
	}
	body := strings.Join(append(head, rest...), "\r\n")
	providerBlock := strings.Join([]string{
		"",
		"[model_providers.moonbridge]",
		`name = "Moon Bridge"`,
		`base_url = "http://` + s.bridgeAddr() + `/v1"`,
		`wire_api = "responses"`,
	}, "\r\n")

	final := strings.TrimRight(body, "\r\n") + "\r\n" + providerBlock + "\r\n"
	return os.WriteFile(s.globalCodexToml(), []byte(final), 0o644)
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}
