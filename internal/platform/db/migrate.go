package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" driver for database/sql (used by goose)
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
	"github.com/pressly/goose/v3/lock"

	"github.com/sarga-app/sarga/internal/platform/config"
	"github.com/sarga-app/sarga/migrations"
)

func openMigrationDB(cfg config.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", buildDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("open migration db: %w", err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func newProvider(db *sql.DB) (*goose.Provider, error) {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("init session locker: %w", err)
	}
	return goose.NewProvider(database.DialectPostgres, db, migrations.FS,
		goose.WithSessionLocker(locker),
	)
}

func MigrateUp(ctx context.Context, cfg config.DatabaseConfig) error {
	db, err := openMigrationDB(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			slog.Warn("close migration db", "err", cerr)
		}
	}()

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
	return nil
}

func MigrateDown(ctx context.Context, cfg config.DatabaseConfig) error {
	db, err := openMigrationDB(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			slog.Warn("close migration db", "err", cerr)
		}
	}()

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

func MigrateStatus(ctx context.Context, cfg config.DatabaseConfig) ([]MigrationStatus, error) {
	db, err := openMigrationDB(cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			slog.Warn("close migration db", "err", cerr)
		}
	}()

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
