package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/trackproj/protocol-gateway/internal/forwarder"
	"github.com/trackproj/protocol-gateway/internal/protocols/gt06"
	"github.com/trackproj/protocol-gateway/internal/protocols/httpjson"
)

func main() {
	listenAddr := getenv("LISTEN_ADDR", ":8081")
	apiServiceURL := getenv("API_SERVICE_URL", "http://api-service:8080")
	gatewaySecret := os.Getenv("GATEWAY_SECRET")
	gt06Addr := getenv("GT06_LISTEN_ADDR", ":5023")

	fwd := forwarder.New(apiServiceURL, gatewaySecret)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// HTTP/JSON trackers (token-authenticated, or external_id + gateway secret).
	r.Post("/proto/http/positions", httpjson.Handler(fwd, gatewaySecret))

	// GT06/Concox trackers connect over raw TCP; expose GT06_LISTEN_ADDR
	// directly (Caddy only proxies HTTP).
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := gt06.NewServer(gt06Addr, fwd).ListenAndServe(ctx); err != nil {
			log.Printf("gt06 adapter stopped: %v", err)
		}
	}()

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("protocol-gateway listening on %s (gt06 on %s)", listenAddr, gt06Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
