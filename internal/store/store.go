package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps the Postgres connection pool and provides health checks.
type Store struct {
	pool *pgxpool.Pool
}

// New opens a connection pool to the database at dsn.
func New(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Pool returns the underlying pgxpool for direct queries.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// Health checks the database is reachable.
func (s *Store) Health(ctx context.Context) error {
	var n int
	return s.pool.QueryRow(ctx, "SELECT 1").Scan(&n)
}

// Migrate runs all .sql files in migrations/ in order. Returns the number of
// statements executed. For P0, this is a simple sequential runner that does
// not track applied migrations; a full migration framework is out of scope.
func (s *Store) Migrate(ctx context.Context, sql string) error {
	_, err := s.pool.Exec(ctx, sql)
	return err
}
