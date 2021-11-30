package taos

import (
	"database/sql/driver"
	"io"
)

// TaosRows  myRowS implemmet for driver.Rows
type TaosRows struct {
	Size        int64
	Len         int64
	Cols        []string
	ColumnMetas []*ColumnMeta
	Data        [][]interface{}
}

// Columns returns the names of the columns. The number of
// columns of the result is inferred from the length of the
// slice. If a particular column name isn't known, an empty
// string should be returned for that entry.
func (r *TaosRows) Columns() []string {
	return r.Cols
}

// Close closes the rows iterator.
func (r *TaosRows) Close() error {
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
func (r *TaosRows) Next(dest []driver.Value) error {
	if r.Size == 0 {
		return io.EOF
	}

	for i, _ := range r.Cols {
		dest[i] = r.Data[r.Len-r.Size][i]
	}

	r.Size--
	return nil
}
