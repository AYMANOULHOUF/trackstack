package ws

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Self-hosted: dashboard is served from the same origin behind Caddy by
	// default. Tighten this with an explicit allow-list if you expose the
	// API on a different origin than the dashboard.
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	conn  *websocket.Conn
	orgID string
	send  chan []byte
}

// Hub fans out events (new positions, geofence enter/exit, speed
// violations) to connected dashboard clients, scoped by org_id so one
// org never sees another org's devices (Step 15 /v1/live).
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

// Broadcast sends a JSON-encoded message to every client belonging to orgID.
func (h *Hub) Broadcast(orgID string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.orgID != orgID {
			continue
		}
		select {
		case c.send <- payload:
		default:
			log.Printf("ws: dropping message for slow client (org %s)", orgID)
		}
	}
}

// ServeWS upgrades the connection and registers the client under orgID.
// orgID must already be derived from a verified JWT before calling this.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, orgID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	c := &client{conn: conn, orgID: orgID, send: make(chan []byte, 32)}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go h.writePump(c)
	h.readPump(c) // blocks until disconnect; cleans up below
}

func (h *Hub) readPump(c *client) {
	defer h.disconnect(c)
	c.conn.SetReadLimit(512)
	for {
		// Dashboard clients are receive-only; we just drain pings/closes.
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) writePump(c *client) {
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (h *Hub) disconnect(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
		c.conn.Close()
	}
}
