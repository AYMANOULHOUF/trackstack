package handlers

import (
	"crypto/subtle"
	"net/http"
	"time"
)

type gatewayIngestRequest struct {
	ExternalID string    `json:"external_id"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Speed      *float64  `json:"speed,omitempty"`
	Heading    *float64  `json:"heading,omitempty"`
	Accuracy   *float64  `json:"accuracy,omitempty"`
	Battery    *float64  `json:"battery,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}

// POST /v1/gateway/positions — called by the protocol gateway, never by
// browsers or devices. The shared secret is compared in constant time and the
// external_id (IMEI/serial) is resolved to a device row before ingest.
func (s *Server) GatewayIngestPosition(w http.ResponseWriter, r *http.Request) {
	if s.GatewaySecret == "" {
		writeErr(w, http.StatusServiceUnavailable, "gateway ingest is not configured")
		return
	}
	got := r.Header.Get("X-Gateway-Secret")
	if subtle.ConstantTimeCompare([]byte(got), []byte(s.GatewaySecret)) != 1 {
		writeErr(w, http.StatusUnauthorized, "invalid gateway secret")
		return
	}

	var req gatewayIngestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExternalID == "" {
		writeErr(w, http.StatusBadRequest, "external_id is required")
		return
	}

	var deviceID string
	if err := s.DB.QueryRowContext(r.Context(),
		`SELECT id FROM devices WHERE external_id = $1`, req.ExternalID,
	).Scan(&deviceID); err != nil {
		writeErr(w, http.StatusNotFound, "no device with that external_id")
		return
	}

	posID, err := s.ingestPosition(r.Context(), deviceID, ingestRequest{
		Lat: req.Lat, Lon: req.Lon, Speed: req.Speed, Heading: req.Heading,
		Accuracy: req.Accuracy, Battery: req.Battery, RecordedAt: req.RecordedAt,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"position_id": posID, "device_id": deviceID,
	})
}
