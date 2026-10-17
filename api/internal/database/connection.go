package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/cog-forge/rolecue/api/internal/config"
)

// NewDatabasePool initializes pgxpool.Pool from DatabaseConfig
func NewDatabasePool(cfg *config.DatabaseConfig) (*pgxpool.Pool, error) {
	pgxConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse database pool config: %w", err)
	}

	// Directly configure pgx connection fields from structured config if URL was not specified
	if cfg.URL == "" {
		if cfg.Host != "" {
			pgxConfig.ConnConfig.Host = cfg.Host
		}
		if cfg.Port > 0 {
			pgxConfig.ConnConfig.Port = uint16(cfg.Port)
		}
		if cfg.User != "" {
			pgxConfig.ConnConfig.User = cfg.User
		}
		if cfg.Password != "" {
			pgxConfig.ConnConfig.Password = cfg.Password
		}
		if cfg.Name != "" {
			pgxConfig.ConnConfig.Database = strings.TrimPrefix(cfg.Name, "/")
		}
	}

	if cfg.MaxOpenConns > 0 {
		pgxConfig.MaxConns = int32(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		pgxConfig.MinConns = int32(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		pgxConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// NewDatabasePoolWithDSN initializes pgxpool.Pool from a raw connection string
func NewDatabasePoolWithDSN(connStr string) (*pgxpool.Pool, error) {
	pgxConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		return nil, err
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
