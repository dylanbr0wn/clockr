// Command db is shiet's dev tooling for the local SQLite store: spin the
// schema up/down, inspect status, reset, and seed.
//
//	go run ./cmd/db <command> [flags]
//
// Target database: --db flag, else $SHIET_DB, else ./shiet.dev.db
// (NOT the app's real db).
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/seed"
)

const devDBDefault = "shiet.dev.db"
const migrationsSrcDir = "internal/db/migrations"

// CLI is the kong grammar for the db tool.
type CLI struct {
	DB string `help:"Path to the SQLite database." env:"SHIET_DB" default:"${devdb}" type:"path"`

	Up      UpCmd      `cmd:"" help:"Apply all pending migrations."`
	Down    DownCmd    `cmd:"" help:"Roll back the most recent migration."`
	Reset   ResetCmd   `cmd:"" help:"Roll every migration back down (destructive)."`
	Redo    RedoCmd    `cmd:"" help:"Re-run the latest migration (down one, then up)."`
	Status  StatusCmd  `cmd:"" help:"Print migration status."`
	Version VersionCmd `cmd:"" help:"Print the current schema version."`
	Create  CreateCmd  `cmd:"" help:"Scaffold a new SQL migration."`
	Seed    SeedCmd    `cmd:"" help:"Seed core data (use --dev for a sample period + calendar)."`
	Nuke    NukeCmd    `cmd:"" help:"Delete the database file (destructive)."`
}

func main() {
	cli := &CLI{}
	kctx := kong.Parse(cli,
		kong.Name("db"),
		kong.Description("shiet local SQLite dev tooling."),
		kong.Vars{"devdb": devDBDefault},
		kong.UsageOnError(),
	)
	kctx.FatalIfErrorf(kctx.Run(&appCtx{dbPath: cli.DB}))
}

// appCtx is bound into each command's Run; carries the resolved db path.
type appCtx struct {
	dbPath string
}

// open opens the target database (commands that need a connection call this).
func (c *appCtx) open() (*sql.DB, error) {
	return db.Open(c.dbPath)
}

type UpCmd struct{}

func (UpCmd) Run(c *appCtx) error {
	conn, err := c.open()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(conn); err != nil {
		return err
	}
	fmt.Printf("migrated up: %s\n", c.dbPath)
	return nil
}

type DownCmd struct{}

func (DownCmd) Run(c *appCtx) error {
	conn, err := c.open()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.MigrateDownOne(conn); err != nil {
		return err
	}
	fmt.Println("rolled back one migration")
	return nil
}

type ResetCmd struct{}

func (ResetCmd) Run(c *appCtx) error {
	conn, err := c.open()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Reset(conn); err != nil {
		return err
	}
	fmt.Println("reset all migrations")
	return nil
}

type RedoCmd struct{}

func (RedoCmd) Run(c *appCtx) error {
	conn, err := c.open()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.MigrateDownOne(conn); err != nil {
		return err
	}
	if err := db.Migrate(conn); err != nil {
		return err
	}
	fmt.Println("redid latest migration")
	return nil
}

type StatusCmd struct{}

func (StatusCmd) Run(c *appCtx) error {
	conn, err := c.open()
	if err != nil {
		return err
	}
	defer conn.Close()
	return db.Status(conn)
}

type VersionCmd struct{}

func (VersionCmd) Run(c *appCtx) error {
	conn, err := c.open()
	if err != nil {
		return err
	}
	defer conn.Close()
	v, err := db.Version(conn)
	if err != nil {
		return err
	}
	fmt.Printf("schema version: %d\n", v)
	return nil
}

type CreateCmd struct {
	Name string `arg:"" help:"Migration name (e.g. add_tags_table)."`
}

func (cmd CreateCmd) Run(*appCtx) error {
	if err := db.CreateMigration(migrationsSrcDir, cmd.Name); err != nil {
		return err
	}
	fmt.Println("created migration in", migrationsSrcDir)
	return nil
}

type SeedCmd struct {
	Dev bool `help:"Also seed a sample period + calendar for local development."`
}

func (cmd SeedCmd) Run(c *appCtx) error {
	conn, err := c.open()
	if err != nil {
		return err
	}
	defer conn.Close()
	// Migrate first so seeding always targets the current schema.
	if err := db.Migrate(conn); err != nil {
		return err
	}
	ctx := context.Background()
	if cmd.Dev {
		if err := seed.Dev(ctx, conn); err != nil {
			return err
		}
		fmt.Println("seeded core + dev sample data")
		return nil
	}
	if err := seed.Core(ctx, conn); err != nil {
		return err
	}
	fmt.Println("seeded core data")
	return nil
}

type NukeCmd struct{}

// Run removes the SQLite database plus its WAL/SHM sidecars.
func (NukeCmd) Run(c *appCtx) error {
	for _, p := range []string{c.dbPath, c.dbPath + "-wal", c.dbPath + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", filepath.Base(p), err)
		}
	}
	fmt.Println("deleted", c.dbPath)
	return nil
}
