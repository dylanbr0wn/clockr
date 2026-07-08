package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("00006_category_color.go", up00006CategoryColor, down00006CategoryColor)
}

func up00006CategoryColor(ctx context.Context, tx *sql.Tx) error {
	exists, err := categoryColumnExists(ctx, tx, "color")
	if err != nil {
		return err
	}
	if !exists {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE category
			ADD COLUMN color TEXT NOT NULL DEFAULT '#64748B';
		`); err != nil {
			return fmt.Errorf("add category.color: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE category SET color = CASE name
			WHEN 'Meetings' THEN '#0EA5E9'
			WHEN 'Deep Work' THEN '#8B5CF6'
			WHEN 'Admin' THEN '#F59E0B'
			WHEN 'Email & Comms' THEN '#14B8A6'
			WHEN 'Breaks' THEN '#64748B'
			ELSE '#64748B'
		END;
	`); err != nil {
		return fmt.Errorf("backfill category.color: %w", err)
	}
	return nil
}

func down00006CategoryColor(ctx context.Context, tx *sql.Tx) error {
	exists, err := categoryColumnExists(ctx, tx, "color")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	_, err = tx.ExecContext(ctx, `ALTER TABLE category DROP COLUMN color;`)
	if err != nil {
		return fmt.Errorf("drop category.color: %w", err)
	}
	return nil
}

func categoryColumnExists(ctx context.Context, tx *sql.Tx, column string) (bool, error) {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(category)`)
	if err != nil {
		return false, fmt.Errorf("inspect category schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("scan category schema: %w", err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("inspect category schema: %w", err)
	}
	return false, nil
}
