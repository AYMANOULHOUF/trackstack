package handlers

import (
	"database/sql"
	"time"
)

// Aliases so devices.go stays readable without importing database/sql directly.
type sql_NullFloat64 = sql.NullFloat64
type sql_NullTime struct {
	Time  time.Time
	Valid bool
}

func (n *sql_NullTime) Scan(src interface{}) error {
	if src == nil {
		n.Valid = false
		return nil
	}
	t, ok := src.(time.Time)
	if !ok {
		n.Valid = false
		return nil
	}
	n.Time = t
	n.Valid = true
	return nil
}

// deriveStatus computes moving / stopped / offline from last known position,
// never reading a stale column (Step 16 item 7 decision: status is computed,
// not stored).
func deriveStatus(lastAt sql_NullTime, offlineAfterSec int, tripStopSpeedKmh float64, lastSpeed sql_NullFloat64, trackingActive bool) string {
	if !lastAt.Valid {
		return "unknown"
	}
	if time.Since(lastAt.Time).Seconds() > float64(offlineAfterSec) {
		return "offline"
	}
	if !trackingActive {
		return "paused"
	}
	if lastSpeed.Valid && lastSpeed.Float64 > tripStopSpeedKmh {
		return "moving"
	}
	return "stopped"
}
