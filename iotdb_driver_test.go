package taos

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseIoTDBDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		wantURL  string
		wantDB   string
		wantUser string
	}{
		{
			name:     "Network-style DSN with escaped credentials and options",
			dsn:      "sensor:p%40ss@http(localhost:18080)/telemetry?row_limit=250&timeout=3s&interpolateParams=false",
			wantURL:  "http://localhost:18080",
			wantDB:   "telemetry",
			wantUser: "sensor",
		},
		{
			name:     "canonical URL and TLS",
			dsn:      "iotdbs://" + url.UserPassword("sensor", "p@ss").String() + "@localhost:18443/telemetry?model=table",
			wantURL:  "https://localhost:18443",
			wantDB:   "telemetry",
			wantUser: "sensor",
		},
		{
			name:     "empty database for database creation",
			dsn:      "sensor:secret@http(localhost:18080)/",
			wantURL:  "http://localhost:18080",
			wantDB:   "",
			wantUser: "sensor",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config, err := parseIoTDBDSN(test.dsn)
			if err != nil {
				t.Fatal(err)
			}
			if got := config.baseURL.String(); got != test.wantURL {
				t.Fatalf("base URL = %q, want %q", got, test.wantURL)
			}
			if config.database != test.wantDB || config.username != test.wantUser {
				t.Fatalf("database/user = %q/%q, want %q/%q", config.database, config.username, test.wantDB, test.wantUser)
			}
			if config.password != "p@ss" && test.name != "empty database for database creation" {
				t.Fatalf("password was not decoded correctly")
			}
		})
	}
}

func TestParseIoTDBDSNRejectsInvalidOptions(t *testing.T) {
	for _, dsn := range []string{
		"user:pass@http(localhost:0)/telemetry",
		"user:pass@tcp(localhost:18080)/telemetry",
		"user:pass@http(localhost:18080)/telemetry?row_limit=0",
		"user:pass@http(localhost:18080)/telemetry?model=tree",
		"iotdb://user@localhost:18080/telemetry",
	} {
		if _, err := parseIoTDBDSN(dsn); err == nil {
			t.Errorf("parseIoTDBDSN(%q) unexpectedly succeeded", dsn)
		}
	}
}

