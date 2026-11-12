//go:build dev

package main

import "embed"

// assets is empty under the dev tag, so linting works without a frontend build.
var assets embed.FS
