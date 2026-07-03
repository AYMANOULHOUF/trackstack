package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/trackproj/api-service/internal/auth"
	"github.com/trackproj/api-service/internal/db"
	"github.com/trackproj/api-service/internal/handlers"
	"github.com/trackproj/api-service/internal/ws"
)

func main() {
	dsn := getenv("DATABASE_URL", "postgres://trackproj:trackproj@localhost:5432/trackproj?sslmode=disable")
	jwtSecret := getenv("JWT_SECRET", "change-me-in-production")
	listenAddr := getenv("LISTEN_ADDR", ":8080")

	conn, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer conn.Close()
	log.Printf("connected to postgres")

	// Seed / update the single global admin from env (compose-controlled).
	seedAdmin(conn, getenv("ADMIN_EMAIL", "admin@trackstack.local"), os.Getenv("ADMIN_PASSWORD"))

	jwtIssuer := auth.NewJWTIssuer(jwtSecret)
	hub := ws.NewHub()
	srv := handlers.NewServer(conn, jwtIssuer, hub)

	router := handlers.NewRouter(srv)

	httpSrv := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("api-service listening on %s", listenAddr)
	if err := httpSrv.ListenAndServe(); err != nil {
		log.Fatalf("ListenAndServe: %v", err)
	}
}

// seedAdmin upserts the global admin user (no org, is_admin = true) from env.
// Idempotent: rerun on every boot so rotating ADMIN_PASSWORD in compose takes
// effect on restart.
func seedAdmin(conn *sql.DB, email, password string) {
	if password == "" {
		log.Printf("WARNING: ADMIN_PASSWORD not set — admin account not seeded")
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Printf("seedAdmin: hash error: %v", err)
		return
	}
	_, err = conn.ExecContext(context.Background(), `
		INSERT INTO users (email, password_hash, is_admin, org_id, role)
		VALUES ($1, $2, true, NULL, 'admin')
		ON CONFLICT (email) DO UPDATE
		SET password_hash = EXCLUDED.password_hash, is_admin = true, org_id = NULL`,
		email, hash)
	if err != nil {
		log.Printf("seedAdmin: %v", err)
		return
	}
	log.Printf("admin account ready: %s", email)
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
