// Package main is the entry point for the gitura desktop application.
package main

import (
	_ "embed"
	"errors"
	"flag"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gitura/internal/app"
)

// githubClientID is the GitHub OAuth app client ID.
// The value is baked in at build time via -ldflags "-X main.githubClientID=...".
// Falls back to a placeholder for local dev builds.
var githubClientID = "Ov23liFakeClientIDDev"

//go:embed build/appicon.png
var appIcon []byte

func main() {
	a := app.New(githubClientID)
	opts := application.Options{
		Name:        "gitura",
		Description: "Review GitHub pull requests",
		Services:    []application.Service{application.NewService(a)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Icon:        appIcon,
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	}
	if err := app.ConfigureLaunch(&opts, a, os.Args[1:], ".", os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		os.Exit(2)
	}
	app.Configure(&opts, a)

	wailsApp := application.New(opts)
	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      app.MainWindow,
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
		BackgroundColour: application.NewRGB(9, 9, 11),
		URL:              "/",
		Linux:            application.LinuxWindow{Icon: appIcon},
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
