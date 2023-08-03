package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/bingoohuang/dblock"
	"github.com/bingoohuang/dblock/pkg/envflag"
	"github.com/bingoohuang/dblock/rdblock"
	"github.com/bingoohuang/dblock/redislock"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/xo/dburl"
)

func main() {
	pDebug := flag.Bool("debug", false, "debugging mode")
	pToken := flag.String("token", "", "token value")
	pMeta := flag.String("meta", "", "meta value")
	pTTL := flag.Duration("ttl", time.Hour, "ttl")
	pKey := flag.String("key", "testkey", "lock key （required）")
	pRelease := flag.Bool("release", false, "release lock")
	pRefresh := flag.Bool("refresh", false, "refresh lock")
	pView := flag.Bool("view", false, "view lock")
	// redis url 格式 https://cloud.tencent.com/developer/article/1451666
	// redis://[:password@]host[:port][/database][?[timeout=timeout[d|h|m|s|ms|us|ns]][&database=database]]
	// postgres://user:pass@localhost/dbname
	// pg://user:pass@localhost/dbname?sslmode=disable
	// mysql://user:pass@localhost/dbname
	// mysql:/var/run/mysqld/mysqld.sock
	// sqlserver://user:pass@remote-host.com/dbname
	// mssql://user:pass@remote-host.com/instance/dbname
	// ms://user:pass@remote-host.com:port/instance/dbname?keepAlive=10
	// oracle://user:pass@somehost.com/sid
	// sap://user:pass@localhost/dbname
	// sqlite:/path/to/file.db
	// file:myfile.sqlite3?loc=auto
	// odbc+postgres://user:pass@localhost:port/dbname?option1=
	pURI := flag.String("uri", "redis://localhost:6379", "uri, e.g."+
		`
redis://localhost:6379
mysql://root:root@localhost:3306/mysql
`)
	_ = envflag.Parse()

	if *pKey == "" || *pURI == "" {
		flag.Usage()
		os.Exit(1)
	}

	// parse url
	v, err := url.Parse(*pURI)
	if err != nil {
		log.Fatalf("parse url: %v", err)
	}
	rdblock.Debug = *pDebug

	var locker dblock.Client
	if v.Scheme == "redis" {
		// Connect to redis.
		opt := &redis.Options{
			Network: "tcp",
			Addr:    v.Host,
			DB:      ParseInt(v.Query().Get("database")),
		}
		if v.User != nil {
			if password, ok := v.User.Password(); ok {
				opt.Password = password
			}
		}
		client := redis.NewClient(opt)
		defer client.Close()

		locker = redislock.New(client)
	} else {
		db, err := dburl.Open(*pURI)
		if err != nil {
			log.Printf("parse url: %v", err)
			return
		}
		locker = rdblock.New(db)
		defer db.Close()
	}

	ctx := context.Background()

	switch {
	case *pRelease:
		lock, err := getLock(err, locker, ctx, pKey, pTTL, pToken, pMeta)
		if err != nil {
			return
		}
		if err := lock.Release(ctx); err != nil {
			log.Printf("release failed: %v", err)
		} else {
			log.Printf("release successfully")
		}
	case *pRefresh:
		lock, err := getLock(err, locker, ctx, pKey, pTTL, pToken, pMeta)
		if err != nil {
			return
		}
		if err := lock.Refresh(ctx, *pTTL); err != nil {
			log.Printf("refresh failed: %v", err)
		} else {
			log.Printf("refresh successfully")
		}
	case *pView:
		if view, ok := locker.(dblock.ClientView); ok {
			if lockView, err := view.View(ctx, *pKey); err != nil {
				log.Printf("view failed: %v", err)
			} else {
				log.Printf("view: %s", lockView)
			}
		}
	default:
		if _, err := getLock(err, locker, ctx, pKey, pTTL, pToken, pMeta); err != nil {
			return
		}
	}
}

func getLock(err error, locker dblock.Client, ctx context.Context, pKey *string, pTTL *time.Duration, pToken *string, pMeta *string) (dblock.Lock, error) {
	lock, err := locker.Obtain(ctx, *pKey, *pTTL, dblock.WithToken(*pToken), dblock.WithMeta(*pMeta))
	if err != nil {
		log.Printf("obtained failed: %v", err)
		return nil, err
	}
	log.Printf("obtained, token: %s, meta: %s", lock.Token(), lock.Metadata())
	ttl, err := lock.TTL(ctx)
	if err != nil {
		log.Printf("obtained failed: %v", err)
		return nil, err
	}
	log.Printf("ttl %s", ttl)
	return lock, nil
}

func ParseInt(s string) int {
	value, _ := strconv.Atoi(s)
	return value
}
