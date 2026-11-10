// Package httpjson adapts simple HTTP/JSON trackers that cannot speak a binary
// protocol. Devices either present their normal bearer token or send the
// gateway secret plus an external_id.
package httpjson

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/trackproj/protocol-gateway/internal/forwarder"
)

type request struct {
	ExternalID string    `json:"external_id"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Speed      *float64  `json:"speed,omitempty"`
	Heading    *float64  `json:"heading,omitempty"`
	Accuracy   *float64  `json:"accuracy,omitempty"`
	Battery    *float64  `json:"battery,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}

func Handler(fwd *forwarder.Forwarder, gatewaySecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "POST only")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.RecordedAt.IsZero() {
			req.RecordedAt = time.Now().UTC()
		}

		position := forwarder.Position{
			ExternalID: req.ExternalID, Lat: req.Lat, Lon: req.Lon,
			Speed: req.Speed, Heading: req.Heading,
			Accuracy: req.Accuracy, Battery: req.Battery, RecordedAt: req.RecordedAt,
		}

		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		switch {
		case token != "":
			if err := fwd.ForwardWithToken(r.Context(), token, position); err != nil {
				writeErr(w, http.StatusBadGateway, err.Error())
				return
			}
		case req.ExternalID != "" && gatewaySecret != "" &&
			subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Gateway-Secret")), []byte(gatewaySecret)) == 1:
			if err := fwd.Forward(r.Context(), position); err != nil {
				writeErr(w, http.StatusBadGateway, err.Error())
				return
			}
		default:
			writeErr(w, http.StatusUnauthorized, "provide a device token or external_id with X-Gateway-Secret")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
