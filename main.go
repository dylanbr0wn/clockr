package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/dylanbr0wn/clockr/internal/db"
	"github.com/dylanbr0wn/clockr/internal/seed"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Open the local database, self-migrate to the latest schema, and seed core
	// data before binding so the service is live when the frontend calls it.
	conn, err := db.OpenDefault()
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	if err := seed.Core(context.Background(), conn); err != nil {
		log.Fatalf("seed database: %v", err)
	}

	// Create an instance of the app structure
	app := NewApp(conn)

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "wails-base-fresh",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
			app.Svc,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
