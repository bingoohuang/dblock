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

## cli

install `go install github.com/bingoohuang/dblock/...@latest`

db 锁

```sh
# 第一次以 key = abc 加锁，成功（默认有效期1小时）
$ dblock -uri mysql://root:root@localhost:3306/mysql -key abc
2023/08/02 23:15:05 obtained, token: ssddGH1_k1__PFD71dawTw, meta: 
2023/08/02 23:15:05 ttl 59m59.994631s

# 再次以 key = abc 加锁，失败 
$ dblock -uri mysql://root:root@localhost:3306/mysql -key abc
2023/08/02 23:15:07 obtained failed: dblock: not obtained

# 以 key = abc 以及 对应的 token 加锁，成功
$ dblock -uri mysql://root:root@localhost:3306/mysql -key abc -token ssddGH1_k1__PFD71dawTw
2023/08/02 23:15:22 obtained, token: ssddGH1_k1__PFD71dawTw, meta: 
2023/08/02 23:15:22 ttl 59m59.994631s

# 释放 key = abc 锁
$ dblock -uri mysql://root:root@localhost:3306/mysql -key abc -token ssddGH1_k1__PFD71dawTw -release
2023/08/02 23:15:26 obtained, token: ssddGH1_k1__PFD71dawTw, meta: 
2023/08/02 23:15:26 ttl 59m59.99436s
2023/08/02 23:15:26 release successfully

# 释放后再次获取 key = abc 锁，成功
$ dblock -uri mysql://root:root@localhost:3306/mysql -key abc                                       
2023/08/02 23:15:30 obtained, token: eF87Wg8N_FVLJvkV7RfEPw, meta: 
2023/08/02 23:15:30 ttl 59m59.994559s
```

redis 锁

```sh
# 首次加锁 ( key = abc ) 成功
$ dblock -uri redis://localhost:6379 -key abc
2023/08/02 23:20:13 obtained, token: dGjS1xUq1RCAbsGPBbP66w, meta: 
2023/08/02 23:20:13 ttl 59m59.998s

# 再次加锁 ( key = abc ) 失败
$ dblock -uri redis://localhost:6379 -key abc
2023/08/02 23:20:14 obtained failed: dblock: not obtained

# 以指定相同的 token 加锁，成功
$ dblock -uri redis://localhost:6379 -key abc -token dGjS1xUq1RCAbsGPBbP66w
2023/08/02 23:20:26 obtained, token: dGjS1xUq1RCAbsGPBbP66w, meta: 
2023/08/02 23:20:26 ttl 59m59.998s

# 释放锁
$ dblock -uri redis://localhost:6379 -key abc -token dGjS1xUq1RCAbsGPBbP66w -release
2023/08/02 23:20:31 obtained, token: dGjS1xUq1RCAbsGPBbP66w, meta: 
2023/08/02 23:20:31 ttl 59m59.999s
2023/08/02 23:20:31 release successfully

# 释放锁后，再次加锁，成功
$ dblock -uri redis://localhost:6379 -key abc                                       
2023/08/02 23:20:41 obtained, token: mX6u3WSLkObsZWYouk6PcA, meta: 
2023/08/02 23:20:41 ttl 59m59.999s
```

## resources

- [PostgreSQL Lock Client for Go](https://github.com/cirello-io/pglock)
- [bsm/redislock](github.com/bsm/redislock)
- [lukas-krecan/ShedLock](https://github.com/lukas-krecan/ShedLock) ShedLock
  确保您的计划任务最多同时执行一次。如果一个任务正在一个节点上执行，它会获取一个锁，以防止从另一个节点（或线程）执行同一任务。请注意，如果一个任务已经在一个节点上执行，则其他节点上的执行不会等待，而是会被跳过。
  ShedLock 使用 Mongo、JDBC 数据库、Redis、Hazelcast、ZooKeeper 等外部存储进行协调。
- [Lockgate](https://github.com/werf/lockgate) is a cross-platform distributed locking library for Go. Supports distributed locks backed by Kubernetes or
  HTTP lock server. Supports conventional OS file locks.