package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime"

	_ "modernc.org/sqlite" // register sqlite driver

	"github.com/sarga-app/sarga/internal/db/queries"
)

const (
	busyTimeout = 5000
	cacheSize   = -65536    // 64 MiB page cache per pool
	mmapSize    = 268435456 // 256 MiB read-only mmap window
)

type DB struct {
	write *sql.DB
	read  *sql.DB
	rq    *queries.Queries
	wq    *queries.Queries
}

func (d *DB) Write() *sql.DB      { return d.write }
func (d *DB) Read() *sql.DB       { return d.read }
func (d *DB) R() *queries.Queries { return d.rq }
func (d *DB) W() *queries.Queries { return d.wq }

func (d *DB) Ping(ctx context.Context) error {
	return d.read.PingContext(ctx)
}

func (d *DB) Close() error {
	werr := d.write.Close()
	rerr := d.read.Close()
	if werr != nil {
		return werr
	}
	return rerr
}

func New(path string) (*DB, error) {
	dsn := buildDSN(path)

	write, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open write pool: %w", err)
	}
	write.SetMaxOpenConns(1)
	if err = write.PingContext(context.Background()); err != nil {
		write.Close() //nolint:errcheck // cleanup on error path
		return nil, fmt.Errorf("ping write pool: %w", err)
	}

	read, err := sql.Open("sqlite", dsn)
	if err != nil {
		write.Close() //nolint:errcheck // cleanup on error path
		return nil, fmt.Errorf("open read pool: %w", err)
	}
	n := max(runtime.GOMAXPROCS(0)*2, 4)
	read.SetMaxOpenConns(n)
	if err = read.PingContext(context.Background()); err != nil {
		write.Close() //nolint:errcheck // cleanup on error path
		read.Close()  //nolint:errcheck // cleanup on error path
		return nil, fmt.Errorf("ping read pool: %w", err)
	}

	slog.Info("db: pools ready", "path", path, "read_conns", n, "write_conns", 1)
	return &DB{
		write: write,
		read:  read,
		rq:    queries.New(read),
		wq:    queries.New(write),
	}, nil
}

func buildDSN(path string) string {
	return fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(%d)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=temp_store(memory)&_pragma=cache_size(%d)&_pragma=mmap_size(%d)",
		path, busyTimeout, cacheSize, mmapSize,
	)
}
