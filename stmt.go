package taos

import (
	"database/sql/driver"
	"errors"
	jsoniter "github.com/json-iterator/go"
	"log"
)

// TaosStmt for sql statement
type TaosStmt struct {
	sqlStr     string
	paramCount int
	conn       *Conn
}

// Close  implement for stmt
func (stmt *TaosStmt) Close() error {
	return nil
}

// Query  implement for Query
func (stmt *TaosStmt) Query(args []driver.Value) (driver.Rows, error) {
	log.Println("do query", args)

	querySql := stmt.sqlStr
	if len(args) != 0 {
		if !stmt.conn.drv.cfg.interpolateParams {
			return nil, driver.ErrSkip
		}

		// try client-side prepare to reduce roundtrip
		prepared, err := interpolateParams(stmt.sqlStr, args)
		if err != nil {
			return nil, err
		}
		querySql = prepared
	}

	query, err := stmt.conn.drv.query(querySql)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	any := jsoniter.Get(query)

	status := any.Get("status").ToString()
	if status != "succ" {
		return nil, errors.New(any.Get("code").ToString() + ":" + any.Get("desc").ToString())
	}

	cms := make([]*ColumnMeta, 0)
	iter := any.Get("column_meta")
	for i := 0; i < iter.Size(); i++ {
		item := iter.Get(i)
		cm := &ColumnMeta{}

		any2 := jsoniter.Get([]byte(item.ToString()))
		cm.Name = any2.Get(0).ToString()
		cm.Type = any2.Get(1).ToInt()
		cm.Len = any2.Get(2).ToInt()

		cms = append(cms, cm)
	}

	cols := make([]string, 0)
	ss := any.Get("head").ToString()
	err = jsoniter.Unmarshal([]byte(ss), &cols)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	iterData := any.Get("data")
	data := make([][]interface{}, 0)
	for i := 0; i < iterData.Size(); i++ {
		item := iterData.Get(i)

		any := jsoniter.Get([]byte(item.ToString()))

		dataItem := make([]interface{}, 0)
		for j := 0; j < any.Size(); j++ {
			dataItem = append(dataItem, any.Get(j).GetInterface())
		}

		data = append(data, dataItem)
	}

	size := any.Get("rows").ToInt64()
	taosRows := TaosRows{
		Size:        size,
		Len:         size,
		Cols:        cols,
		ColumnMetas: cms,
		Data:        data,
	}

	return &taosRows, nil
}

// NumInput row numbers
func (stmt *TaosStmt) NumInput() int {
	// don't know how many row numbers
	return stmt.paramCount
}

// Exec exec  implement
func (stmt *TaosStmt) Exec(args []driver.Value) (driver.Result, error) {

	querySql := stmt.sqlStr
	if len(args) != 0 {
		if !stmt.conn.drv.cfg.interpolateParams {
			return nil, driver.ErrSkip
		}

		// try client-side prepare to reduce roundtrip
		prepared, err := interpolateParams(stmt.sqlStr, args)
		if err != nil {
			return nil, err
		}
		querySql = prepared
	}

	query, err := stmt.conn.drv.query(querySql)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	any := jsoniter.Get(query)

	status := any.Get("status").ToString()
	if status != "succ" {
		return nil, errors.New("[" + any.Get("code").ToString() + "]:" + any.Get("desc").ToString())
	}

	re := &TaosResult{
		RAf: any.Get("data", 0).ToInt64(),
	}

	return re, nil
}
