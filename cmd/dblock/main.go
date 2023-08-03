package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/bingoohuang/dblock"
	"github.com/bingoohuang/dblock/pkg/envflag"
	"github.com/bingoohuang/dblock/pkg/helper"
	"github.com/bingoohuang/dblock/rdblock"
	_ "github.com/go-sql-driver/mysql"
)

var (
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
	pURI     = flag.String("uri", "redis://localhost:6379/0", `uri, e.g. mysql://root:root@localhost:3306/mysql`)
	pKey     = flag.String("key", "testkey", "lock key")
	pToken   = flag.String("token", "", "token value")
	pTTL     = flag.Duration("ttl", time.Hour, "ttl")
	pMeta    = flag.String("meta", "", "meta value")
	pRelease = flag.Bool("release", false, "release lock")
	pRefresh = flag.Bool("refresh", false, "refresh lock")
	pView    = flag.Bool("view", false, "view lock")
	pDebug   = flag.Bool("debug", false, "debugging mode")
)

func main() {
	_ = envflag.Parse()

	if *pKey == "" || *pURI == "" {
		flag.Usage()
		os.Exit(1)
	}

	rdblock.Debug = *pDebug
	locker, err := helper.Create(*pURI)
	if err != nil {
		log.Fatalf("create lock: %v", err)
	}
	defer locker.Close()

	ctx := context.Background()

	switch {
	case *pRelease:
		lock, err := getLock(ctx, locker, *pKey, *pToken, *pMeta, *pTTL)
		if err != nil {
			return
		}
		if err := lock.Release(ctx); err != nil {
			log.Printf("release failed: %v", err)
		} else {
			log.Printf("release successfully")
		}
	case *pRefresh:
		lock, err := getLock(ctx, locker, *pKey, *pToken, *pMeta, *pTTL)
		if err != nil {
			return
		}
		if err := lock.Refresh(ctx, *pTTL); err != nil {
			log.Printf("refresh failed: %v", err)
		} else {
			log.Printf("refresh successfully")
		}
	case *pView:
		if lockView, err := locker.View(ctx, *pKey); err != nil {
			log.Printf("view failed: %v", err)
		} else {
			log.Printf("view: %s", lockView)
		}
	default:
		if _, err := getLock(ctx, locker, *pKey, *pToken, *pMeta, *pTTL); err != nil {
			return
		}
	}
}

func getLock(ctx context.Context, locker dblock.Client, key, token, meta string, ttl time.Duration) (dblock.Lock, error) {
	lock, err := locker.Obtain(ctx, key, ttl, dblock.WithToken(token), dblock.WithMeta(meta))
	if err != nil {
		log.Printf("obtained failed: %v", err)
		return nil, err
	}
	log.Printf("obtained, token: %s, meta: %s", lock.Token(), lock.Metadata())
	lockTTL, err := lock.TTL(ctx)
	if err != nil {
		log.Printf("obtained failed: %v", err)
		return nil, err
	}
	log.Printf("ttl %s", lockTTL)
	return lock, nil
}