func TestIoTDBDatabaseSQLQueryExecAndPing(t *testing.T) {
	var queryRequest iotdbQueryRequest
	var execRequest iotdbNonQueryRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == iotdbPingEndpoint {
			if _, _, ok := request.BasicAuth(); ok {
				t.Error("ping should not send credentials")
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(writer, `{"code":200,"message":"SUCCESS_STATUS"}`)
			return
		}
		username, password, ok := request.BasicAuth()
		if !ok || username != "sensor" || password != "p@ss" {
			t.Errorf("unexpected authentication: %q", request.Header.Get("Authorization"))
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(writer, `{"code":801,"message":"WRONG_LOGIN_PASSWORD"}`)
			return
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", request.Header.Get("Content-Type"))
		}
		defer request.Body.Close()
		switch request.URL.Path {
		case iotdbQueryEndpoint:
			if err := json.NewDecoder(request.Body).Decode(&queryRequest); err != nil {
				t.Errorf("decode query request: %v", err)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, `{"column_names":["temperature","active","time"],"data_types":["FLOAT","BOOLEAN","TIMESTAMP"],"values":[[21.5,true,1739702535000],[null,false,"2025-02-16T07:02:16Z"]]}`)
		case iotdbNonQueryEndpoint:
			if err := json.NewDecoder(request.Body).Decode(&execRequest); err != nil {
				t.Errorf("decode non-query request: %v", err)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, `{"code":200,"message":"SUCCESS_STATUS"}`)
		default:
			t.Errorf("unexpected request path: %s", request.URL.Path)
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	db, err := openIoTDBTestDB(t, server.URL, "telemetry?row_limit=25&timeout=2s")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("PingContext: %v", err)
	}

	rows, err := db.QueryContext(context.Background(), "SELECT temperature, active, time FROM readings WHERE device_id = ?", "sensor'1")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	if queryRequest.Database != "telemetry" || queryRequest.RowLimit != 25 {
		t.Fatalf("query request database/limit = %q/%d", queryRequest.Database, queryRequest.RowLimit)
	}
	if queryRequest.SQL != "SELECT temperature, active, time FROM readings WHERE device_id = 'sensor''1'" {
		t.Fatalf("query SQL = %q", queryRequest.SQL)
	}
	if columns, err := rows.Columns(); err != nil || strings.Join(columns, ",") != "temperature,active,time" {
		t.Fatalf("Columns = %v, %v", columns, err)
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil || columnTypes[0].DatabaseTypeName() != "FLOAT" || columnTypes[1].ScanType().Kind() != reflect.Bool || columnTypes[2].ScanType().String() != "time.Time" {
		t.Fatalf("ColumnTypes = %v, %v", columnTypes, err)
	}
	if nullable, known := columnTypes[0].Nullable(); known || nullable {
		t.Fatalf("IoTDB nullability metadata = %v, known %v; want unknown", nullable, known)
	}
	if !rows.Next() {
		t.Fatalf("expected a query row, got error %v", rows.Err())
	}
	var temperature float64
	var active bool
	var timestamp time.Time
	if err := rows.Scan(&temperature, &active, &timestamp); err != nil {
		t.Fatal(err)
	}
	if temperature != 21.5 || !active || !timestamp.Equal(time.Date(2025, 2, 16, 10, 42, 15, 0, time.UTC)) {
		t.Fatalf("unexpected values: %v %v %v", temperature, active, timestamp)
	}
	if !rows.Next() {
		t.Fatalf("expected a second query row, got error %v", rows.Err())
	}
	var nullableTemperature sql.NullFloat64
	if err := rows.Scan(&nullableTemperature, &active, &timestamp); err != nil {
		t.Fatal(err)
	}
	if nullableTemperature.Valid || active || !timestamp.Equal(time.Date(2025, 2, 16, 7, 2, 16, 0, time.UTC)) {
		t.Fatalf("unexpected nullable row values: %v %v %v", nullableTemperature, active, timestamp)
	}
	if rows.Next() || rows.Err() != nil {
		t.Fatalf("unexpected additional row or error: %v", rows.Err())
	}
	_ = rows.Close()

	result, err := db.ExecContext(context.Background(), "CREATE TABLE readings (time TIMESTAMP TIME, temperature FLOAT FIELD)")
	if err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	if execRequest.Database != "telemetry" || execRequest.SQL != "CREATE TABLE readings (time TIMESTAMP TIME, temperature FLOAT FIELD)" {
		t.Fatalf("non-query request = %+v", execRequest)
	}
	if result == nil {
		t.Fatal("expected a non-nil Exec result")
	}
	if _, err := result.RowsAffected(); !errors.Is(err, ErrIoTDBRowsAffectedUnavailable) {
		t.Fatalf("RowsAffected error = %v", err)
	}
	if _, err := result.LastInsertId(); !errors.Is(err, ErrIoTDBLastInsertIDUnavailable) {
		t.Fatalf("LastInsertId error = %v", err)
	}
}

func TestIoTDBRESTErrorMappingAndClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == iotdbQueryEndpoint {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, `{"code":411,"message":"RESULT_ROW_LIMIT_EXCEEDED"}`)
			return
		}
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(writer, `{"code":801,"message":"WRONG_LOGIN_PASSWORD"}`)
	}))
	defer server.Close()

	db, err := openIoTDBTestDB(t, server.URL, "telemetry")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Query("SELECT * FROM readings")
	var restError *IoTDBRESTError
	if !errors.As(err, &restError) || restError.Code != 411 || restError.Message != "RESULT_ROW_LIMIT_EXCEEDED" {
		t.Fatalf("query error = %#v", err)
	}

	connector, err := (&IoTDBDriver{}).OpenConnector(makeIoTDBTestDSN(server.URL, "telemetry"))
	if err != nil {
		t.Fatal(err)
	}
	connection, err := connector.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := connection.(driver.QueryerContext).QueryContext(context.Background(), "SELECT * FROM readings", nil); !errors.Is(err, driver.ErrBadConn) {
		t.Fatalf("query after Close error = %v", err)
	}
}

func TestIoTDBTransactionsAreNotSupported(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	connector, err := (&IoTDBDriver{}).OpenConnector(makeIoTDBTestDSN(server.URL, "telemetry"))
	if err != nil {
		t.Fatal(err)
	}
	connection, err := connector.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.Begin(); !errors.Is(err, ErrIoTDBTransactionsNotSupported) {
		t.Fatalf("Begin error = %v", err)
	}
}

