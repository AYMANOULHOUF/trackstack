package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trackproj/api-service/internal/auth"
	"github.com/trackproj/api-service/internal/db"
	"github.com/trackproj/api-service/internal/handlers"
	"github.com/trackproj/api-service/internal/notify"
	"github.com/trackproj/api-service/internal/trips"
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

	notifier := notify.NewDispatcher(conn, notify.Config{
		SMTPAddr:     os.Getenv("SMTP_ADDR"),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPass:     os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     os.Getenv("SMTP_FROM"),
		VAPIDPublic:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivate: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject: os.Getenv("VAPID_SUBJECT"),
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go notifier.Run(ctx)
	go trips.NewWorker(conn, durationEnv("TRIP_WORKER_INTERVAL", 30*time.Second)).Run(ctx)

	srv := handlers.NewServer(conn, jwtIssuer, hub, notifier, os.Getenv("GATEWAY_SECRET"))

	router := handlers.NewRouter(srv)

	httpSrv := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	log.Printf("api-service listening on %s", listenAddr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}
	log.Printf("api-service stopped")
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

func durationEnv(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Printf("invalid %s=%q, using %s", key, v, fallback)
		return fallback
	}
	return d
}
