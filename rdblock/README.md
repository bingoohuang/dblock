# rdb

关系型 db 模拟 shedlock-spring 实现作业调度。

```sql
CREATE TABLE shedlock
(
    name      VARCHAR(1024) primary key,
    expire_at TIMESTAMP,
    locked_at TIMESTAMP,
    locked_by VARCHAR(1024),
    hostname  VARCHAR(1024),
    pid       INT,
    process   VARCHAR(1024)
)
```
