// copyright to rongfengliang

// this is a demo golang driver for test

package mydb

import (
	"database/sql"
	"log"
)

func init() {
	log.Println("register taoSql driver")
	sql.Register("taoSql", &Driver{})
}
