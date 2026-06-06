package server

import (
	"testing"

	"moonbridge/internal/config"
	"moonbridge/internal/service/provider"
	"moonbridge/internal/service/runtime"
)

// TestResolveModelOrFallbackUsesDefaultForUnknownModel verifies that an unknown
// model name (e.g. Codex's plan-mode helper "gpt-5.4-mini") falls back to the
// configured default model instead of failing with model_not_found, which is
// what previously broke Codex plan mode when routed through DeepSeek.
func TestResolveModelOrFallbackUsesDefaultForUnknownModel(t *testing.T) {
	pm, err := provider.NewProviderManager(map[string]provider.ProviderConfig{
		"deepseek": {
			BaseURL: "https://api.deepseek.com/anthropic",
			APIKey:  "test-key",
		},
	}, map[string]provider.ModelRoute{
		"moonbridge": {Provider: "deepseek", Name: "deepseek-v4-pro"},
	})
	if err != nil {
		t.Fatalf("NewProviderManager() error = %v", err)
	}

	cfg := config.Config{DefaultModel: "moonbridge"}
	srv := &Server{runtime: runtime.NewRuntime(cfg, pm, nil), providerMgr: pm}

	// Unknown model: should fall back to the default ("moonbridge" -> deepseek-v4-pro).
	route, err := srv.resolveModelOrFallback("gpt-5.4-mini")
	if err != nil {
		t.Fatalf("resolveModelOrFallback(unknown) error = %v, want fallback success", err)
	}
	pref, ok := route.Preferred()
	if !ok {
		t.Fatal("resolved fallback route has no preferred candidate")
	}
	if pref.ProviderKey != "deepseek" || pref.UpstreamModel != "deepseek-v4-pro" {
		t.Fatalf("fallback candidate = %s/%s, want deepseek/deepseek-v4-pro", pref.ProviderKey, pref.UpstreamModel)
	}

	// Known model still resolves directly (fallback path must not interfere).
	if _, err := srv.resolveModelOrFallback("moonbridge"); err != nil {
		t.Fatalf("resolveModelOrFallback(known) error = %v", err)
	}
}

// TestResolveModelOrFallbackNoDefaultStillErrors verifies that when no default
// model is configured, an unknown model still surfaces the original error
// rather than silently succeeding.
func TestResolveModelOrFallbackNoDefaultStillErrors(t *testing.T) {
	pm, err := provider.NewProviderManager(map[string]provider.ProviderConfig{
		"deepseek": {BaseURL: "https://api.deepseek.com/anthropic", APIKey: "test-key"},
	}, map[string]provider.ModelRoute{
		"moonbridge": {Provider: "deepseek", Name: "deepseek-v4-pro"},
	})
	if err != nil {
		t.Fatalf("NewProviderManager() error = %v", err)
	}

	// No DefaultModel set -> DefaultModelAlias() is empty -> no fallback.
	srv := &Server{runtime: runtime.NewRuntime(config.Config{}, pm, nil), providerMgr: pm}

	if _, err := srv.resolveModelOrFallback("gpt-5.4-mini"); err == nil {
		t.Fatal("resolveModelOrFallback(unknown) with no default = nil error, want failure")
	}
}
