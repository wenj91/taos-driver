package mydb

import (
	"database/sql/driver"
	"errors"
	"log"
)

// Conn for db open
type Conn struct {
	Token string
}

// Prepare statement for prepare exec
func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	log.Println("prepare:", query)
	return &TaosStmt{
		SQL: query,
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
