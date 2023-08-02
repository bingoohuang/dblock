package rdblock_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/bingoohuang/dblock/rdblock"
	"log"
	"time"

	"github.com/bingoohuang/dblock"
	_ "github.com/go-sql-driver/mysql"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
}

func openDB() *sql.DB {
	// Connect to mysql.
	db, err := sql.Open("mysql",
		"root:root@(127.0.0.1:3306)/mdb?charset=utf8mb4&parseTime=true&loc=Local")
	if err != nil {
		log.Panic(err)
	}

	// See "Important settings" section.
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	return db
}

func Example() {
	// Connect to mysql.
	db, err := sql.Open("mysql",
		"root:root@(127.0.0.1:3306)/mdb?charset=utf8mb4&parseTime=true&loc=Local")
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()

	// Create a new lock client.
	locker := rdblock.New(db)

	ctx := context.Background()

	// Try to obtain lock.
	lock, err := locker.Obtain(ctx, "my-key", 100*time.Millisecond)
	if errors.Is(err, dblock.ErrNotObtained) {
		fmt.Println("Could not obtain lock!")
	} else if err != nil {
		log.Panicln(err)
	}

	// Don't forget to defer Release.
	defer lock.Release(ctx)
	fmt.Println("I have a lock!")

	// Sleep and check the remaining TTL.
	time.Sleep(50 * time.Millisecond)
	if ttl, err := lock.TTL(ctx); err != nil {
		log.Panicln(err)
	} else if ttl > 0 {
		fmt.Println("Yay, I still have my lock!")
	}

	// extend my lock.
	if err := lock.Refresh(ctx, 100*time.Millisecond); err != nil {
		log.Panicln(err)
	}

	// Sleep a little longer, then check.
	time.Sleep(100 * time.Millisecond)
	if ttl, err := lock.TTL(ctx); err != nil {
		log.Panicln(err)
	} else if ttl == 0 {
		fmt.Println("Now, my lock has expired!")
	}

	// Output:
	// I have a lock!
	// Yay, I still have my lock!
	// Now, my lock has expired!
}

func ExampleClient_Obtain_retry() {
	db := openDB()
	defer db.Close()

	// Create a new lock client.
	locker := rdblock.New(db)

	ctx := context.Background()

	// Retry every 100ms, for up-to 3x
	backoff := dblock.LimitRetry(dblock.LinearBackoff(100*time.Millisecond), 3)

	// Obtain lock with retry
	lock, err := locker.Obtain(ctx, "my-key", time.Second, dblock.WithRetryStrategy(backoff))
	if errors.Is(err, dblock.ErrNotObtained) {
		fmt.Println("Could not obtain lock!")
	} else if err != nil {
		log.Panicln(err)
	}
	defer lock.Release(ctx)

	fmt.Println("I have a lock!")

	// Output: I have a lock!
}

func ExampleClient_Obtain_customDeadline() {
	db := openDB()
	defer db.Close()

	// Create a new lock client.
	locker := rdblock.New(db)

	// Retry every 500ms, for up-to a minute
	backoff := dblock.LinearBackoff(500 * time.Millisecond)
	ctx := context.Background()
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Minute))
	defer cancel()

	// Obtain lock with retry + custom deadline
	lock, err := locker.Obtain(ctx, "my-key", time.Second, dblock.WithRetryStrategy(backoff))
	if errors.Is(err, dblock.ErrNotObtained) {
		fmt.Println("Could not obtain lock!")
	} else if err != nil {
		log.Panicln(err)
	}
	defer lock.Release(ctx)

	fmt.Println("I have a lock!")
	// Output: I have a lock!
}
