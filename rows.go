package taos

import (
	"database/sql/driver"
	"io"
)

// taosRows  myRowS implemmet for driver.Rows
type taosRows struct {
	Size        int64
	Len         int64
	Cols        []string
	ColumnMetas []*columnMeta
	Data        [][]interface{}
}

// Columns returns the names of the columns. The number of
// columns of the result is inferred from the length of the
// slice. If a particular column name isn't known, an empty
// string should be returned for that entry.
func (r *taosRows) Columns() []string {
	return r.Cols
}

// Close closes the rows iterator.
func (r *taosRows) Close() error {
	return nil
}

// Next is called to populate the next row of data into
// the provided slice. The provided slice will be the same
// size as the Columns() are wide.
//
// Next should return io.EOF when there are no more rows.
//
// The dest should not be written to outside of Next. Care
// should be taken when closing Rows not to modify
// a buffer held in dest.
func (r *taosRows) Next(dest []driver.Value) error {
	if r.Size == 0 {
		return io.EOF
	}

	for i, _ := range r.Cols {
		data := convert(r.ColumnMetas[i].Type, r.Data[r.Len-r.Size][i])
		dest[i] = data
	}

	r.Size--
	return nil
}
