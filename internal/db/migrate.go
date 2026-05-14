package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"

	"github.com/sarga-app/sarga/migrations"
)

func openMigrationDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open migration db: %w", err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func newProvider(db *sql.DB) (*goose.Provider, error) {
	p, err := goose.NewProvider(database.DialectSQLite3, db, migrations.FS)
	if err != nil {
		return nil, fmt.Errorf("init goose provider: %w", err)
	}
	return p, nil
}

func MigrateUp(ctx context.Context, path string) error {
	db, err := openMigrationDB(path)
	if err != nil {
		return err
	}
	defer closeDB(db)

	p, err := newProvider(db)
	if err != nil {
		return err
	}
	results, err := p.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	if len(results) == 0 {
		slog.Info("migrations: schema already up to date")
	} else {
		slog.Info("migrations: applied",
			"count", len(results),
			"from", results[0].Source.Version,
			"to", results[len(results)-1].Source.Version,
		)
	}
	if _, err := db.ExecContext(ctx, "ANALYZE"); err != nil {
		slog.Warn("migrations: ANALYZE failed", "err", err)
	}
	return nil
}

func MigrateDown(ctx context.Context, path string) error {
	db, err := openMigrationDB(path)
	if err != nil {
		return err
	}
	defer closeDB(db)

	p, err := newProvider(db)
	if err != nil {
		return err
	}
	if _, err := p.Down(ctx); err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

type MigrationStatus struct {
	Version int64
	Name    string
	Applied bool
}

func MigrateStatus(ctx context.Context, path string) ([]MigrationStatus, error) {
	db, err := openMigrationDB(path)
	if err != nil {
		return nil, err
	}
	defer closeDB(db)

	p, err := newProvider(db)
	if err != nil {
		return nil, err
	}
	statuses, err := p.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate status: %w", err)
	}
	out := make([]MigrationStatus, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, MigrationStatus{
			Version: s.Source.Version,
			Name:    s.Source.Path,
			Applied: s.State == goose.StateApplied,
		})
	}
	return out, nil
}

func closeDB(db *sql.DB) {
	if err := db.Close(); err != nil {
		slog.Warn("close migration db", "err", err)
	}
}
