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
	flag.Parse()

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

	lock, err := locker.Obtain(ctx, *pKey, *pTTL, dblock.WithToken(*pToken), dblock.WithMeta(*pMeta))
	if err != nil {
		log.Printf("obtained failed: %v", err)
		return
	}
	log.Printf("obtained, token: %s, meta: %s", lock.Token(), lock.Metadata())
	ttl, err := lock.TTL(ctx)
	if err != nil {
		log.Printf("obtained failed: %v", err)
		return
	}
	log.Printf("ttl %s", ttl)

	switch {
	case *pRelease:
		if err := lock.Release(ctx); err != nil {
			log.Printf("release failed: %v", err)
		} else {
			log.Printf("release successfully")
		}
	case *pRefresh:
		if err := lock.Refresh(ctx, *pTTL); err != nil {
			log.Printf("refresh failed: %v", err)
		} else {
			log.Printf("refresh successfully")
		}
	}
}

func ParseInt(s string) int {
	value, _ := strconv.Atoi(s)
	return value
}
