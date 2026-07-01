package event

import "time"

// PositionEvent is the normalized, protocol-agnostic shape that every
// protocol adapter (phone HTTP, future hardware trackers) resolves to
// before forwarding to the shared ingest function.
// Fields mirror models.PositionIngest in api-service (Step 8 / Step 15).
type PositionEvent struct {
	DeviceID   string    `json:"device_id"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Speed      *float64  `json:"speed,omitempty"`
	Heading    *float64  `json:"heading,omitempty"`
	Accuracy   *float64  `json:"accuracy,omitempty"`
	Battery    *float64  `json:"battery,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}
