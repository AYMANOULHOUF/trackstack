package trips

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Worker periodically segments each device's new positions into trips.
//
// It is intentionally simple: one pass walks every device, reads the fixes
// newer than its cursor, and applies the actions returned by Segment. That is
// enough for a self-hosted fleet and keeps the logic easy to audit.
type Worker struct {
	DB       *sql.DB
	Interval time.Duration
	Batch    int
	Logger   *log.Logger
}

func NewWorker(db *sql.DB, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Worker{
		DB:       db,
		Interval: interval,
		Batch:    5000,
		Logger:   log.Default(),
	}
}

// Run blocks until ctx is cancelled, segmenting on every tick.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	w.Logger.Printf("trip worker started (interval %s)", w.Interval)
	for {
		select {
		case <-ctx.Done():
			w.Logger.Printf("trip worker stopped")
			return
		case <-ticker.C:
			if err := w.RunOnce(ctx); err != nil {
				w.Logger.Printf("trip worker pass failed: %v", err)
			}
		}
	}
}

// RunOnce segments every device once. Per-device failures are logged and do
// not stop the rest of the fleet.
func (w *Worker) RunOnce(ctx context.Context) error {
	rows, err := w.DB.QueryContext(ctx, `
		SELECT id, trip_stop_speed_kmh, trip_stop_duration_seconds, trip_cursor_position_id
		FROM devices ORDER BY id`)
	if err != nil {
		return fmt.Errorf("list devices: %w", err)
	}

	type device struct {
		id        string
		stopSpeed float64
		stopSecs  int
		cursor    int64
	}
	devices := []device{}
	for rows.Next() {
		var d device
		if err := rows.Scan(&d.id, &d.stopSpeed, &d.stopSecs, &d.cursor); err != nil {
			rows.Close()
			return fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("list devices: %w", err)
	}

	for _, d := range devices {
		if err := w.processDevice(ctx, d.id, d.stopSpeed, time.Duration(d.stopSecs)*time.Second, d.cursor); err != nil {
			w.Logger.Printf("trip worker: device %s: %v", d.id, err)
		}
	}
	return nil
}

func (w *Worker) processDevice(ctx context.Context, deviceID string, stopSpeed float64, stopDuration time.Duration, cursor int64) error {
	rows, err := w.DB.QueryContext(ctx, `
		SELECT id, ST_Y(geom::geometry), ST_X(geom::geometry), speed, recorded_at
		FROM positions
		WHERE device_id = $1 AND id > $2
		ORDER BY id ASC
		LIMIT $3`, deviceID, cursor, w.Batch)
	if err != nil {
		return fmt.Errorf("load positions: %w", err)
	}

	points := []Point{}
	for rows.Next() {
		var p Point
		var speed sql.NullFloat64
		if err := rows.Scan(&p.ID, &p.Lat, &p.Lon, &speed, &p.RecordedAt); err != nil {
			rows.Close()
			return fmt.Errorf("scan position: %w", err)
		}
		if speed.Valid {
			v := speed.Float64
			p.Speed = &v
		}
		points = append(points, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("load positions: %w", err)
	}
	if len(points) == 0 {
		return nil
	}

	st, openID, err := w.loadState(ctx, deviceID, cursor)
	if err != nil {
		return err
	}

	actions, next := Segment(points, st, stopSpeed, stopDuration)
	if len(actions) == 0 {
		return w.saveCursor(ctx, deviceID, next.Cursor)
	}

	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	for _, a := range actions {
		switch a.Kind {
		case ActionStart:
			if err := tx.QueryRowContext(ctx, `
				INSERT INTO trips (device_id, start_at, start_geom, end_at, end_geom, distance_m)
				VALUES ($1, $2,
				        ST_SetSRID(ST_MakePoint($4, $3), 4326)::geography,
				        $2,
				        ST_SetSRID(ST_MakePoint($4, $3), 4326)::geography,
				        0)
				RETURNING id`, deviceID, a.At, a.Lat, a.Lon).Scan(&openID); err != nil {
				return fmt.Errorf("insert trip: %w", err)
			}
		case ActionUpdate, ActionClose:
			if openID == 0 {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE trips SET
					end_at = $1,
					end_geom = ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography,
					distance_m = $4
				WHERE id = $5`, a.At, a.Lat, a.Lon, a.DistanceM, openID); err != nil {
				return fmt.Errorf("update trip: %w", err)
			}
			if a.Kind == ActionClose {
				openID = 0
			}
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE devices SET trip_cursor_position_id = $1 WHERE id = $2`, next.Cursor, deviceID); err != nil {
		return fmt.Errorf("save cursor: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (w *Worker) loadState(ctx context.Context, deviceID string, cursor int64) (State, int64, error) {
	st := State{Cursor: cursor}

	var (
		id             int64
		startAt        time.Time
		endAt          sql.NullTime
		distance       sql.NullFloat64
		endLat, endLon sql.NullFloat64
	)
	err := w.DB.QueryRowContext(ctx, `
		SELECT id, start_at, end_at, distance_m,
		       ST_Y(end_geom::geometry), ST_X(end_geom::geometry)
		FROM trips
		WHERE device_id = $1 AND end_at IS NULL
		ORDER BY start_at DESC
		LIMIT 1`, deviceID).Scan(&id, &startAt, &endAt, &distance, &endLat, &endLon)
	if err == sql.ErrNoRows {
		return st, 0, nil
	}
	if err != nil {
		return st, 0, fmt.Errorf("load open trip: %w", err)
	}

	st.Open = true
	st.TripStart = startAt
	if endAt.Valid {
		st.LastMovingAt = endAt.Time
	}
	if distance.Valid {
		st.DistanceM = distance.Float64
	}
	if endLat.Valid && endLon.Valid {
		st.LastLat, st.LastLon = endLat.Float64, endLon.Float64
	}
	return st, id, nil
}

func (w *Worker) saveCursor(ctx context.Context, deviceID string, cursor int64) error {
	if cursor == 0 {
		return nil
	}
	_, err := w.DB.ExecContext(ctx,
		`UPDATE devices SET trip_cursor_position_id = $1 WHERE id = $2`, cursor, deviceID)
	if err != nil {
		return fmt.Errorf("save cursor: %w", err)
	}
	return nil
}
