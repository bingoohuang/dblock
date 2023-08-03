package helper

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/bingoohuang/dblock"
	"github.com/bingoohuang/dblock/rdblock"
	"github.com/bingoohuang/dblock/redislock"
	"github.com/redis/go-redis/v9"
	"github.com/xo/dburl"
)

func Create(uri string) (dblock.ClientCloser, error) {
	v, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	if v.Scheme == "redis" {
		db := strings.TrimPrefix(v.Path, "/")
		opt := &redis.Options{
			Network: "tcp",
			Addr:    v.Host,
			DB:      ParseInt(db),
		}
		if v.User != nil {
			if password, ok := v.User.Password(); ok {
				opt.Password = password
			}
		}
		redisClient := redis.NewClient(opt)
		if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
			return nil, err
		}
		return &redisClientCloser{
			Client:      redislock.New(redisClient),
			redisClient: redisClient,
		}, nil
	}

	db, err := dburl.Open(uri)
	if err != nil {
		return nil, fmt.Errorf("dburl open: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &dbClientCloser{
		DB:     db,
		Client: rdblock.New(db),
	}, nil
}

type redisClientCloser struct {
	dblock.Client
	redisClient *redis.Client
}

func (r redisClientCloser) Close() error {
	return r.redisClient.Close()
}

type dbClientCloser struct {
	dblock.Client
	DB *sql.DB
}

func (r dbClientCloser) Close() error {
	return r.DB.Close()
}

func ParseInt(s string) int {
	value, _ := strconv.Atoi(s)
	return value
}
