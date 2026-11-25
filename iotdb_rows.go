package taos

import (
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"
)

type iotdbRows struct {
	mu       sync.Mutex
	columns  []string
	types    []string
	rows     [][]driver.Value
	position int
	closed   bool
}

func newIoTDBRows(response *iotdbQueryResponse, config *iotdbConfig) (*iotdbRows, error) {
	if response.ColumnNames == nil || response.DataTypes == nil || response.Values == nil {
		return nil, fmt.Errorf("IoTDB Table query response must contain column_names, data_types and values")
	}
	if len(response.Values) > config.rowLimit {
		return nil, fmt.Errorf("IoTDB query returned more than the requested row_limit of %d", config.rowLimit)
	}
	if len(response.ColumnNames) != len(response.DataTypes) {
		return nil, fmt.Errorf("IoTDB returned %d column names but %d data types", len(response.ColumnNames), len(response.DataTypes))
	}
	rows := &iotdbRows{
		columns: append([]string(nil), response.ColumnNames...),
		types:   append([]string(nil), response.DataTypes...),
		rows:    make([][]driver.Value, len(response.Values)),
	}
	for rowIndex, rawRow := range response.Values {
		if len(rawRow) != len(rows.columns) {
			return nil, fmt.Errorf("IoTDB row %d has %d values for %d columns", rowIndex, len(rawRow), len(rows.columns))
		}
		row := make([]driver.Value, len(rawRow))
		for columnIndex, rawValue := range rawRow {
			value, err := convertIoTDBValue(rawValue, rows.types[columnIndex], config.location)
			if err != nil {
				return nil, fmt.Errorf("convert IoTDB row %d column %q: %w", rowIndex, rows.columns[columnIndex], err)
			}
			row[columnIndex] = value
		}
		rows.rows[rowIndex] = row
	}
	return rows, nil
}

func (r *iotdbRows) Columns() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.columns...)
}

func (r *iotdbRows) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	r.rows = nil
	return nil
}

func (r *iotdbRows) Next(destination []driver.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.position >= len(r.rows) {
		return io.EOF
	}
	if len(destination) != len(r.columns) {
		return fmt.Errorf("destination has %d values for %d IoTDB columns", len(destination), len(r.columns))
	}
	copy(destination, r.rows[r.position])
	r.position++
	return nil
}

func (r *iotdbRows) ColumnTypeDatabaseTypeName(index int) string {
	if index < 0 || index >= len(r.types) {
		return ""
	}
	return strings.ToUpper(r.types[index])
}

func (r *iotdbRows) ColumnTypeScanType(index int) reflect.Type {
	if index < 0 || index >= len(r.types) {
		return reflect.TypeOf(new(interface{})).Elem()
	}
	switch strings.ToUpper(r.types[index]) {
	case "BOOLEAN", "BOOL":
		return reflect.TypeOf(false)
	case "INT32", "INT64", "TINYINT", "SMALLINT", "INT", "BIGINT":
		return reflect.TypeOf(int64(0))
	case "FLOAT", "DOUBLE":
		return reflect.TypeOf(float64(0))
	case "TIMESTAMP":
		return reflect.TypeOf(time.Time{})
	case "BLOB":
		return reflect.TypeOf([]byte(nil))
	case "STRING", "TEXT", "DATE":
		return reflect.TypeOf("")
	default:
		return reflect.TypeOf(new(interface{})).Elem()
	}
}

func (r *iotdbRows) ColumnTypeNullable(int) (bool, bool) {
	return false, false
}

func (r *iotdbRows) ColumnTypeLength(int) (int64, bool) {
	return 0, false
}

func (r *iotdbRows) HasNextResultSet() bool {
	return false
}

func (r *iotdbRows) NextResultSet() error {
	return io.EOF
}

var (
	_ driver.Rows                           = (*iotdbRows)(nil)
	_ driver.RowsColumnTypeDatabaseTypeName = (*iotdbRows)(nil)
	_ driver.RowsColumnTypeScanType         = (*iotdbRows)(nil)
	_ driver.RowsColumnTypeNullable         = (*iotdbRows)(nil)
	_ driver.RowsColumnTypeLength           = (*iotdbRows)(nil)
	_ driver.RowsNextResultSet              = (*iotdbRows)(nil)
)
