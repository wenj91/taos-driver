package taos

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"strconv"
	"time"
)

// func interpolateParams(query string, args []driver.Value) (string, error) {
// 	// Number of ? should be same to len(args)
// 	if strings.Count(query, "?") != len(args) {
// 		return "", driver.ErrSkip
// 	}
// 	buf := bytes.NewBufferString("")
// 	argPos := 0

// 	for i := 0; i < len(query); i++ {
// 		q := strings.IndexByte(query[i:], '?')
// 		if q == -1 {
// 			buf.WriteString(query[i:])
// 			break
// 		}
// 		buf.WriteString(query[i : i+q])
// 		i += q

// 		arg := args[argPos]
// 		argPos++

// 		if arg == nil {
// 			buf.WriteString("NULL")
// 			continue
// 		}
// 		switch v := arg.(type) {
// 		case int64:
// 			buf.WriteString(strconv.FormatInt(v, 10))
// 		case uint64:
// 			buf.WriteString(strconv.FormatUint(v, 10))
// 		case float64:
// 			buf.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
// 		case bool:
// 			if v {
// 				buf.WriteByte('1')
// 			} else {
// 				buf.WriteByte('0')
// 			}
// 		case time.Time:
// 			t := v.Format(time.RFC3339Nano)
// 			buf.WriteByte('\'')
// 			buf.WriteString(t)
// 			buf.WriteByte('\'')
// 		case []byte:
// 			buf.Write(v)
// 		case string:
// 			buf.WriteString(v)
// 		default:
// 			return "", driver.ErrSkip
// 		}
// 		if buf.Len() > MaxTaosSqlLen {
// 			return "", errors.New("sql statement exceeds the maximum length")
// 		}
// 	}
// 	if argPos != len(args) {
// 		return "", driver.ErrSkip
// 	}
// 	return buf.String(), nil
// }

// BOOL_TYPE      = 1
// TINYINT_TYPE   = 2
// SMALLINT_TYPE  = 3
// INT_TYPE       = 4
// BIGINT_TYPE    = 5
// FLOAT_TYPE     = 6
// DOUBLE_TYPE    = 7
// BINARY_TYPE    = 8
// TIMESTAMP_TYPE = 9
// NCHAR_TYPE     = 10
func convert(typ int, data interface{}) interface{} {

	if nil == data {
		return data
	}

	// todo: convert data to dest type
	switch typ {
	case BOOL_TYPE:
		b := data.(float64)
		return b == 1
	case TINYINT_TYPE:
		t := data.(float64)
		return int64(t)
	case SMALLINT_TYPE:
		si := data.(float64)
		return int64(si)
	case INT_TYPE:
		i := data.(float64)
		return int64(i)
	case BIGINT_TYPE:
		bi := data.(float64)
		return int64(bi)
	case FLOAT_TYPE:
	case DOUBLE_TYPE:

	case BINARY_TYPE:

	case TIMESTAMP_TYPE:
		ts := int64(data.(float64))

		tUnix := ts / int64(time.Microsecond)
		tUnixNanoRemainder := (ts % int64(time.Microsecond)) * int64(time.Millisecond)
		timeT := time.Unix(tUnix, tUnixNanoRemainder)

		return timeT
	case NCHAR_TYPE:

	}

	return data
}

func paramsCount(str string) int {
	stack := NewStack()
	sStack := NewStack()
	count := 0
	for i := 0; i < len(str); i++ {
		ch := str[i]
		if ch == '"' && stack.Len() == 0 && sStack.Len() == 0 {
			if i == 0 || str[i-1] != '\\' {
				stack.Push(ch)
			}
		} else if ch == '"' && stack.Len() > 0 && sStack.Len() == 0 {
			if i == 0 || str[i-1] != '\\' {
				stack.Pop()
			}
		}

		if ch == '\'' && stack.Len() == 0 && sStack.Len() == 0 {
			if i == 0 || str[i-1] != '\\' {
				sStack.Push(ch)
			}
		} else if ch == '\'' && sStack.Len() > 0 && stack.Len() == 0 {
			if i == 0 || str[i-1] != '\\' {
				sStack.Pop()
			}
		}

		if ch == '?' && stack.Len() == 0 && sStack.Len() == 0 {
			count++
		}
	}

	return count
}

func interpolateParams(query string, args []driver.Value) (string, error) {
	// Number of ? should be same to len(args)
	if paramsCount(query) != len(args) {
		return "", driver.ErrSkip
	}
	buf := bytes.NewBufferString("")
	argPos := 0

	stack := NewStack()
	sStack := NewStack()
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '"' && stack.Len() == 0 && sStack.Len() == 0 {
			if i == 0 || query[i-1] != '\\' {
				stack.Push(ch)
			}
		} else if ch == '"' && stack.Len() > 0 && sStack.Len() == 0 {
			if i == 0 || query[i-1] != '\\' {
				stack.Pop()
			}
		}

		if ch == '\'' && stack.Len() == 0 && sStack.Len() == 0 {
			if i == 0 || query[i-1] != '\\' {
				sStack.Push(ch)
			}
		} else if ch == '\'' && sStack.Len() > 0 && stack.Len() == 0 {
			if i == 0 || query[i-1] != '\\' {
				sStack.Pop()
			}
		}

		if ch == '?' && stack.Len() == 0 && sStack.Len() == 0 {
			arg := args[argPos]
			argPos++

			if arg == nil {
				buf.WriteString("NULL")
				continue
			}
			switch v := arg.(type) {
			case int64:
				buf.WriteString(strconv.FormatInt(v, 10))
			case uint64:
				buf.WriteString(strconv.FormatUint(v, 10))
			case float64:
				buf.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
			case bool:
				if v {
					buf.WriteByte('1')
				} else {
					buf.WriteByte('0')
				}
			case time.Time:
				t := v.Format(time.RFC3339Nano)
				buf.WriteByte('\'')
				buf.WriteString(t)
				buf.WriteByte('\'')
			case []byte:
				buf.Write(v)
			case string:
				buf.WriteString(v)
			default:
				return "", driver.ErrSkip
			}
		} else {
			buf.WriteByte(ch)
		}

		if buf.Len() > MaxTaosSqlLen {
			return "", errors.New("sql statement exceeds the maximum length")
		}
	}

	if argPos != len(args) {
		return "", driver.ErrSkip
	}
	return buf.String(), nil
}
