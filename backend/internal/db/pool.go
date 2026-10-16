// Package db opens the PostgreSQL pool and runs migrations.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"childconnect/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	// Neon's pooler shares server sessions between clients. Named prepared statements there can outlive a
	// schema reset ("cache lookup failed for type"), so use unnamed statements with a client-side describe cache.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe
	cfg.MaxConnIdleTime = 4 * time.Minute // Neon closes idle connections; recycle them first
	cfg.HealthCheckPeriod = time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pctx, cancel := context.WithTimeout(ctx, 20*time.Second) // the first request after a Neon pause is slow
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}
	return pool, nil
}

// Migrate applies the embedded goose migrations using the direct (non-pooled) connection.
func Migrate(url string) error {
	sqldb, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer sqldb.Close()
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(sqldb, ".")
}

// Reset drops everything and re-applies the migrations. Used only by the seed command.
func Reset(url string) error {
	sqldb, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer sqldb.Close()
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.DownTo(sqldb, ".", 0); err != nil {
		return err
	}
	return goose.Up(sqldb, ".")
}
