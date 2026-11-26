package taos

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The Table SQL lexer escapes a quote by doubling it. Backslashes are ordinary characters.
func ioTDBParamOffsets(sql string) []int {
	var offsets []int
	for i := 0; i < len(sql); i++ {
		switch sql[i] {
		case '?':
			offsets = append(offsets, i)
		case '\'', '"', '`':
			quote := sql[i]
			for i++; i < len(sql); i++ {
				if sql[i] == quote {
					if i+1 == len(sql) || sql[i+1] != quote {
						break
					}
					i++
				}
			}
		case '-':
			if strings.HasPrefix(sql[i:], "--") {
				end := strings.IndexAny(sql[i+2:], "\r\n")
				if end < 0 {
					return offsets
				}
				i += 2 + end
			}
		case '/':
			if strings.HasPrefix(sql[i:], "/*") {
				end := strings.Index(sql[i+2:], "*/")
				if end < 0 {
					return offsets
				}
				i += 3 + end
			}
		}
	}
	return offsets
}

func interpolateIoTDBParams(query string, args []driver.Value, config *iotdbConfig) (string, error) {
	if len(query) > maxIoTDBSQLLength {
		return "", fmt.Errorf("IoTDB SQL statement exceeds %d bytes", maxIoTDBSQLLength)
	}
	offsets := ioTDBParamOffsets(query)
	if len(offsets) != len(args) {
		return "", fmt.Errorf("IoTDB SQL has %d positional parameters but received %d values", len(offsets), len(args))
	}
	if len(args) == 0 {
		return query, nil
	}
	if !config.interpolateArgs {
		return "", fmt.Errorf("IoTDB REST API does not support server-side prepared parameters; enable interpolateParams")
	}

	var output strings.Builder
	start := 0
	for i, pos := range offsets {
		literal, err := ioTDBLiteral(args[i])
		if err != nil {
			return "", fmt.Errorf("encode IoTDB parameter %d: %w", i+1, err)
		}
		if len(literal) > maxIoTDBSQLLength-output.Len()-(pos-start) {
			return "", fmt.Errorf("IoTDB SQL statement exceeds %d bytes", maxIoTDBSQLLength)
		}
		output.WriteString(query[start:pos])
		output.WriteString(literal)
		start = pos + 1
	}
	output.WriteString(query[start:])
	if output.Len() > maxIoTDBSQLLength {
		return "", fmt.Errorf("IoTDB SQL statement exceeds %d bytes", maxIoTDBSQLLength)
	}
	return output.String(), nil
}

func ioTDBLiteral(value driver.Value) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "NULL", nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return "", errors.New("non-finite floating point values are not supported")
		}
		return strconv.FormatFloat(typed, 'g', -1, 64), nil
	case bool:
		return strconv.FormatBool(typed), nil
	case time.Time:
		if typed.Year() < 1 || typed.Year() > 9999 {
			return "", errors.New("time outside the Table datetime literal range")
		}
		// Table datetime literals carry an explicit numeric offset, not a session timezone.
		return typed.UTC().Format("2006-01-02T15:04:05.999999999-07:00"), nil
	case string:
		if !utf8.ValidString(typed) {
			return "", errors.New("string is not valid UTF-8; use []byte for BLOB")
		}
		if len(typed) > maxIoTDBSQLLength {
			return "", errors.New("string exceeds maximum SQL size")
		}
		return "'" + strings.ReplaceAll(typed, "'", "''") + "'", nil
	case []byte:
		if typed == nil {
			return "NULL", nil
		}
		if len(typed) > maxIoTDBSQLLength/2 {
			return "", errors.New("BLOB exceeds maximum SQL size")
		}
		return "X'" + hex.EncodeToString(typed) + "'", nil
	default:
		return "", fmt.Errorf("unsupported SQL parameter type %T", value)
	}
}

func convertIoTDBValue(raw json.RawMessage, dataType string, location *time.Location) (driver.Value, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	switch strings.ToUpper(dataType) {
	case "BOOLEAN", "BOOL":
		var value bool
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return nil, fmt.Errorf("expected BOOLEAN value: %w", err)
		}
		return value, nil
	case "INT32", "INT64", "TINYINT", "SMALLINT", "INT", "BIGINT":
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		decoder.UseNumber()
		var number json.Number
		if err := decoder.Decode(&number); err == nil {
			integer, err := number.Int64()
			if err == nil {
				return integer, nil
			}
		}
		return nil, fmt.Errorf("expected integer value")
	case "FLOAT", "DOUBLE":
		var number float64
		if err := json.Unmarshal(trimmed, &number); err != nil {
			return nil, fmt.Errorf("expected floating point value: %w", err)
		}
		return number, nil
	case "TIMESTAMP":
		return convertIoTDBTimestamp(trimmed, location)
	case "BLOB":
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return nil, fmt.Errorf("expected BLOB value: %w", err)
		}
		if !strings.HasPrefix(value, "0x") {
			return nil, errors.New("expected BLOB in 0x hexadecimal format")
		}
		return hex.DecodeString(value[2:])
	case "STRING", "TEXT", "DATE":
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return nil, fmt.Errorf("expected string value for %s: %w", dataType, err)
		}
		return value, nil
	default:
		return convertIoTDBGenericValue(trimmed)
	}
}

func convertIoTDBTimestamp(raw json.RawMessage, location *time.Location) (driver.Value, error) {
	if location == nil {
		location = time.UTC
	}
	var integer int64
	if err := json.Unmarshal(raw, &integer); err == nil {
		seconds := integer / 1000
		nanoseconds := (integer % 1000) * int64(time.Millisecond)
		return time.Unix(seconds, nanoseconds).In(location), nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("expected TIMESTAMP value: %w", err)
	}
	if integer, err := strconv.ParseInt(value, 10, 64); err == nil {
		seconds := integer / 1000
		nanoseconds := (integer % 1000) * int64(time.Millisecond)
		return time.Unix(seconds, nanoseconds).In(location), nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
		var parsed time.Time
		var err error
		if layout == time.RFC3339Nano {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, location)
		}
		if err == nil {
			return parsed.In(location), nil
		}
	}
	return nil, fmt.Errorf("unrecognized TIMESTAMP value %q", value)
}

func convertIoTDBGenericValue(raw json.RawMessage) (driver.Value, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	switch typed := value.(type) {
	case nil, bool, string:
		return typed, nil
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer, nil
		}
		return typed.Float64()
	case []interface{}, map[string]interface{}:
		return string(raw), nil
	default:
		return nil, fmt.Errorf("unsupported IoTDB value type %T", value)
	}
}
