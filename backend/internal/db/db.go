package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

type PoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

func Connect(ctx context.Context, url string, poolConfig PoolConfig) (*DB, error) {
	poolConfig = applyPoolConfigDefaults(poolConfig)

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}

	config.MaxConns = poolConfig.MaxConns
	config.MinConns = poolConfig.MinConns
	config.MaxConnLifetime = poolConfig.MaxConnLifetime
	config.MaxConnIdleTime = poolConfig.MaxConnIdleTime
	config.HealthCheckPeriod = poolConfig.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}

	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

func applyPoolConfigDefaults(poolConfig PoolConfig) PoolConfig {
	if poolConfig.MaxConns <= 0 {
		poolConfig.MaxConns = 40
	}
	if poolConfig.MinConns <= 0 {
		poolConfig.MinConns = 5
	}
	if poolConfig.MaxConnLifetime <= 0 {
		poolConfig.MaxConnLifetime = 30 * time.Minute
	}
	if poolConfig.MaxConnIdleTime <= 0 {
		poolConfig.MaxConnIdleTime = 10 * time.Minute
	}
	if poolConfig.HealthCheckPeriod <= 0 {
		poolConfig.HealthCheckPeriod = time.Minute
	}
	return poolConfig
}
