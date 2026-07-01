package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	listenAddr := getenv("LISTEN_ADDR", ":8081")
	_ = getenv("API_SERVICE_URL", "http://api-service:8080")

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Future hardware protocol adapters mount here, e.g.:
	// r.Mount("/proto/gl200", gl200.Handler(fwder))
	// r.Mount("/proto/tk103", tk103.Handler(fwder))

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("protocol-gateway listening on %s", listenAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("ListenAndServe: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
