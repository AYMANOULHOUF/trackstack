// Package trips turns a stream of position fixes into trips.
//
// A trip starts once the device reports motion on two consecutive fixes: the
// first moving fix is only a candidate, because a single fix above the stop
// speed is usually GPS noise or a brief pull-away. It stays open while the
// device keeps moving and closes once it has been stationary for
// trip_stop_duration_seconds. The pure Segment function contains all of the
// decision logic so it can be tested without a database; Worker applies the
// resulting actions to Postgres.
package trips

import (
	"math"
	"time"
)

// Point is the subset of a position row the segmenter needs.
type Point struct {
	ID         int64
	Lat        float64
	Lon        float64
	Speed      *float64
	RecordedAt time.Time
}

// State is the segmenter's memory between batches. It mirrors the open trip
// row plus the position cursor.
type State struct {
	Open         bool
	TripStart    time.Time
	LastMovingAt time.Time
	LastLat      float64
	LastLon      float64
	DistanceM    float64
	Cursor       int64

	// Pending marks a moving fix that has not been confirmed by a second
	// moving fix yet. The cursor deliberately stays before it so a worker
	// restart re-reads the same fix instead of losing the candidate.
	Pending    bool
	PendingAt  time.Time
	PendingLat float64
	PendingLon float64
	PendingID  int64
}

// ActionKind describes what the worker should do with a trip row.
type ActionKind string

const (
	// ActionStart inserts a new trip and makes it the open trip.
	ActionStart ActionKind = "start"
	// ActionUpdate moves the open trip's end point forward and updates distance.
	ActionUpdate ActionKind = "update"
	// ActionClose closes the open trip at the last moving fix.
	ActionClose ActionKind = "close"
)

// Action is a single database operation derived from a batch of fixes.
type Action struct {
	Kind      ActionKind
	At        time.Time
	Lat       float64
	Lon       float64
	DistanceM float64
}

// Segment processes points in order and returns the actions to apply.
//
// It is deliberately independent of SQL: the worker loads an open trip into
// State, calls Segment, then executes the actions and stores the new cursor.
func Segment(points []Point, st State, stopSpeedKmh float64, stopDuration time.Duration) ([]Action, State) {
	actions := make([]Action, 0, len(points))

	for _, p := range points {
		if p.ID <= st.Cursor {
			continue
		}
		moving := p.Speed != nil && *p.Speed > stopSpeedKmh

		// A long silence between moving fixes means the previous trip ended
		// where the device stopped, not where it reappeared.
		if st.Open && p.RecordedAt.Sub(st.LastMovingAt) >= stopDuration {
			actions = append(actions, Action{
				Kind: ActionClose, At: st.LastMovingAt,
				Lat: st.LastLat, Lon: st.LastLon, DistanceM: st.DistanceM,
			})
			st.Open = false
			st.DistanceM = 0
			st.LastMovingAt = time.Time{}
		}

		if !st.Open {
			if !moving {
				// Stationary fix: any unconfirmed candidate was a one-off.
				st.Pending = false
				st.Cursor = p.ID
				continue
			}

			// A candidate that was never confirmed within the stop window is
			// discarded rather than turned into a zero-length trip.
			if st.Pending && p.RecordedAt.Sub(st.PendingAt) >= stopDuration {
				st.Pending = false
			}

			if !st.Pending {
				st.Pending = true
				st.PendingAt = p.RecordedAt
				st.PendingLat, st.PendingLon = p.Lat, p.Lon
				st.PendingID = p.ID
				continue
			}

			// Second moving fix: the candidate becomes a real trip.
			st.Open = true
			st.TripStart = st.PendingAt
			st.LastMovingAt = st.PendingAt
			st.LastLat, st.LastLon = st.PendingLat, st.PendingLon
			st.DistanceM = 0
			st.Pending = false
			st.Cursor = st.PendingID
			actions = append(actions, Action{
				Kind: ActionStart, At: st.PendingAt, Lat: st.LastLat, Lon: st.LastLon,
			})
			// Fall through so this fix also advances the trip.
		}

		if !moving {
			st.Cursor = p.ID
			continue
		}

		if d := HaversineM(st.LastLat, st.LastLon, p.Lat, p.Lon); d > 3 {
			st.DistanceM += d
		}
		st.LastMovingAt = p.RecordedAt
		st.LastLat, st.LastLon = p.Lat, p.Lon
		st.Cursor = p.ID
		actions = append(actions, Action{
			Kind: ActionUpdate, At: p.RecordedAt, Lat: p.Lat, Lon: p.Lon, DistanceM: st.DistanceM,
		})
	}

	return actions, st
}

// HaversineM returns the great-circle distance in metres.
func HaversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6371000.0
	toRad := func(deg float64) float64 { return deg * math.Pi / 180 }

	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
