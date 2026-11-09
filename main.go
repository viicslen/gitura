// Package main is the entry point for the gitura desktop application.
package main

import (
	_ "embed"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed build/appicon.png
var appIcon []byte

func main() {
	configureLinuxWebView()

	app := NewApp()
	assetMiddleware := assetserver.Middleware(assetProxyMiddleware(app))

	err := wails.Run(&options.App{
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
		AssetServer:      &assetserver.Options{Assets: assets, Middleware: assetMiddleware},
		BackgroundColour: &options.RGBA{R: 9, G: 9, B: 11, A: 1},
		// Wails only defaults the GPU policy to Never when Linux options are nil; keep it for GPUs that fail EGL/GBM.
		Linux:         &linux.Options{Icon: appIcon, WebviewGpuPolicy: linux.WebviewGpuPolicyNever},
		OnStartup:     app.startup,
		OnDomReady:    app.domReady,
		OnBeforeClose: app.beforeClose,
		Bind:          []interface{}{app},
	})

	if err != nil {
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
