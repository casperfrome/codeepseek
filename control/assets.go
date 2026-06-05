package main

import _ "embed"

// panelHTML is the codeepseek config panel, embedded so the control server is self-contained.
//
//go:embed assets/config_tool.html
var panelHTML []byte
