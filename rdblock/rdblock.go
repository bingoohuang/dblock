package rdblock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bingoohuang/dblock"
)

type DB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type logDb struct {
	db DB
}

func (d *logDb) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	log.Printf("query: %q", query)
	return d.db.QueryRowContext(ctx, query, args...)
}

func (d *logDb) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	log.Printf("query: %q", query)
	result, err := d.db.ExecContext(ctx, query, args...)
	if result != nil {
		if affected, err := result.RowsAffected(); err == nil {
			log.Printf("affected: %d", affected)
		}
	}

	return result, err
}

var Debug bool

// Client wraps a redis client.
type Client struct {
	client             DB
	Table              string
	NotAutoCreateTable bool

	autoCreateTableChecked bool
}

// New creates a new Client instance with a custom namespace.
func New(client DB) *Client {
	c := &Client{client: client}
	if Debug {
		c.client = &logDb{db: client}
	}
	return c
}

// Obtain tries to obtain a new lock using a key with the given TTL.
// May return ErrNotObtained if not successful.
func (c *Client) Obtain(ctx context.Context, key string, ttl time.Duration, optionsFns ...dblock.OptionsFn) (dblock.Lock, error) {
	opt := &dblock.Options{}
	for _, f := range optionsFns {
		f(opt)
	}

	if c.Table == "" {
		c.Table = "t_shedlock"
	}

	if !c.NotAutoCreateTable && !c.autoCreateTableChecked {
		if _, err := c.client.ExecContext(ctx, `CREATE TABLE `+c.Table+`(lock_name VARCHAR(64) NOT NULL PRIMARY KEY, `+
			`lock_until VARCHAR(64) NOT NULL, locked_at VARCHAR(64) NOT NULL, locked_by VARCHAR(1024) NOT NULL, `+
			`token_value VARCHAR(64) NOT NULL, meta_value VARCHAR(1024)NOT NULL, locked_pid VARCHAR(64) NOT NULL)`); err != nil {
			if Debug {
				log.Printf("auto creaet table failed: %v", err)
			}
		}
		c.autoCreateTableChecked = true
	}

	token := opt.Token

	// Create a random token
	if token == "" {
		var err error
		if token, err = dblock.RandomToken(); err != nil {
			return nil, err
		}
	}

	retry := opt.GetRetryStrategy()
	lockUntil := time.Now().Add(ttl)

	// make sure we don't retry forever
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, lockUntil)
		defer cancel()
	}

	var ticker *time.Ticker
	for {
		lockUntilStr := lockUntil.Format(time.RFC3339Nano)
		if ok, err := c.obtain(ctx, key, token, opt.Meta, lockUntilStr); err != nil {
			return nil, err
		} else if ok {
			return &Lock{
				Client:   c,
				Key:      key,
				token:    token,
				metadata: opt.Meta,
				Until:    lockUntilStr,
			}, nil
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
	token    string
	metadata string
	Until    string
}

// Token returns the token value set by the lock.
func (l *Lock) Token() string { return l.token }

// Metadata returns the metadata of the lock.
func (l *Lock) Metadata() string { return l.metadata }

// TTL returns the remaining time-to-live. Returns 0 if the lock has expired.
func (l *Lock) TTL(ctx context.Context) (time.Duration, error) {
	sh := &shedLock{
		Table: l.Table,
		Name:  l.Key,
		Token: l.token,
	}
	found, err := sh.query(ctx, l.client)
	if err != nil {
		return 0, err
	}

	if !found {
		return 0, nil
	}

	if sh.Meta == NonValue {
		sh.Meta = ""
	}

	if Debug {
		log.Printf("found: %+v", sh)
	}

	lockUntil, err := time.Parse(time.RFC3339Nano, sh.Until)
	if err != nil {
		return 0, fmt.Errorf("parse lockUnitl %s: %w", sh.Until, err)
	}

	l.Until = sh.Until

	now := time.Now()
	if Debug {
		log.Printf("now: %s", now.Format(time.RFC3339Nano))
	}
	ttl := lockUntil.Sub(now)
	if Debug {
		log.Printf("ttl: %s", ttl)
	}
	if ttl > 0 {
		return ttl, nil
	}

	return 0, nil
}

// Refresh extends the lock with a new TTL.
// May return ErrNotObtained if refresh is unsuccessful.
func (l *Lock) Refresh(ctx context.Context, ttl time.Duration) error {
	sh := &shedLock{
		Table: l.Table,
		Name:  l.Key,
		Token: l.token,
		Until: time.Now().Add(ttl).Format(time.RFC3339Nano),
	}
	status, err := sh.extend(ctx, l.client)
	if err != nil {
		return err
	}
	if status {
		return nil
	}
	return dblock.ErrNotObtained
}

// Release manually releases the lock.
// May return ErrLockNotHeld.
func (l *Lock) Release(ctx context.Context) error {
	sh := &shedLock{
		Table: l.Table,
		Name:  l.Key,
		Token: l.token,
	}
	res, err := sh.unlock(ctx, l.client)
	if err != nil {
		return err
	}
	if !res {
		return dblock.ErrLockNotHeld
	}

	return nil
}

func (c *Client) obtain(ctx context.Context, key, token, meta, lockUntil string) (bool, error) {
	sh := shedLock{
		Table: c.Table,
		Name:  key,
		Token: token,
		Meta:  meta,
		Until: lockUntil,
	}
	if sh.insert(ctx, c.client) {
		return true, nil
	}

	return sh.update(ctx, c.client)
}

type shedLock struct {
	Table string
	Name  string
	At    string
	Until string
	By    string
	Token string
	Meta  string
	Pid   string
}

func (l *shedLock) query(ctx context.Context, db DB) (bool, error) {
	s := `select lock_until, locked_at, locked_by, token_value, meta_value, locked_pid from {Table} ` +
		`WHERE lock_name = {Name} AND token_value = {Token}`
	s = strings.ReplaceAll(s, "{Table}", l.Table)
	s = strings.ReplaceAll(s, "{Name}", singleQuote(l.Name))
	s = strings.ReplaceAll(s, "{Token}", singleQuote(l.Token))

	row := db.QueryRowContext(ctx, s)
	if err := row.Scan(&l.Until, &l.At, &l.By, &l.Token, &l.Meta, &l.Pid); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("query: %w", err)
	}

	return true, nil
}

