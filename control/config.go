package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// mbConfig mirrors the subset of mb_config.json the control server needs. The
// file is the editable source of truth for the panel; config.yml is generated
// from it. Unknown fields are ignored on read and preserved on save (save reuses
// the raw JSON the panel POSTs, so we never need a full round-trip model here).
type mbConfig struct {
	CodexHome        string `json:"codexHome"`
	GlobalCodexHome  string `json:"globalCodexHome"`
	NativeCodexModel string `json:"nativeCodexModel"`
	CodexAppPath     string `json:"codexAppPath"`
	CodexWorkdir     string `json:"codexWorkdir"`

	Active struct {
		Preset    string `json:"preset"`
		Reasoning string `json:"reasoning"`
	} `json:"active"`

	Presets map[string]struct {
		APIKey  string `json:"apiKey"`
		BaseURL string `json:"baseUrl"`
	} `json:"presets"`
}

// loadMBConfig reads and parses mb_config.json from the backend root.
func loadMBConfig(root string) (*mbConfig, error) {
	b, err := os.ReadFile(filepath.Join(root, "mb_config.json"))
	if err != nil {
		return nil, err
	}
	var c mbConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// activePreset returns the preset selected by active.preset, or false if absent.
func (c *mbConfig) activePreset() (apiKey, baseURL string, ok bool) {
	p, ok := c.Presets[c.Active.Preset]
	if !ok {
		return "", "", false
	}
	return p.APIKey, p.BaseURL, true
}
