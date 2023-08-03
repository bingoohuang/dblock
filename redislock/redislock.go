package redislock

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/bingoohuang/dblock"
	"github.com/redis/go-redis/v9"
)

var (
	luaRefresh = redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("pexpire", KEYS[1], ARGV[2]) else return 0 end`)
	luaRelease = redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`)
	// PTTL returns the amount of remaining time in milliseconds.
	luaPTTL   = redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("pttl", KEYS[1]) else return -3 end`)
	luaObtain = redis.NewScript(`
if redis.call("set", KEYS[1], ARGV[1], "NX", "PX", ARGV[3]) then return redis.status_reply("OK") end

local offset = tonumber(ARGV[2])
if redis.call("getrange", KEYS[1], 0, offset-1) == string.sub(ARGV[1], 1, offset) then return redis.call("set", KEYS[1], ARGV[1], "PX", ARGV[3]) end
`)
)

// Obtain is a short-cut for New(...).Obtain(...).
func Obtain(ctx context.Context, client *redis.Client, key string, ttl time.Duration, optionsFns ...dblock.OptionsFn) (dblock.Lock, error) {
	return New(client).Obtain(ctx, key, ttl, optionsFns...)
}

// Client wraps a redis client.
type Client struct {
	client *redis.Client
}

// New creates a new Client instance with a custom namespace.
func New(client *redis.Client) *Client {
	return &Client{client: client}
}

type lockView struct {
	TokenMeta string
	time.Duration
}

func (l lockView) GetToken() string    { return l.TokenMeta }
func (l lockView) GetMetadata() string { return l.GetMetadata() }
func (l lockView) GetUntil() string    { return l.Duration.String() }
func (l lockView) String() string {
	return "{TokenMeta: " + l.TokenMeta + " Duration: " + l.Duration.String() + "}"
}

func (c *Client) View(ctx context.Context, key string) (dblock.LockView, error) {
	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return &lockView{}, nil
		}
		return nil, err
	}

	duration, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	return &lockView{TokenMeta: result, Duration: duration}, nil
}

// Obtain tries to obtain a new lock using a key with the given TTL.
// May return ErrNotObtained if not successful.
func (c *Client) Obtain(ctx context.Context, key string, ttl time.Duration, optionsFns ...dblock.OptionsFn) (dblock.Lock, error) {
	opt := &dblock.Options{}
	for _, f := range optionsFns {
		f(opt)
	}

	token := opt.Token

	// Create a random token
	if token == "" {
		var err error
		if token, err = dblock.RandomToken(); err != nil {
			return nil, err
		}
	}

	value := token + opt.Meta
	ttlVal := strconv.FormatInt(int64(ttl/time.Millisecond), 10)
	retry := opt.GetRetryStrategy()

	// make sure we don't retry forever
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, time.Now().Add(ttl))
		defer cancel()
	}

	var ticker *time.Ticker
	for {
		if ok, err := c.obtain(ctx, key, value, len(token), ttlVal); err != nil {
			return nil, err
		} else if ok {
			return &Lock{Client: c, Key: key, value: value, tokenLen: len(token)}, nil
		}

		backoff := retry.NextBackoff()
		if backoff < 1 {
			return nil, dblock.ErrNotObtained
		}

		if ticker == nil {
			ticker = time.NewTicker(backoff)
			defer ticker.Stop()
		} else {
			ticker.Reset(backoff)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// Lock represents an obtained, distributed lock.
type Lock struct {
	*Client
	Key      string
	value    string
	tokenLen int
}

// Token returns the token value set by the lock.
func (l *Lock) Token() string {
	return l.value[:l.tokenLen]
}

// Metadata returns the metadata of the lock.
func (l *Lock) Metadata() string {
	return l.value[l.tokenLen:]
}

// TTL returns the remaining time-to-live. Returns 0 if the lock has expired.
func (l *Lock) TTL(ctx context.Context) (time.Duration, error) {
	res, err := luaPTTL.Run(ctx, l.client, []string{l.Key}, l.value).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	if num := res.(int64); num > 0 {
		return time.Duration(num) * time.Millisecond, nil
	}
	return 0, nil
}

// Refresh extends the lock with a new TTL.
// May return ErrNotObtained if refresh is unsuccessful.
func (l *Lock) Refresh(ctx context.Context, ttl time.Duration) error {
	ttlVal := strconv.FormatInt(int64(ttl/time.Millisecond), 10)
	status, err := luaRefresh.Run(ctx, l.client, []string{l.Key}, l.value, ttlVal).Result()
	if err != nil {
		return err
	}
	if status == int64(1) {
		return nil
	}
	return dblock.ErrNotObtained
}

// Release manually releases the lock.
// May return ErrLockNotHeld.
func (l *Lock) Release(ctx context.Context) error {
	res, err := luaRelease.Run(ctx, l.client, []string{l.Key}, l.value).Result()
	if errors.Is(err, redis.Nil) {
		return dblock.ErrLockNotHeld
	}
	if err != nil {
		return err
	}

	if i, ok := res.(int64); !ok || i != 1 {
		return dblock.ErrLockNotHeld
	}
	return nil
}

func (c *Client) obtain(ctx context.Context, key, value string, tokenLen int, ttlVal string) (bool, error) {
	_, err := luaObtain.Run(ctx, c.client, []string{key}, value, tokenLen, ttlVal).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}
