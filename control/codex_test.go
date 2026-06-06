package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetCodexBridgePreservesNonRoutingConfig(t *testing.T) {
	dir := t.TempDir()
	globalHome := filepath.Join(dir, "global")
	codexHome := filepath.Join(dir, "bridge")
	if err := os.MkdirAll(globalHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}

	original := strings.Join([]string{
		`model = "gpt-5.5"`,
		`model_provider = "openai"`,
		`model_reasoning_effort = "high"`,
		`[plugins."browser@openai-bundled"]`,
		`enabled = true`,
		`[mcp_servers.node_repl]`,
		`command = "node_repl.exe"`,
		`[desktop]`,
		`localeOverride = "zh-CN"`,
	}, "\n")
	if err := os.WriteFile(filepath.Join(globalHome, "config.toml"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &server{
		globalCodexHome: globalHome,
		codexHome:       codexHome,
	}
	if err := s.setCodexBridge(); err != nil {
		t.Fatal(err)
	}

	gotBytes, err := os.ReadFile(filepath.Join(globalHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)

	for _, want := range []string{
		`model = "moonbridge"`,
		`model_provider = "moonbridge"`,
		`model_catalog_json = "` + strings.ReplaceAll(filepath.Join(codexHome, "models_catalog.json"), `\`, `\\`) + `"`,
		`wire_api = "responses"`,
		`[plugins."browser@openai-bundled"]`,
		`[mcp_servers.node_repl]`,
		`[desktop]`,
		`localeOverride = "zh-CN"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("config.toml missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `model_provider = "openai"`) {
		t.Fatalf("old model_provider was not replaced:\n%s", got)
	}
}

func TestStripConflictingCatalogBaseInstructionsOnlyRemovesGeneratedTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models_catalog.json")
	catalog := map[string]any{
		"models": []any{
			map[string]any{
				"slug":              "deepseek-v4-pro",
				"base_instructions": "You are deepseek-v4-pro, a coding agent.\nUnless the user explicitly asks for a plan, asks a question about the code, is brainstorming possible approaches, or otherwise makes clear that they do not want code changes yet, you assume they want you to make the change or run the tools needed to solve the problem.",
			},
			map[string]any{
				"slug":              "custom-model",
				"base_instructions": "Use the user's custom domain-specific instructions.",
			},
		},
	}
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := stripConflictingCatalogBaseInstructions(path); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Models []map[string]any `json:"models"`
	}
	gotBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(gotBytes, &got); err != nil {
		t.Fatal(err)
	}

	if _, ok := got.Models[0]["base_instructions"]; ok {
		t.Fatalf("generated base_instructions should have been removed: %#v", got.Models[0])
	}
	if got.Models[1]["base_instructions"] != "Use the user's custom domain-specific instructions." {
		t.Fatalf("custom base_instructions should be preserved: %#v", got.Models[1])
	}
}
