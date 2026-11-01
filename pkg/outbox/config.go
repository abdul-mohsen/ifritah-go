package outbox

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultInterval    = 500 * time.Millisecond
	defaultBatchSize   = 50
	defaultLease       = time.Minute
	defaultMaxAttempts = 10
	maxBatchSize       = 1000
	maxAttempts        = 1000
	maxOwnerLen        = 128
)

// DrainerFromEnv builds the drainer for this process from the environment, and returns the interval
// between drains. owner names the process in the leases. OUTBOX_INTERVAL, OUTBOX_BATCH_SIZE,
// OUTBOX_LEASE and OUTBOX_MAX_ATTEMPTS override the defaults: 500ms, 50, 1m and 10.
func DrainerFromEnv(db *sql.DB, pub Publisher, owner string) (*Drainer, time.Duration, error) {
	interval, err := durationEnv("OUTBOX_INTERVAL", defaultInterval)
	if err != nil {
		return nil, 0, err
	}
	batch, err := intEnv("OUTBOX_BATCH_SIZE", defaultBatchSize)
	if err != nil {
		return nil, 0, err
	}
	lease, err := durationEnv("OUTBOX_LEASE", defaultLease)
	if err != nil {
		return nil, 0, err
	}
	attempts, err := intEnv("OUTBOX_MAX_ATTEMPTS", defaultMaxAttempts)
	if err != nil {
		return nil, 0, err
	}
	switch {
	case owner == "" || len(owner) > maxOwnerLen:
		return nil, 0, fmt.Errorf("outbox: owner must be 1 to %d characters", maxOwnerLen)
	case interval <= 0:
		return nil, 0, errors.New("outbox: OUTBOX_INTERVAL must be positive")
	case batch <= 0 || batch > maxBatchSize:
		return nil, 0, fmt.Errorf("outbox: OUTBOX_BATCH_SIZE must be 1 to %d", maxBatchSize)
	case lease <= 0:
		return nil, 0, errors.New("outbox: OUTBOX_LEASE must be positive")
	case attempts <= 0 || attempts > maxAttempts:
		return nil, 0, fmt.Errorf("outbox: OUTBOX_MAX_ATTEMPTS must be 1 to %d", maxAttempts)
	}
	return &Drainer{
		DB:          db,
		Publisher:   pub,
		Owner:       owner,
		BatchSize:   int32(batch),
		Lease:       lease,
		MaxAttempts: uint32(attempts),
	}, interval, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("outbox: %s: %w", key, err)
	}
	return d, nil
}

func intEnv(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("outbox: %s: %w", key, err)
	}
	return n, nil
}
