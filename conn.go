package taos

import (
	"database/sql/driver"
	"errors"
)

// conn for db open
type conn struct {
	token string
	drv   *Driver
}

// Prepare statement for prepare exec
func (c *conn) Prepare(query string) (driver.Stmt, error) {
	// log.Println("prepare:", query)

	paramCount := paramsCount(query)
	return &taosStmt{
		sqlStr:     query,
		conn:       c,
		paramCount: paramCount,
	}, nil
}

// Close close db connection
func (c *conn) Close() error {
	return errors.New("can't close connection")
}

// Begin begin
func (c *conn) Begin() (driver.Tx, error) {
	return nil, errors.New("not support tx")
}
