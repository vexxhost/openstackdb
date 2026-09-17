// Package openstackdb provides connections to OpenStack service databases.
package openstackdb

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"time"
)

// ConnectionOptions controls the pool. Zero values use bounded defaults.
type ConnectionOptions struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Connect opens and verifies a connection with a ten-second deadline.
// The caller owns the returned pool and must close it.
func Connect(connectionString string) (*sql.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return ConnectContext(ctx, connectionString, ConnectionOptions{})
}

// ConnectContext opens a pool from an oslo.db-style MySQL URL and verifies it
// within ctx's deadline. It does not grant or enforce database permissions.
func ConnectContext(ctx context.Context, connectionString string, opts ConnectionOptions) (*sql.DB, error) {
	if opts.MaxOpenConns < 0 || opts.MaxIdleConns < 0 || opts.ConnMaxLifetime < 0 {
		return nil, fmt.Errorf("connection options must not be negative")
	}
	dsn, err := ParseOsloDBConnectionString(connectionString)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if opts.MaxOpenConns == 0 {
		opts.MaxOpenConns = 5
	}
	if opts.MaxIdleConns == 0 {
		opts.MaxIdleConns = 2
	}
	db.SetMaxOpenConns(opts.MaxOpenConns)
	db.SetMaxIdleConns(opts.MaxIdleConns)
	db.SetConnMaxLifetime(opts.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return db, nil
}
