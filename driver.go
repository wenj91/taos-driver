package taos

import (
	"database/sql/driver"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	// "strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
)

// Driver mydb driver for implement database/sql/driver
type Driver struct {
	cfg   *config
	cli   *http.Client
	token string
}

func init() {
	log.Println("driver is call ")
}

// Open for implement driver interface
func (driver *Driver) Open(name string) (driver.Conn, error) {
	// log.Println("exec open driver")

	cfg, err := parseDSN(name)
	if err != nil {
		return nil, err
	}

	driver.cfg = cfg

	// 简单的http client配置，todo: http client 连接池管理http连接
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100

	driver.cli = &http.Client{
		Timeout:   30 * time.Second,
		Transport: t,
	}

	token, err := driver.login()
	if err != nil {
		return nil, err
	}

	driver.token = token

	return &conn{
		drv: driver,
	}, nil
}

func (driver *Driver) login() (string, error) {
	url := "http://" +
		driver.cfg.getUri() +
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
		driver.cfg.getUri() +
		"/rest/sqlt/" +
		driver.cfg.dbName

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI(url)
	// fasthttp does not automatically request a gzipped response.
	// We must explicitly ask for it.
	// req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Add("Authorization", "Taosd "+driver.token)
	req.Header.Add("Content-Type", "text/plain")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetBody([]byte(sql))

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err := fasthttp.Do(req, resp)

	if err != nil {
		// log.Printf("Client get failed: %s\n", err)
		return nil, err
	}

	// Do we need to decompress the response?
	// contentEncoding := resp.Header.Peek("Content-Encoding")
	// var body []byte
	// if bytes.EqualFold(contentEncoding, []byte("gzip")) {
	// 	// log.Println("Unzipping...")
	// 	body, _ = resp.BodyGunzip()
	// } else {
	// 	body = resp.Body()
	// }

	// log.Println(string(body))

	return resp.Body(), nil
}

func (driver *Driver) doGet(urlStr string) ([]byte, error) {
	method := "GET"

	req, err := http.NewRequest(method, urlStr, nil)

	if err != nil {
		return nil, err
	}
	res, err := driver.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
