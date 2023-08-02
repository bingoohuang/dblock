# dblock

distributed lock based on rdbms, redis, etc.

```go
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bingoohuang/dblock"
	"github.com/bingoohuang/dblock/rdblock"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// db based
	db, err := sql.Open("mysql",
		"root:root@(127.0.0.1:3306)/mdb?charset=utf8mb4&parseTime=true&loc=Local")
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()
	locker := rdblock.New(db)

	// // or redis based
	// client := redis.NewClient(&redis.Options{
	// 	Network: "tcp",
	// 	Addr:    "127.0.0.1:6379",
	// })
	// defer client.Close()
	// locker = redislock.New(client)

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
```

## resources

- [PostgreSQL Lock Client for Go](https://github.com/cirello-io/pglock)
- [bsm/redislock](github.com/bsm/redislock)
- [lukas-krecan/ShedLock](https://github.com/lukas-krecan/ShedLock) ShedLock
  确保您的计划任务最多同时执行一次。如果一个任务正在一个节点上执行，它会获取一个锁，以防止从另一个节点（或线程）执行同一任务。请注意，如果一个任务已经在一个节点上执行，则其他节点上的执行不会等待，而是会被跳过。
  ShedLock 使用 Mongo、JDBC 数据库、Redis、Hazelcast、ZooKeeper 等外部存储进行协调。
- [Lockgate](https://github.com/werf/lockgate) is a cross-platform distributed locking library for Go. Supports distributed locks backed by Kubernetes or
  HTTP lock server. Supports conventional OS file locks.