package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	service := &Relay{jobs: make(map[string]job)}
	app := application.New(application.Options{
		Name:        "Relay",
		Description: "Android 文件传输",
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
	})
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Relay · 渡线",
		Width:            1200,
		Height:           780,
		MinWidth:         850,
		MinHeight:        610,
		BackgroundColour: application.NewRGB(246, 247, 245),
		EnableFileDrop:   true,
		Mac:              application.MacWindow{TitleBar: application.MacTitleBarHiddenInset, InvisibleTitleBarHeight: 48},
	})
	service.window = window
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		app.Event.Emit("relay:drop", e.Context().DroppedFiles())
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
