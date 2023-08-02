package dblock

import (
	"context"
	"time"
)

// DefaultClient setup the default client.
var DefaultClient Client

// Obtain tries to obtain a new lock using a key with the given TTL.
// May return ErrNotObtained if not successful,
// or ErrNoProviders if no providers registers.
func Obtain(ctx context.Context, key string, ttl time.Duration, optionsFns ...OptionsFn) (Lock, error) {
	if DefaultClient == nil {
		return nil, ErrNoProviders
	}

	return DefaultClient.Obtain(ctx, key, ttl, optionsFns...)
}
