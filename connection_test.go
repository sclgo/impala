package impala

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/sclgo/impala/internal/fi"
	"github.com/stretchr/testify/require"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestIntegration(t *testing.T) {
	fi.SkipLongTest(t)

	dsn := os.Getenv("IMPALA_DSN")
	if dsn == "" {
		ctx := context.Background()
		c := fi.NoError(Setup(ctx)).Require(t)
		defer fi.NoErrorF(fi.Bind(c.Terminate, ctx), t)
		dsn = GetDsn(ctx, t, c)
	}

	conn := open(t, dsn)
	defer conn.Close()

	t.Run("Pinger", func(t *testing.T) {
		testPinger(t, conn)
	})
	t.Run("Select", func(t *testing.T) {
		testSelect(t, conn)
	})
}

func testPinger(t *testing.T, conn *sql.DB) {
	require.NoError(t, conn.Ping())
}

func testSelect(t *testing.T, db *sql.DB) {
	sampletime, _ := time.Parse(time.RFC3339, "2019-01-01T12:00:00Z")

	tests := []struct {
		sql string
		res interface{}
	}{
		{sql: "1", res: int8(1)},
		{sql: "cast(1 as smallint)", res: int16(1)},
		{sql: "cast(1 as int)", res: int32(1)},
		{sql: "cast(1 as bigint)", res: int64(1)},
		{sql: "cast(1.0 as float)", res: float64(1)},
		{sql: "cast(1.0 as double)", res: float64(1)},
		{sql: "cast(1.0 as real)", res: float64(1)},
		{sql: "'str'", res: "str"},
		{sql: "cast('str' as char(10))", res: "str       "},
		{sql: "cast('str' as varchar(100))", res: "str"},
		{sql: "cast('2019-01-01 12:00:00' as timestamp)", res: sampletime},
	}

	var res interface{}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			err := db.QueryRow(fmt.Sprintf("select %s", tt.sql)).Scan(&res)
			require.NoError(t, err)
			require.Equal(t, res, tt.res)
		})
	}
}

func open(t *testing.T, dsn string) *sql.DB {
	db, err := sql.Open("impala", dsn)
	if err != nil {
		t.Fatalf("Could not connect to %s: %s", dsn, err)
	}
	return db
}

const dbPort = "21050/tcp"

func Setup(ctx context.Context) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "apache/kudu:impala-latest",
		ExposedPorts: []string{dbPort},
		Cmd:          []string{"impala"},
		WaitingFor:   wait.ForLog("Impala has started.").WithStartupTimeout(3 * time.Minute),
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func GetDsn(ctx context.Context, t *testing.T, c testcontainers.Container) string {
	port := fi.NoError(c.MappedPort(ctx, dbPort)).Require(t).Port()
	host := fi.NoError(c.Host(ctx)).Require(t)
	u := &url.URL{
		Scheme: "impala",
		Host:   net.JoinHostPort(host, port),
		User:   url.User("impala"),
	}
	t.Log("url", u.String())
	return u.String()
}
