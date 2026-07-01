package phone

// Package phone implements the HTTP-based protocol adapter for the Android
// companion app (Step 8: phone sends positions over HTTPS). In the current
// architecture the phone posts directly to api-service /v1/positions; this
// adapter exists as the future extension point for:
//   - Batch upload support (send N positions in one request when reconnecting)
//   - Protocol version negotiation
//   - Alternate encodings (msgpack, protobuf) if bandwidth becomes a concern
//
// For the initial scaffold it re-exposes /v1/positions with identical semantics
// so the gateway can be deployed in front of api-service as an optional layer.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/trackproj/protocol-gateway/internal/event"
)

// Adapter forwards positions to the api-service ingest endpoint.
type Adapter struct {
	APIServiceURL string // e.g. "http://api-service:8080"
	HTTPClient    *http.Client
}

func New(apiServiceURL string) *Adapter {
	return &Adapter{
		APIServiceURL: apiServiceURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Forward ships a normalized PositionEvent to the api-service, passing
// through the device's bearer token so api-service can authenticate it.
func (a *Adapter) Forward(ev event.PositionEvent, deviceToken string) error {
	body := map[string]interface{}{
		"lat":         ev.Lat,
		"lon":         ev.Lon,
		"speed":       ev.Speed,
		"heading":     ev.Heading,
		"accuracy":    ev.Accuracy,
		"battery":     ev.Battery,
		"recorded_at": ev.RecordedAt,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, a.APIServiceURL+"/v1/positions", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+deviceToken)

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api-service returned %d: %s", resp.StatusCode, b)
	}
	return nil
}
