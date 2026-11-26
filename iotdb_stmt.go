package taos

import (
	"context"
	"database/sql/driver"
	"fmt"
	"sync"
)

type iotdbStmt struct {
	conn     *iotdbConn
	query    string
	numInput int

	mu     sync.Mutex
	closed bool
}

func (s *iotdbStmt) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func (s *iotdbStmt) NumInput() int {
	return s.numInput
}

func (s *iotdbStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), namedValuesFromIoTDB(args))
}

func (s *iotdbStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), namedValuesFromIoTDB(args))
}

func (s *iotdbStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	values, err := namedIoTDBValues(args)
	if err != nil {
		return nil, err
	}
	return s.conn.exec(ctx, s.query, values)
}

func (s *iotdbStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	values, err := namedIoTDBValues(args)
	if err != nil {
		return nil, err
	}
	return s.conn.query(ctx, s.query, values)
}

func (s *iotdbStmt) checkOpen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("IoTDB statement is closed")
	}
	return nil
}

func namedValuesFromIoTDB(values []driver.Value) []driver.NamedValue {
	args := make([]driver.NamedValue, len(values))
	for index, value := range values {
		args[index] = driver.NamedValue{Ordinal: index + 1, Value: value}
	}
	return args
}

var (
	_ driver.Stmt             = (*iotdbStmt)(nil)
	_ driver.StmtExecContext  = (*iotdbStmt)(nil)
	_ driver.StmtQueryContext = (*iotdbStmt)(nil)
)
