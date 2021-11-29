package mydb

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

// TaosStmt for sql statement
type TaosStmt struct {
	SQL string
}

// Close  implement for stmt
func (stmt *TaosStmt) Close() error {
	return nil
}

// Query  implement for Query
func (stmt *TaosStmt) Query(args []driver.Value) (driver.Rows, error) {
	log.Println("do query", args)

	url := "http://127.0.0.1:6041/rest/sql"
	method := "GET"

	payload := strings.NewReader(stmt.SQL)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		log.Println(err)
		return nil, err
	}
	req.Header.Add("Authorization", "Basic cm9vdDp0YW9zZGF0YQ==")
	req.Header.Add("Content-Type", "text/plain")

	res, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	log.Println(string(body))

	status := jsoniter.Get(body, "status").ToString()
	if status != "succ" {
		return nil, errors.New(jsoniter.Get(body, "code").ToString() + ":" + jsoniter.Get(body, "desc").ToString())
	}

	cms := make([]*ColumnMeta, 0)
	iter := jsoniter.Get(body, "column_meta")
	for i := 0; i < iter.Size(); i++ {
		item := iter.Get(i)
		cm := &ColumnMeta{}

		any := jsoniter.Get([]byte(item.ToString()))
		cm.Name = any.Get(0).ToString()
		cm.Type = any.Get(1).ToInt()
		cm.Len = any.Get(2).ToInt()

		cms = append(cms, cm)
	}

	cols := make([]string, 0)
	ss := jsoniter.Get(body, "head").ToString()
	jsoniter.Unmarshal([]byte(ss), &cols)

	iterData := jsoniter.Get(body, "data")
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

	size := jsoniter.Get(body, "rows").ToInt64()
	myrows := TaosRows{
		Size:        size,
		Len:         size,
		Cols:        cols,
		ColumnMetas: cms,
		Data:        data,
	}

	return &myrows, nil
}

// NumInput row numbers
func (stmt *TaosStmt) NumInput() int {
	// don't know how many row numbers
	return -1
}

// Exec exec  implement
func (stmt *TaosStmt) Exec(args []driver.Value) (driver.Result, error) {
	url := "http://127.0.0.1:6041/rest/sql"
	method := "GET"

	payload := strings.NewReader(stmt.SQL)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	req.Header.Add("Authorization", "Basic cm9vdDp0YW9zZGF0YQ==")
	req.Header.Add("Content-Type", "text/plain")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Println(string(body))

	status := jsoniter.Get(body, "status").ToString()
	if status != "succ" {
		return nil, errors.New("[" + jsoniter.Get(body, "code").ToString() + "]:" + jsoniter.Get(body, "desc").ToString())
	}

	re := &TaosResult{
		RAf: jsoniter.Get(body, "data", 0).ToInt64(),
	}

	return re, nil
}
