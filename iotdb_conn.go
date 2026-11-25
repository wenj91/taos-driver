package taos

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

var (
	ErrIoTDBTransactionsNotSupported = errors.New("IoTDB REST API does not support transactions")
	ErrIoTDBLastInsertIDUnavailable  = errors.New("IoTDB REST API does not report a last insert ID")
	ErrIoTDBRowsAffectedUnavailable  = errors.New("IoTDB REST API does not report rows affected")
)

type iotdbConn struct {
	config *iotdbConfig
	client *http.Client

	mu     sync.Mutex
	closed bool
}

func (c *iotdbConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *iotdbConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.checkOpen(); err != nil {
		return nil, err
	}
	if len(query) > maxIoTDBSQLLength {
		return nil, fmt.Errorf("IoTDB SQL statement exceeds %d bytes", maxIoTDBSQLLength)
	}
	return &iotdbStmt{conn: c, query: query, numInput: len(ioTDBParamOffsets(query))}, nil
}

func (c *iotdbConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	c.client.CloseIdleConnections()
	return nil
}

func (c *iotdbConn) Begin() (driver.Tx, error) {
	return nil, ErrIoTDBTransactionsNotSupported
}

func (c *iotdbConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return nil, ErrIoTDBTransactionsNotSupported
}

func (c *iotdbConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	values, err := namedIoTDBValues(args)
	if err != nil {
		return nil, err
	}
	return c.exec(ctx, query, values)
}

func (c *iotdbConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	values, err := namedIoTDBValues(args)
	if err != nil {
		return nil, err
	}
	return c.query(ctx, query, values)
}

func (c *iotdbConn) Ping(ctx context.Context) error {
	return c.ping(ctx)
}

func (c *iotdbConn) CheckNamedValue(value *driver.NamedValue) error {
	if value.Name != "" {
		return fmt.Errorf("IoTDB REST driver supports positional ? parameters only")
	}
	converted, err := driver.DefaultParameterConverter.ConvertValue(value.Value)
	if err != nil {
		return err
	}
	value.Value = converted
	return nil
}

func (c *iotdbConn) IsValid() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed
}

func (c *iotdbConn) ResetSession(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.checkOpen()
}

func (c *iotdbConn) exec(ctx context.Context, statement string, args []driver.Value) (driver.Result, error) {
	sqlText, err := interpolateIoTDBParams(statement, args, c.config)
	if err != nil {
		return nil, err
	}
	request := iotdbNonQueryRequest{Database: c.config.database, SQL: sqlText}
	var response iotdbRESTEnvelope
	if err := c.postJSON(ctx, iotdbNonQueryEndpoint, request, &response, true); err != nil {
		return nil, err
	}
	return IoTDBResult{Code: response.Code, Message: response.Message}, nil
}

func (c *iotdbConn) query(ctx context.Context, statement string, args []driver.Value) (driver.Rows, error) {
	if c.config.database == "" {
		return nil, fmt.Errorf("IoTDB Table queries require a database in the DSN")
	}
	sqlText, err := interpolateIoTDBParams(statement, args, c.config)
	if err != nil {
		return nil, err
	}
	request := iotdbQueryRequest{Database: c.config.database, SQL: sqlText, RowLimit: c.config.rowLimit}
	var response iotdbQueryResponse
	if err := c.postJSON(ctx, iotdbQueryEndpoint, request, &response, false); err != nil {
		return nil, err
	}
	return newIoTDBRows(&response, c.config)
}

func (c *iotdbConn) checkOpen() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return driver.ErrBadConn
	}
	return nil
}

func namedIoTDBValues(args []driver.NamedValue) ([]driver.Value, error) {
	values := make([]driver.Value, len(args))
	for index, arg := range args {
		if arg.Name != "" {
			return nil, fmt.Errorf("IoTDB REST driver supports positional ? parameters only")
		}
		if arg.Ordinal != index+1 {
			return nil, fmt.Errorf("invalid IoTDB parameter ordinal %d", arg.Ordinal)
		}
		value, err := driver.DefaultParameterConverter.ConvertValue(arg.Value)
		if err != nil {
			return nil, err
		}
		values[index] = value
	}
	return values, nil
}

var (
	_ driver.Conn               = (*iotdbConn)(nil)
	_ driver.ConnBeginTx        = (*iotdbConn)(nil)
	_ driver.ExecerContext      = (*iotdbConn)(nil)
	_ driver.QueryerContext     = (*iotdbConn)(nil)
	_ driver.Pinger             = (*iotdbConn)(nil)
	_ driver.NamedValueChecker  = (*iotdbConn)(nil)
	_ driver.SessionResetter    = (*iotdbConn)(nil)
	_ driver.Validator          = (*iotdbConn)(nil)
	_ driver.ConnPrepareContext = (*iotdbConn)(nil)
)
