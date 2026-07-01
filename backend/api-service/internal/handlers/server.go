package handlers

import (
	"database/sql"

	"github.com/trackproj/api-service/internal/auth"
	"github.com/trackproj/api-service/internal/ws"
)

// Server bundles the dependencies every handler needs. Constructed once in
// cmd/api/main.go and passed to route registration.
type Server struct {
	DB     *sql.DB
	JWT    *auth.JWTIssuer
	Hub    *ws.Hub
}

func NewServer(db *sql.DB, jwt *auth.JWTIssuer, hub *ws.Hub) *Server {
	return &Server{DB: db, JWT: jwt, Hub: hub}
}
