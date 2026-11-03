package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// lockKeyPrefix namespaces the jobs' advisory locks, so they never collide with the transaction locks
// other features take on hashed keys.
const lockKeyPrefix = "rolecue.jobs."

// unlockTimeout bounds the release of a lock. It runs on a fresh context, so a run cancelled by shutdown
// still releases its lock.
const unlockTimeout = 5 * time.Second

// PostgresLocker takes one PostgreSQL session advisory lock per job and run. The lock lives on one
// pooled connection for the length of the run; it is released with pg_advisory_unlock, or by
// PostgreSQL itself if the connection or the process dies, so a crashed instance never blocks a job.
type PostgresLocker struct {
	pool *pgxpool.Pool
}

// NewPostgresLocker locks over the API's pool.
func NewPostgresLocker(pool *pgxpool.Pool) (*PostgresLocker, error) {
	if pool == nil {
		return nil, errors.New("jobs: a database pool is required")
	}
	return &PostgresLocker{pool: pool}, nil
}

// TryLock tries the job's lock without waiting.
func (l *PostgresLocker) TryLock(ctx context.Context, name string) (func(), bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	key := lockKeyPrefix + name
	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1, 0))", key).Scan(&acquired); err != nil {
		conn.Release()
		return nil, false, err
	}
	if !acquired {
		conn.Release()
		return nil, false, nil
	}
	unlock := func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), unlockTimeout)
		defer cancel()
		if _, err := conn.Exec(unlockCtx, "SELECT pg_advisory_unlock(hashtextextended($1, 0))", key); err != nil {
			// A connection that may still hold the lock must not go back to the pool: closing it ends
			// the database session, which releases the lock.
			_ = conn.Conn().Close(unlockCtx)
		}
		conn.Release()
	}
	return unlock, true, nil
}
