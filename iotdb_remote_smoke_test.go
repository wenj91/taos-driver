package taos

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestIoTDBRemoteWriteRead(t *testing.T) {
	dsn := os.Getenv("IOTDB_TEST_WRITE_DSN")
	if dsn == "" {
		t.Skip("set IOTDB_TEST_WRITE_DSN for the Table REST write/read test")
	}
	config, err := parseIoTDBDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := sql.Open("iotdb", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var suffix [12]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	database := fmt.Sprintf("sqldriver_smoke_%x", suffix)
	table := database + ".readings"
	if _, err := admin.Exec("CREATE DATABASE " + database); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP DATABASE " + database); err != nil {
			t.Errorf("drop %s: %v", database, err)
		} else {
			t.Logf("Dropped %s", database)
		}
	}()
	config.database = database
	db := sql.OpenDB(&iotdbConnector{config: config, driver: &IoTDBDriver{}})
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE " + table + " (device STRING TAG, reading DOUBLE FIELD, note STRING FIELD)"); err != nil {
		t.Fatal(err)
	}
	wantTime := time.Unix(1700000000, 123000000).UTC()
	stmt, err := db.Prepare("INSERT INTO " + table + " (time,device,reading,note) VALUES (?,?,?,?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(wantTime, "sensor-a", -3.25, "O'Brien"); err != nil {
		t.Fatal(err)
	}
	var timestamp time.Time
	var reading float64
	var note string
	if err := db.QueryRow("SELECT time,device,reading,note FROM "+table+" WHERE device=? LIMIT 1", "sensor-a").Scan(&timestamp, new(string), &reading, &note); err != nil {
		t.Fatal(err)
	}
	if !timestamp.Equal(wantTime) || reading != -3.25 || note != "O'Brien" {
		t.Fatalf("read %s, %v, %q", timestamp, reading, note)
	}
}

func TestIoTDBRemoteReadOnlySmoke(t *testing.T) {
	dsn := os.Getenv("IOTDB_TEST_DSN")
	if dsn == "" {
		t.Skip("set IOTDB_TEST_DSN to run the read-only IoTDB smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := sql.Open("iotdb", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping IoTDB: %v", err)
	}

	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		t.Fatalf("query IoTDB databases: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) == 0 || columns[0] != "Database" {
		t.Fatalf("unexpected SHOW DATABASES columns: %v", columns)
	}

	databaseCount := 0
	for rows.Next() {
		var databaseName string
		destinations := make([]interface{}, len(columns))
		destinations[0] = &databaseName
		otherValues := make([]interface{}, len(columns)-1)
		for index := range otherValues {
			destinations[index+1] = &otherValues[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			t.Fatalf("scan SHOW DATABASES row: %v", err)
		}
		if databaseName == "" {
			t.Fatal("IoTDB returned an empty database name")
		}
		databaseCount++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate SHOW DATABASES results: %v", err)
	}
	if databaseCount == 0 {
		t.Fatal("IoTDB returned no visible databases")
	}
}
