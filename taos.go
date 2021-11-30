// copyright to rongfengliang

// this is a demo golang driver for test

package taos

import (
	"database/sql"
	"log"
)

func init() {
	log.Println("register taosSql driver")
	sql.Register("taosSql", &Driver{})
}