func (l *shedLock) insert(ctx context.Context, db DB) bool {
	s := `INSERT INTO {Table} (lock_name, lock_until, locked_at, locked_by, token_value, meta_value, locked_pid) ` +
		`VALUES ({Name}, {Until}, {At}, {By}, {Token}, {Meta}, {LockedPid})`
	s = strings.ReplaceAll(s, "{Table}", l.Table)
	s = strings.ReplaceAll(s, "{Name}", singleQuote(l.Name))
	s = strings.ReplaceAll(s, "{Until}", singleQuote(l.Until))
	s = strings.ReplaceAll(s, "{At}", singleQuote(time.Now().Format(time.RFC3339Nano)))
	s = strings.ReplaceAll(s, "{By}", singleQuote(Hostname))
	s = strings.ReplaceAll(s, "{Token}", singleQuote(l.Token))
	s = strings.ReplaceAll(s, "{Meta}", singleQuote(l.Meta))
	s = strings.ReplaceAll(s, "{LockedPid}", singleQuote(Pid))

	if _, err := db.ExecContext(ctx, s); err == nil {
		return true
	}

	return false
}

func (l *shedLock) update(ctx context.Context, db DB) (bool, error) {
	s := `UPDATE {Table} SET lock_until = {Until}, ` +
		`locked_at = {At}, locked_by = {By}, ` +
		`token_value = {Token}, meta_value = {Meta}, locked_pid = {LockedPid} ` +
		`WHERE lock_name = {Name} AND (token_value = {Token} or lock_until <= {Now} )`
	s = strings.ReplaceAll(s, "{Table}", l.Table)
	s = strings.ReplaceAll(s, "{Name}", singleQuote(l.Name))
	s = strings.ReplaceAll(s, "{Until}", singleQuote(l.Until))
	now := singleQuote(time.Now().Format(time.RFC3339Nano))
	s = strings.ReplaceAll(s, "{Now}", now)
	s = strings.ReplaceAll(s, "{At}", now)
	s = strings.ReplaceAll(s, "{By}", singleQuote(Hostname))
	s = strings.ReplaceAll(s, "{Token}", singleQuote(l.Token))
	s = strings.ReplaceAll(s, "{Meta}", singleQuote(l.Meta))
	s = strings.ReplaceAll(s, "{LockedPid}", singleQuote(Pid))

	result, err := db.ExecContext(ctx, s)
	if err != nil {
		return false, fmt.Errorf("update lock %q : %w", s, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("RowsAffected: %w", err)
	}

	return rowsAffected > 0, nil
}

func (l *shedLock) extend(ctx context.Context, db DB) (bool, error) {
	s := `UPDATE {Table} SET lock_until = {Until} ` +
		`WHERE lock_name = {Name} AND token_value = {Token}`
	s = strings.ReplaceAll(s, "{Table}", l.Table)
	s = strings.ReplaceAll(s, "{Name}", singleQuote(l.Name))
	s = strings.ReplaceAll(s, "{Until}", singleQuote(l.Until))
	s = strings.ReplaceAll(s, "{Token}", singleQuote(l.Token))
	result, err := db.ExecContext(ctx, s)
	if err != nil {
		return false, fmt.Errorf("update lock %q : %w", s, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("RowsAffected: %w", err)
	}

	return rowsAffected > 0, nil
}

func (l *shedLock) unlock(ctx context.Context, db DB) (bool, error) {
	l.Until = time.Now().Add(-time.Second).Format(time.RFC3339Nano)
	s := `UPDATE {Table} SET lock_until = {Until} ` +
		`WHERE lock_name = {Name} AND token_value = {Token}`
	s = strings.ReplaceAll(s, "{Table}", l.Table)
	s = strings.ReplaceAll(s, "{Name}", singleQuote(l.Name))
	s = strings.ReplaceAll(s, "{Until}", singleQuote(l.Until))
	s = strings.ReplaceAll(s, "{Token}", singleQuote(l.Token))
	result, err := db.ExecContext(ctx, s)
	if err != nil {
		return false, fmt.Errorf("update lock %q : %w", s, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("RowsAffected: %w", err)
	}

	return rowsAffected > 0, nil
}

const (
	quote  = '\''
	escape = '\\'
)

const NonValue = "(nil)"

// singleQuote returns a single-quoted Go string literal representing s. But, nothing else escapes.
func singleQuote(s string) string {
	if s == "" {
		s = NonValue
	}
	out := []rune{quote}
	for _, r := range s {
		switch r {
		case quote:
			out = append(out, escape, r)
		default:
			out = append(out, r)
		}
	}
	out = append(out, quote)
	return string(out)
}

var Hostname = func() string {
	hostname, err := os.Hostname()
	if err != nil {
		return err.Error()
	}

	return hostname
}()

var Pid = func() string {
	pid := os.Getpid()
	return strconv.Itoa(pid)
}()
