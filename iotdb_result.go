package taos

import "database/sql/driver"

// IoTDBResult exposes the status envelope returned by the non-query REST API.
type IoTDBResult struct {
	Code    int
	Message string
}

func (IoTDBResult) LastInsertId() (int64, error) {
	return 0, ErrIoTDBLastInsertIDUnavailable
}

func (IoTDBResult) RowsAffected() (int64, error) {
	return 0, ErrIoTDBRowsAffectedUnavailable
}

var _ driver.Result = IoTDBResult{}
