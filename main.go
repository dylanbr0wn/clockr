package main

import (
	"context"
	"embed"
	"log"
	"net/http"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/dylanbr0wn/shiet/internal/api/appapi"
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/db"
	applog "github.com/dylanbr0wn/shiet/internal/log"
	"github.com/dylanbr0wn/shiet/internal/seed"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger, logCloser, err := applog.Open(cfg.Log.Path, cfg.Log.Level, wailsDevConsole)
	if err != nil {
		log.Fatalf("open log: %v", err)
	}
	defer logCloser.Close()
	logger.Info().
		Str("path", cfg.Log.Path).
		Str("level", cfg.Log.Level).
		Bool("console", wailsDevConsole).
		Msg("app starting")

	// Open the local database, self-migrate to the latest schema, and seed core
	// data before binding so the service is live when the frontend calls it.
	conn, err := db.Open(cfg.DB.Path)
	if err != nil {
		logger.Fatal().Err(err).Msg("open database")
	}
	if err := db.Migrate(conn); err != nil {
		logger.Fatal().Err(err).Msg("migrate database")
	}
	if err := seed.Core(context.Background(), conn); err != nil {
		logger.Fatal().Err(err).Msg("seed database")
	}

	// Create an instance of the app structure
	app := NewApp(conn, cfg, logger)

	// Create application with options
	err = wails.Run(&options.App{
		Title:     "wails-base-fresh",
		Width:     1280,
		Height:    768,
		MinWidth:  1024,
		MinHeight: 680,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Handler: http.StripPrefix("/rpc", appapi.NewHandler(appapi.Dependencies{
				Service:         app.Svc,
				SyncPeriod:      wrapSyncPeriod(logger, app.Svc.SyncPeriod),
				ListConnections: app.registry.List,
				RefreshGitHubRepos: func(ctx context.Context, accountID string) error {
					_, err := app.github.SyncRepos(ctx, accountID)
					return err
				},
				RefreshSlackChannels: func(ctx context.Context, accountID string) error {
					_, err := app.slack.SyncChannels(ctx, accountID)
					return err
				},
				RefreshBitbucketResources: func(ctx context.Context, accountID string) error {
					_, _, err := app.bitbucket.SyncWorkspacesRepos(ctx, accountID)
					return err
				},
				Google: app.google,
				GitHub: app.github,
				Slack:  app.slack,
				Bitbucket: app.bitbucket,
			})),
		},
		Frameless:        false,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		logger.Error().Err(err).Msg("wails run failed")
	}
}
