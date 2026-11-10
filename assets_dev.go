//go:build dev

package main

import "embed"

// assets is unused in dev mode; Wails serves frontend files directly.
var assets embed.FS
