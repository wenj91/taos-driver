# Go Connector for TDengine

[![Build Status](https://cloud.drone.io/api/badges/taosdata/driver-go/status.svg)](https://cloud.drone.io/taosdata/driver-go)


为TDengine提供了GO驱动程序 `taosSql` [taos-driver]，实现了GO语言的内置数据库操作接口 `database/sql/driver`。

## 提示

`github.com/wenj91/taos-driver` 是基于TDengine的RESTFul api重构的驱动版本，纯golang实现。

## 安装

对新建项目，建议使用Go 1.14+，并使用 GO Modules 方式进行管理。

```sh
go mod init taos-demo
```

引入taosSql：

```go
import (
    "database/sql"
    _ "github.com/wenj91/taos-driver"
)
```

使用`go mod`方式管理依赖包：

```sh
go mod tidy
```

或通过`go get`直接下载安装：

```sh
go get github.com/wenj91/taos-driver
```

## 用法

### `database/sql` 标准接口

TDengine Go 连接器提供 database/sql 标准接口，使用方法简单示例如下：

```go
package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/wenj91/taos-driver"
)

func main() {
	var taosuri = "root:taosdata@/http(localhost:6041)/test"
	taos, err := sql.Open("taosSql", taosuri)
	if err != nil {
		fmt.Println("failed to connect TDengine, err:", err)
		return
	}
	defer taos.Close()
	taos.Exec("create database if not exists test")
	taos.Exec("use test")
	taos.Exec("create table if not exists tb1 (ts timestamp, a int)")
	_, err = taos.Exec("insert into tb1 values(now, 0)(now+1s,1)(now+2s,2)(now+3s,3)")
	if err != nil {
		fmt.Println("failed to insert, err:", err)
		return
	}
	rows, err := taos.Query("select * from tb1")
	if err != nil {
		fmt.Println("failed to select from table, err:", err)
		return
	}

	defer rows.Close()
	for rows.Next() {
		var r struct {
			ts time.Time
			a  int
		}
		err := rows.Scan(&r.ts, &r.a)
		if err != nil {
			fmt.Println("scan error:\n", err)
			return
		}
		fmt.Println(r.ts, r.a)
	}
}
```

常用API列表：

- `sql.Open(DRIVER_NAME string, dataSourceName string) *DB`

  该API用来创建`database/sql` DB对象，类型为`*DB`，DRIVER_NAME设置为字符串`taosSql`,
  dataSourceName设置为字符串`user:password@/tcp(host:port)/dbname`，对应于TDengine的高可用机制，可以使用 `user:password@/cfg/dbname`
  来使用`/etc/taos/taos.cfg`中的多EP配置。

  **注意**： 该API成功创建的时候，并没有做权限等检查，只有在真正执行Query或者Exec的时候才能真正的去创建连接，并同时检查user/password/host/port是不是合法。

- `func (db *DB) Exec(query string, args ...interface{}) (Result, error)`

  sql.Open内置的方法，用来执行非查询相关SQL，如`create`, `alter`等。

- `func (db *DB) Query(query string, args ...interface{}) (*Rows, error)`

  sql.Open内置的方法，用来执行查询语句。

- `func (db *DB) Close() error`

  sql.Open内置的方法，关闭DB对象。

## 导航

taos-driver: [https://github.com/wenj91/taos-driver](https://github.com/wenj91/taos-driver)

driver-go: [https://github.com/taosdata/driver-go](https://github.com/taosdata/driver-go)

TDengine: [https://github.com/taosdata/TDengine](https://github.com/taosdata/TDengine)
