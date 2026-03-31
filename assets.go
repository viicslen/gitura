//go:build !dev

// Package main is the entry point for the gitura desktop application.
package main

import "embed"

//go:embed all:frontend/dist
var assets embed.FS
