//go:build dev

// Package main is the entry point for the gitura desktop application.
package main

import "embed"

// assets is unused in dev mode; Wails serves frontend files directly.
var assets embed.FS
