package taos_test

import (
	"database/sql"
	"log"
	"testing"

	_ "github.com/wenj91/taos-driver"
)

// MyTb tb1 demo model
type MyTb struct {
	Time string
	A    int
}

func TestDb(t *testing.T) {
	db, err := sql.Open("taosSql", "root:taosdata@http(localhost:6041)/test")
	if err != nil {
		t.Errorf("some error %s", err.Error())
	}

	db.Exec("create database if not exists test")
	db.Exec("use test")
	db.Exec("create table if not exists tb1 (ts timestamp, a int)")
	_, err = db.Exec("insert into tb1 values(now, 0)(now+1s,1)(now+2s,2)(now+3s,3)")
	if err != nil {
		log.Fatal("failed to insert, err:", err)
	}

	rows, err := db.Query("select * from tb1")
	if err != nil {
		log.Fatal("some wrong for query", err.Error())
	}
	for rows.Next() {
		var tb MyTb
		if err := rows.Scan(&tb.Time, &tb.A); err != nil {
			log.Println("scan value erro", err.Error())
		} else {
			log.Println(tb)
		}
	}

	_, err = db.Exec(`create table if not exists MD5_fa0f7ac06608830346a51c03de15eaf2 using http_client_requests_seconds_sum tags("339ac2bfa5fd9d673a96f112ea0e738d","false",?,"xx.com","xxxx","dev","127.0.0.1","xx-service","GET","APP","xxx","200","xxx","xxx","/uu/get?corpid={1}&corpsecret={2}")`, "\"xxx\"")
	if nil != err {
		log.Fatal(err)
	}

	_, err = db.Exec(`insert into tb1 values ("2021-09-04 21:03:38.745", 2)`)
	log.Println(err)

	rows, err = db.Query("select * from tb1 where a = ?", 1)
	if err != nil {
		log.Fatal("some wrong for query", err.Error())
	}
	for rows.Next() {
		var tb MyTb
		if err := rows.Scan(&tb.Time, &tb.A); err != nil {
			log.Println("scan value erro", err.Error())
		} else {
			log.Println(tb)
		}
	}
}

func Example() {
	db, err := sql.Open("mydb", "mydb://dalong@127.0.0.1/demoapp")
	if err != nil {
		log.Fatalf("some error %s", err.Error())
	}
	rows, err := db.Query("select * from demoapp")
	if err != nil {
		log.Println("some wrong for query", err.Error())
	}
	for rows.Next() {
		rows.Scan()
	}
}
