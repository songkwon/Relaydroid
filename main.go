package main

import (
	"context"
	"embed"
	"log"
	"time"

	"github.com/songkwon/relaydroid/internal/transfer"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/Relaydroid-icon.png
var appIcon []byte

func main() {
	service := &Relaydroid{jobs: make(map[string]job)}
	app := application.New(application.Options{
		Name:        "Relaydroid",
		Description: "Android 文件传输",
		Icon:        appIcon,
		Assets:      application.AssetOptions{Handler: application.BundledAssetFileServer(assets)},
		Services:    []application.Service{application.NewService(service)},
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	service.app = app
	app.OnShutdown(func() {
		if err := service.StopLegacyWiFi(context.Background()); err != nil {
			log.Print(err)
		}
		_ = service.FinderUnmount()
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		transfer.ReleaseMTP(ctx)
	})
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Relaydroid",
		Width:            1200,
		Height:           780,
		MinWidth:         850,
		MinHeight:        610,
		BackgroundColour: application.NewRGB(246, 247, 245),
		EnableFileDrop:   true,
		Mac:              application.MacWindow{TitleBar: application.MacTitleBarHiddenInset, InvisibleTitleBarHeight: 28},
	})
	service.window = window
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		app.Event.Emit("relay:drop", e.Context().DroppedFiles())
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
