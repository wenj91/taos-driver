package taos

import (
	"database/sql/driver"
	"errors"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Driver mydb driver for implement database/sql/driver
type Driver struct {
	cfg *config
	cli *http.Client
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
	driver.cli = &http.Client{}

	token, err := driver.login()
	if err != nil {
		return nil, err
	}

	return &conn{
		token: token,
		drv:   driver,
	}, nil
}

func (driver *Driver) login() (string, error) {
	url := "http://" +
		driver.cfg.addr + ":" +
		strconv.Itoa(driver.cfg.port) +
		"/rest/login/" +
		driver.cfg.user + "/" +
		driver.cfg.passwd

	res, err := driver.doGet(url)
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

func (driver *Driver) useDB() error {
	_, err := driver.query("use " + driver.cfg.dbName)
	return err
}

func (driver *Driver) query(sql string) ([]byte, error) {
	url := "http://" +
		driver.cfg.addr + ":" +
		strconv.Itoa(driver.cfg.port) +
		"/rest/sqlt/" +
		driver.cfg.dbName

	method := "POST"

	payload := strings.NewReader(sql)

	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Set the auth for the request.
	req.SetBasicAuth(driver.cfg.user, driver.cfg.passwd)
	req.Header.Add("Content-Type", "text/plain")

	res, err := driver.cli.Do(req)
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

	return body, nil
}

func (driver *Driver) doGet(urlStr string) ([]byte, error) {
	method := "GET"

	req, err := http.NewRequest(method, urlStr, nil)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	res, err := driver.cli.Do(req)
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

	return body, nil
}
