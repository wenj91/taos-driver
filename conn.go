package taos

import (
	"database/sql/driver"
	"errors"
	"log"
	"strings"
)

// Conn for db open
type Conn struct {
	token string
	drv   *Driver
}

// Prepare statement for prepare exec
func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	log.Println("prepare:", query)

	paramCount := strings.Count(query, "?")
	return &TaosStmt{
		sqlStr:     query,
		conn:       c,
		paramCount: paramCount,
	}, nil
}

// Close close db connection
func (c *Conn) Close() error {
	return errors.New("can't close connection")
}

// Begin begin
func (c *Conn) Begin() (driver.Tx, error) {
	return nil, errors.New("not support tx")
}
