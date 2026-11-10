// Package forwarder sends normalized positions from protocol adapters to the
// api-service. Hardware trackers authenticate with the gateway secret and an
// external id; token-capable devices keep using the normal device endpoint.
package forwarder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Position struct {
	ExternalID string    `json:"external_id,omitempty"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Speed      *float64  `json:"speed,omitempty"`
	Heading    *float64  `json:"heading,omitempty"`
	Accuracy   *float64  `json:"accuracy,omitempty"`
	Battery    *float64  `json:"battery,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}

type Forwarder struct {
	APIURL string
	Secret string
	Client *http.Client
}

func New(apiURL, secret string) *Forwarder {
	return &Forwarder{
		APIURL: apiURL,
		Secret: secret,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Forward posts a hardware position to the secret-protected gateway endpoint.
func (f *Forwarder) Forward(ctx context.Context, p Position) error {
	if p.ExternalID == "" {
		return fmt.Errorf("external_id is required")
	}
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.APIURL+"/v1/gateway/positions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gateway-Secret", f.Secret)
	return f.do(req)
}

// ForwardWithToken proxies a device that already holds a long-lived token to
// the normal ingest endpoint, preserving the existing auth model.
func (f *Forwarder) ForwardWithToken(ctx context.Context, token string, p Position) error {
	if token == "" {
		return fmt.Errorf("device token is required")
	}
	body, err := json.Marshal(map[string]interface{}{
		"lat": p.Lat, "lon": p.Lon, "speed": p.Speed, "heading": p.Heading,
		"accuracy": p.Accuracy, "battery": p.Battery, "recorded_at": p.RecordedAt,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.APIURL+"/v1/positions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return f.do(req)
}

func (f *Forwarder) do(req *http.Request) error {
	resp, err := f.Client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("api-service returned %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	return nil
}
