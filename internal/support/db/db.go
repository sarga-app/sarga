package db

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"

	_ "modernc.org/sqlite"
)

const (
	busyTimeout = 5000
	cacheSize   = -65536    // 64 MiB page cache per pool
	mmapSize    = 268435456 // 256 MiB read-only mmap window
)

// DB holds a single-writer + multi-reader pool pair for SQLite WAL mode.
type DB struct {
	write *sql.DB
	read  *sql.DB
}

func (d *DB) Write() *sql.DB { return d.write }
func (d *DB) Read() *sql.DB  { return d.read }

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
	if err := write.Ping(); err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("ping write pool: %w", err)
	}

	read, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("open read pool: %w", err)
	}
	n := runtime.GOMAXPROCS(0) * 2
	if n < 4 {
		n = 4
	}
	read.SetMaxOpenConns(n)
	if err := read.Ping(); err != nil {
		_ = write.Close()
		_ = read.Close()
		return nil, fmt.Errorf("ping read pool: %w", err)
	}

	return &DB{write: write, read: read}, nil
}

func buildDSN(path string) string {
	return fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(%d)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=temp_store(memory)&_pragma=cache_size(%d)&_pragma=mmap_size(%d)",
		path, busyTimeout, cacheSize, mmapSize,
	)
}
