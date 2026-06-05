package main

import _ "embed"

// panelHTML is the Moon Bridge config panel, vendored from the moon-bridge repo
// (config_tool.html) and embedded so the control server is self-contained.
//
//go:embed assets/config_tool.html
var panelHTML []byte
