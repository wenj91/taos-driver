package mydb

import (
	"database/sql/driver"
	"errors"
	jsoniter "github.com/json-iterator/go"
	"log"
	"strconv"
)

// Driver mydb driver for implement database/sql/driver
type Driver struct {
	cfg *config
}

func init() {
	log.Println("driver is call ")
}

// Open for implement driver interface
func (driver *Driver) Open(name string) (driver.Conn, error) {
	log.Println("exec open driver")

	cfg, err := parseDSN(name)
	if err != nil {
		return nil, err
	}

	driver.cfg = cfg

	token, err := driver.login()
	if err != nil {
		return nil, err
	}

	return &Conn{
		token: token,
	}, nil
}

func (driver *Driver) login() (string, error) {
	url := "http://" +
		driver.cfg.addr + ":" +
		strconv.Itoa(driver.cfg.port) +
		"/rest/login/" +
		driver.cfg.user + "/" +
		driver.cfg.passwd

	res, err := doGet(url)
	if nil != err {
		return "", err
	}

	any := jsoniter.Get(res)

	status := any.Get("status").ToString()
	if status != "succ" {
		return "", errors.New("[" + any.Get("code").ToString() +
			"]" + any.Get("desc").ToString())
	}

	return any.Get("desc").ToString(), nil
}

func (driver *Driver) useDB() (string, error) {
	url := "http://" +
		driver.cfg.addr + ":" +
		strconv.Itoa(driver.cfg.port) +
		"/rest/login/" +
		driver.cfg.user + "/" +
		driver.cfg.passwd

	res, err := doGet(url)
	if nil != err {
		return "", err
	}

	any := jsoniter.Get(res)

	status := any.Get("status").ToString()
	if status != "succ" {
		return "", errors.New("[" + any.Get("code").ToString() +
			"]" + any.Get("desc").ToString())
	}

	return any.Get("desc").ToString(), nil
}
