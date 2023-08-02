# rdb

关系型 db 模拟 shedlock-spring 实现作业调度。

```sql
-- 建表语句
CREATE TABLE shedlock
(
    lock_name   VARCHAR(64)   NOT NULL PRIMARY KEY,
    lock_until  VARCHAR(64)   NOT NULL,
    locked_at   VARCHAR(64)   NOT NULL,
    locked_by   VARCHAR(1024) NOT NULL,
    token_value VARCHAR(64)   NOT NULL,
    meta_value  VARCHAR(1024),
    locked_pid  VARCHAR(64)   NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

时间格式：RFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"

## resouces

1. [hshe/go-shedlock](https://github.com/hshe/go-shedlock)