func TestIoTDBParameterInterpolationIgnoresQuotedAndCommentQuestionMarks(t *testing.T) {
	config, err := parseIoTDBDSN("user:secret@http(localhost:18080)/telemetry")
	if err != nil {
		t.Fatal(err)
	}
	query := `SELECT '?' AS literal, value FROM readings WHERE value = ? /* ? */ -- ?` + "\n"
	interpolated, err := interpolateIoTDBParams(query, []driver.Value{int64(7)}, config)
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT '?' AS literal, value FROM readings WHERE value = 7 /* ? */ -- ?` + "\n"
	if interpolated != want {
		t.Fatalf("interpolated SQL = %q, want %q", interpolated, want)
	}
}

func TestIoTDBParameterLiterals(t *testing.T) {
	testCases := []struct {
		name  string
		value driver.Value
		want  string
	}{
		{name: "escaped string", value: "x' OR 1=1 --", want: "'x'' OR 1=1 --'"},
		{name: "binary literal", value: []byte{0, 0xff, 0x41}, want: "X'00ff41'"},
		{name: "null", value: nil, want: "NULL"},
		{name: "boolean", value: true, want: "true"},
		{name: "datetime with offset", value: time.Date(2025, 2, 16, 12, 0, 0, 123000000, time.FixedZone("plus-eight", 8*60*60)), want: "2025-02-16T04:00:00.123+00:00"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ioTDBLiteral(testCase.value)
			if err != nil {
				t.Fatal(err)
			}
			if got != testCase.want {
				t.Fatalf("literal = %q, want %q", got, testCase.want)
			}
		})
	}
	if _, err := ioTDBLiteral(math.NaN()); err == nil {
		t.Fatal("NaN parameter should be rejected")
	}
	if _, err := ioTDBLiteral(uint64(1)); err == nil {
		t.Fatal("unsupported parameter type should be rejected")
	}
}

func TestIoTDBValueConversions(t *testing.T) {
	testCases := []struct {
		typeName string
		raw      string
		want     interface{}
	}{
		{typeName: "INT32", raw: "2147483647", want: int64(2147483647)},
		{typeName: "DOUBLE", raw: "21.5", want: float64(21.5)},
		{typeName: "STRING", raw: `"sensor-1"`, want: "sensor-1"},
		{typeName: "BLOB", raw: `"0x00ff41"`, want: []byte{0, 0xff, 0x41}},
		{typeName: "DATE", raw: `"2025-02-16"`, want: "2025-02-16"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.typeName, func(t *testing.T) {
			got, err := convertIoTDBValue(json.RawMessage(testCase.raw), testCase.typeName, time.UTC)
			if err != nil {
				t.Fatal(err)
			}
			switch want := testCase.want.(type) {
			case []byte:
				gotBytes, ok := got.([]byte)
				if !ok || !bytes.Equal(gotBytes, want) {
					t.Fatalf("value = %#v, want %#v", got, want)
				}
			default:
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("value = %#v (%T), want %#v (%T)", got, got, want, want)
				}
			}
		})
	}
	if got, err := convertIoTDBValue(json.RawMessage("null"), "FLOAT", time.UTC); err != nil || got != nil {
		t.Fatalf("SQL NULL conversion = %#v, %v", got, err)
	}
}

func TestIoTDBColumnTypeScanTypes(t *testing.T) {
	rows := &iotdbRows{types: []string{"TINYINT", "SMALLINT", "INT", "BIGINT", "TEXT", "DATE", "UNKNOWN"}}
	for index, typeName := range rows.types {
		got := rows.ColumnTypeScanType(index)
		switch typeName {
		case "TINYINT", "SMALLINT", "INT", "BIGINT":
			if got.Kind() != reflect.Int64 {
				t.Errorf("%s ScanType = %v, want int64", typeName, got)
			}
		case "TEXT", "DATE":
			if got.Kind() != reflect.String {
				t.Errorf("%s ScanType = %v, want string", typeName, got)
			}
		default:
			if got.Kind() != reflect.Interface {
				t.Errorf("%s ScanType = %v, want interface{}", typeName, got)
			}
		}
	}
}

func openIoTDBTestDB(t *testing.T, serverURL, database string) (*sql.DB, error) {
	t.Helper()
	connector, err := (&IoTDBDriver{}).OpenConnector(makeIoTDBTestDSN(serverURL, database))
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

func makeIoTDBTestDSN(serverURL, database string) string {
	hostPort := strings.TrimPrefix(serverURL, "http://")
	return fmt.Sprintf("sensor:p%%40ss@http(%s)/%s", hostPort, database)
}
