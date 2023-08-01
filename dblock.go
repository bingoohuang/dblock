package dblock

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotObtained is returned when a lock cannot be obtained.
	ErrNotObtained = errors.New("dblock: not obtained")

	// ErrLockNotHeld is returned when trying to release an inactive lock.
	ErrLockNotHeld = errors.New("dblock: lock not held")
)

type Client interface {
	// Obtain tries to obtain a new lock using a key with the given TTL.
	// May return ErrNotObtained if not successful.
	Obtain(ctx context.Context, key string, ttl time.Duration, opt *Options) (Lock, error)
}

// Lock represents an obtained, distributed lock.
type Lock interface {
	// Token returns the token value set by the lock.
	Token() string

	// Metadata returns the metadata of the lock.
	Metadata() string

	// TTL returns the remaining time-to-live. Returns 0 if the lock has expired.
	TTL(ctx context.Context) (time.Duration, error)
	// Refresh extends the lock with a new TTL.
	// May return ErrNotObtained if refresh is unsuccessful.
	Refresh(ctx context.Context, ttl time.Duration) error
	// Release manually releases the lock.
	// May return ErrLockNotHeld.
	Release(ctx context.Context) error
}

// Options describe the options for the lock
type Options struct {
	// RetryStrategy allows to customise the lock retry strategy.
	// Default: do not retry
	RetryStrategy RetryStrategy

	// Meta string.
	Meta string

	// Token is a unique value that is used to identify the lock. By default, a random tokens are generated. Use this
	// option to provide a custom token instead.
	Token string
}

func (o *Options) GetRetryStrategy() RetryStrategy {
	if o.RetryStrategy != nil {
		return o.RetryStrategy
	}
	return NoRetry()
}
