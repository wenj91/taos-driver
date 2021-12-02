package taos

import ( 
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestParamsCount(t *testing.T) {

	sqlStr := "select * from tb1"
	count := paramsCount(sqlStr)
	assert.Equal(t, 0, count, "count should be equal 0.")

	sqlStr = "select * from tb1 where a = ?"
	count = paramsCount(sqlStr)
	assert.Equal(t, 1, count, "count should be equal 1.")

	sqlStr = `select "xx?xx" from tb1`
	count = paramsCount(sqlStr)
	assert.Equal(t, 0, count, "count should be equal 0.")

	sqlStr = `select "xx?xx" from tb1 where a = ?`
	count = paramsCount(sqlStr)
	assert.Equal(t, 1, count, "count should be equal 1.")

	sqlStr = `select 'xx?xx' from tb1 where a = ?`
	count = paramsCount(sqlStr)
	assert.Equal(t, 1, count, "count should be equal 1.")

	sqlStr = `select 'xx?xx' aa, "\"???" from tb1 where a = ?`
	count = paramsCount(sqlStr)
	assert.Equal(t, 1, count, "count should be equal 1.")
}