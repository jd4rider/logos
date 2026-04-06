package main

import _ "embed"

//go:embed scripts/setup-runtime.sh
var setupRuntimeShell string

//go:embed scripts/setup-runtime.ps1
var setupRuntimePowerShell string

//go:embed scripts/kokoro_speak.py
var kokoroSpeakScript string
