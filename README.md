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

## Apache IoTDB Table REST driver

该仓库另注册一个独立的 `database/sql` driver：`iotdb`。原有 `taosSql` 连接器和 REST 路径保持不变。IoTDB 实现面向 IoTDB 2.x Table 模型 REST API v1，服务端需启用 REST 服务；认证使用 HTTP Basic Auth，生产环境应使用 HTTPS 并保持证书校验。

IoTDB Table 查询、非查询请求及返回结构遵循 [Apache IoTDB RESTAPI V1 文档](https://iotdb.apache.org/UserGuide/latest-Table/API/RestAPI-V1_apache.html)：`QueryContext` 使用 `/rest/table/v1/query`，`ExecContext` 使用 `/rest/table/v1/nonQuery`。REST v1 不提供 SQL server-side prepared statement、事务或可靠的受影响行数/自增 ID，因此问号参数会在客户端转为 SQL 字面量；`Begin`、`RowsAffected` 和 `LastInsertId` 会返回明确的不支持错误。

### 配置与使用

通过环境变量提供 DSN，避免把账号密码写进源码：

```sh
: "${IOTDB_DSN:?请通过安全配置设置连接串}"
```

DSN 也支持连接器的网络语法，例如 `username:password@http(host:18080)/telemetry` 和 `username:password@https(host:18443)/telemetry`。特殊账号字符应按 URL 查询转义规则编码。Table 查询的 database 来自 DSN 路径；每个请求都会显式传入该值，因此不要依赖 `USE` 在连接池内切换 database。创建 database 时可使用空 database 路径并执行 `CREATE DATABASE ...`。

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/wenj91/taos-driver"
)

func main() {
	db, err := sql.Open("iotdb", os.Getenv("IOTDB_DSN"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	rows, err := db.QueryContext(ctx,
		"SELECT time, device_id, temperature FROM readings WHERE device_id = ? LIMIT 100",
		"sensor-01",
	)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var timestamp time.Time
		var deviceID string
		var temperature sql.NullFloat64
		if err := rows.Scan(&timestamp, &deviceID, &temperature); err != nil {
			log.Fatal(err)
		}
		fmt.Println(timestamp, deviceID, temperature)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}
```

`TIMESTAMP` 查询值按 IoTDB Table REST 的毫秒 epoch 数值映射到 `time.Time`；若查询投影将其转换成 `INT64`，则映射成 `int64`。其余常见类型映射为 `BOOLEAN` → `bool`、`INT32/INT64` → `int64`、`FLOAT/DOUBLE` → `float64`、`STRING/DATE` → `string`、`BLOB` → `[]byte`，JSON `null` 保留为 SQL NULL。

### 支持范围

- 支持 `database/sql` 的连接池、`PingContext`、context-aware Query/Exec、Prepare、问号占位符字面量转换、查询列元数据、REST 错误码映射和可关闭连接。
- 仅支持 IoTDB 2.x Table REST v1。该 driver 不实现 Tree 模型、IoTDB 1.x REST 协议或 RPC/Session 客户端。
- `PingContext` 检查未认证的 `/ping` 服务健康状态，不验证账号密码；认证会在第一次查询或执行时验证。
- 默认 `row_limit=10000`、`timeout=15s`，可通过 DSN 的 `row_limit` 和 `timeout` 配置；单次响应限制为 64 MiB。
- 本地验证：`go test -run '^Test(IoTDB|ParseIoTDB)' .`。
- 可选 IoTDB 只读远端验收使用 `IOTDB_TEST_DSN`，要求 Table REST 服务可访问且账号有权执行 `SHOW DATABASES`：`go test -run '^TestIoTDBRemoteReadOnlySmoke$' -v .`。通过 SSH 隧道时，DSN 主机与端口应指向本地转发端点；此用例不写入或清理服务端数据。
- 可选远端写入/读取验收使用单独的 `IOTDB_TEST_WRITE_DSN`，需要创建和删除 database 的权限：`go test -run '^TestIoTDBRemoteWriteRead$' -v .`。该用例会创建唯一的临时 database，并在测试结束时删除。

更多 API 细节见 [Apache IoTDB Table RESTAPI V1](https://iotdb.apache.org/UserGuide/latest-Table/API/RestAPI-V1_apache.html)。
