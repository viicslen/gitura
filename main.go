// Package main is the entry point for the gitura desktop application.
package main

import (
	_ "embed"
	"errors"
	"flag"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"gitura/internal/app"
)

// githubClientID is the GitHub OAuth app client ID.
// The value is baked in at build time via -ldflags "-X main.githubClientID=...".
// Falls back to a placeholder for local dev builds.
var githubClientID = "Ov23liFakeClientIDDev"

//go:embed build/appicon.png
var appIcon []byte

func main() {
	configureLinuxWebView()

	opts := &options.App{
		Title:     "gitura",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		// Wails on Linux caps an unset max size at the monitor the window opened on, which
		// blocks widening it on a larger monitor.
		MaxWidth:         16384,
		MaxHeight:        16384,
		Frameless:        true, // the app draws its own title strip and window controls
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 9, G: 9, B: 11, A: 1},
		// Wails only defaults the GPU policy to Never when Linux options are nil; keep it for GPUs that fail EGL/GBM.
		Linux: &linux.Options{Icon: appIcon, WebviewGpuPolicy: linux.WebviewGpuPolicyNever},
	}
	a := app.New(githubClientID)
	if err := app.ConfigureLaunch(opts, a, os.Args[1:], ".", os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		os.Exit(2)
	}
	app.Configure(opts, a)

	if err := wails.Run(opts); err != nil {
		println("Error:", err.Error())
	}
}

// configureLinuxWebView applies conservative WebKitGTK defaults for Linux GPUs
// that fail EGL/GBM initialisation and otherwise abort before rendering.
func configureLinuxWebView() {
	if runtime.GOOS != "linux" {
		return
	}

	if _, exists := os.LookupEnv("WEBKIT_DISABLE_COMPOSITING_MODE"); !exists {
		_ = os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")
	}
	if _, exists := os.LookupEnv("WEBKIT_DISABLE_DMABUF_RENDERER"); !exists {
		_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}
}
