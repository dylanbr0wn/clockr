package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dylanbr0wn/clockr/internal/service"
)

// App struct
type App struct {
	ctx  context.Context
	conn *sql.DB
	Svc  *service.Service
}

// NewApp creates a new App over an already-open database connection. The
// connection is opened, migrated, and seeded in main before binding, so Svc is
// live at bind time (Wails reflects bound instances up front).
func NewApp(conn *sql.DB) *App {
	return &App{
		conn: conn,
		Svc:  service.New(conn),
	}
}

// startup is called when the app starts; saves the context for runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called on app exit; close the database cleanly.
func (a *App) shutdown(ctx context.Context) {
	if a.conn != nil {
		_ = a.conn.Close()
	}
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
