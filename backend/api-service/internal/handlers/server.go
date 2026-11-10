package handlers

import (
	"database/sql"

	"github.com/trackproj/api-service/internal/auth"
	"github.com/trackproj/api-service/internal/notify"
	"github.com/trackproj/api-service/internal/ws"
)

// Server bundles the dependencies every handler needs. Constructed once in
// cmd/api/main.go and passed to route registration.
type Server struct {
	DB            *sql.DB
	JWT           *auth.JWTIssuer
	Hub           *ws.Hub
	Notifier      *notify.Dispatcher
	GatewaySecret string
}

func NewServer(db *sql.DB, jwt *auth.JWTIssuer, hub *ws.Hub, notifier *notify.Dispatcher, gatewaySecret string) *Server {
	return &Server{DB: db, JWT: jwt, Hub: hub, Notifier: notifier, GatewaySecret: gatewaySecret}
}
