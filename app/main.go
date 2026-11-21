package main

import (
	"context"
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:dist
var assets embed.FS

func main() {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		log.Fatal(err)
	}
	bridge := NewBridge()
	err = wails.Run(&options.App{
		Title:       "Shikigami",
		Width:       1200,
		Height:      800,
		MinWidth:    800,
		MinHeight:   500,
		AssetServer: &assetserver.Options{Assets: sub},
		// Samakan dengan --bg tema gelap (#1b1b1f).
		BackgroundColour: options.NewRGBA(27, 27, 31, 255),
		Menu:             buildMenu(bridge),
		DragAndDrop:      &options.DragAndDrop{EnableFileDrop: true},
		OnStartup:        func(ctx context.Context) { bridge.SetContext(ctx) },
		OnShutdown:       func(ctx context.Context) { _ = bridge.Close() },
		Bind:             []any{bridge},
	})
	if err != nil {
		log.Fatalf("gagal menjalankan aplikasi: %v", err)
	}
}
