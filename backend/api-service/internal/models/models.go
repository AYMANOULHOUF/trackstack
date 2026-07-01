package models

import "time"

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type Device struct {
	ID                       string     `json:"id"`
	OrgID                    string     `json:"org_id"`
	Name                     string     `json:"name"`
	Type                     string     `json:"type"`
	SpeedLimitKmh            *float64   `json:"speed_limit_kmh,omitempty"`
	TripStopSpeedKmh         float64    `json:"trip_stop_speed_kmh"`
	TripStopDurationSeconds  int        `json:"trip_stop_duration_seconds"`
	OfflineAfterSeconds      int        `json:"offline_after_seconds"`
	LastPositionID           *int64     `json:"last_position_id,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
}

// DeviceCreateResponse includes the plaintext API token, returned only once
// at creation/rotation time (Step 15: "shown once, hashed at rest").
type DeviceCreateResponse struct {
	Device
	APIToken string `json:"api_token"`
}

type Position struct {
	ID         int64     `json:"id"`
	DeviceID   string    `json:"device_id"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Speed      *float64  `json:"speed,omitempty"`
	Heading    *float64  `json:"heading,omitempty"`
	Accuracy   *float64  `json:"accuracy,omitempty"`
	Battery    *float64  `json:"battery,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
	ReceivedAt time.Time `json:"received_at"`
}

// PositionIngest is the shape accepted by POST /v1/positions. It is
// intentionally protocol-agnostic (read_me.md Step 15 / Step 8) so the
// future protocol-gateway can write through the same internal function.
type PositionIngest struct {
	DeviceID   string    `json:"device_id,omitempty"` // set internally for token-authed phone uploads
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Speed      *float64  `json:"speed,omitempty"`
	Heading    *float64  `json:"heading,omitempty"`
	Accuracy   *float64  `json:"accuracy,omitempty"`
	Battery    *float64  `json:"battery,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}

type Geofence struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	// GeoJSON polygon, e.g. {"type":"Polygon","coordinates":[[[lon,lat],...]]}
	GeoJSON   string    `json:"geojson"`
	CreatedAt time.Time `json:"created_at"`
}

type GeofenceEvent struct {
	ID          int64     `json:"id"`
	DeviceID    string    `json:"device_id"`
	GeofenceID  string    `json:"geofence_id"`
	Type        string    `json:"type"` // enter | exit
	OccurredAt  time.Time `json:"occurred_at"`
}

type SpeedEvent struct {
	ID          int64     `json:"id"`
	DeviceID    string    `json:"device_id"`
	Speed       float64   `json:"speed"`
	LimitAtTime float64   `json:"limit_at_time"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type Trip struct {
	ID         int64      `json:"id"`
	DeviceID   string     `json:"device_id"`
	StartAt    time.Time  `json:"start_at"`
	EndAt      *time.Time `json:"end_at,omitempty"`
	DistanceM  *float64   `json:"distance_m,omitempty"`
}

type ActivityLog struct {
	ID          int64     `json:"id"`
	OrgID       string    `json:"org_id"`
	ActorUserID *string   `json:"actor_user_id,omitempty"`
	Action      string    `json:"action"`
	TargetType  string    `json:"target_type,omitempty"`
	TargetID    string    `json:"target_id,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// DeviceStatus is computed at query time, never stored (read_me.md Step 16
// item 7), to avoid stale-status bugs.
type DeviceStatus string

const (
	StatusMoving  DeviceStatus = "moving"
	StatusStopped DeviceStatus = "stopped"
	StatusOffline DeviceStatus = "offline"
)
