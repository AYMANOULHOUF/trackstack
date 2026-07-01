package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Connect opens a connection pool to Postgres using a standard DSN
// (e.g. "postgres://user:pass@host:5432/dbname?sslmode=disable").
func Connect(dsn string) (*sql.DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}

	conn.SetMaxOpenConns(20)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(30 * time.Minute)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("pinging db: %w", err)
	}

	return conn, nil
}
