package mydb_test

import (
	"database/sql"
	"log"
	"testing"

	_ "github.com/wenj91/taos-driver"
	mydb "github.com/wenj91/taos-driver"
)

func TestDb(t *testing.T) {
	db, err := sql.Open("taosSql", "root:taosdata@http(localhost:6041)/test")
	if err != nil {
		t.Errorf("some error %s", err.Error())
	}
	rows, err := db.Query("select * from tb1")
	if err != nil {
		log.Fatal("some wrong for query", err.Error())
	}
	for rows.Next() {
		var user mydb.MyUser
		if err := rows.Scan(&user.Name, &user.Age); err != nil {
			log.Println("scan value erro", err.Error())
		} else {
			log.Println(user)
		}
	}

	_, err = db.Exec(`insert into tb1 values ("2021-09-04 21:03:38.745", 2)`)
	log.Println(err)
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
