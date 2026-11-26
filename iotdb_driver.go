package taos

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"net"
	"net/http"
	"time"
)

// IoTDBDriver implements database/sql over the IoTDB Table REST API.
type IoTDBDriver struct{}

type iotdbConnector struct {
	config *iotdbConfig
	driver *IoTDBDriver
}

func init() {
	sql.Register("iotdb", &IoTDBDriver{})
}

func (d *IoTDBDriver) Open(name string) (driver.Conn, error) {
	connector, err := d.OpenConnector(name)
	if err != nil {
		return nil, err
	}
	return connector.Connect(context.Background())
}

func (d *IoTDBDriver) OpenConnector(name string) (driver.Connector, error) {
	config, err := parseIoTDBDSN(name)
	if err != nil {
		return nil, err
	}
	return &iotdbConnector{config: config, driver: d}, nil
}

func (c *iotdbConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	transport.MaxConnsPerHost = 100
	transport.IdleConnTimeout = 90 * time.Second
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext

	return &iotdbConn{
		config: c.config,
		client: &http.Client{Transport: transport, Timeout: c.config.timeout},
	}, nil
}

func (c *iotdbConnector) Driver() driver.Driver {
	return c.driver
}

var (
	_ driver.Driver        = (*IoTDBDriver)(nil)
	_ driver.DriverContext = (*IoTDBDriver)(nil)
	_ driver.Connector     = (*iotdbConnector)(nil)
)
