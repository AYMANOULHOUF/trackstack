package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/trackproj/api-service/internal/auth"
)

type ingestRequest struct {
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Speed      *float64  `json:"speed,omitempty"`
	Heading    *float64  `json:"heading,omitempty"`
	Accuracy   *float64  `json:"accuracy,omitempty"`
	Battery    *float64  `json:"battery,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}

// POST /v1/positions — authenticated with device token. Stores the fix, then
// fans it out to every org the device is assigned to (many-to-many) plus the
// admin firehose, running geofence + speed checks per org.
func (s *Server) IngestPosition(w http.ResponseWriter, r *http.Request) {
	deviceID := r.Context().Value(auth.CtxDeviceID).(string)

	var req ingestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RecordedAt.IsZero() {
		req.RecordedAt = time.Now().UTC()
	}

	// --- 1. Insert position ---
	var posID int64
	err := s.DB.QueryRowContext(r.Context(), `
		INSERT INTO positions (device_id, geom, speed, heading, accuracy, battery, recorded_at)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4, $5, $6, $7, $8)
		RETURNING id`,
		deviceID, req.Lon, req.Lat, req.Speed, req.Heading, req.Accuracy, req.Battery, req.RecordedAt,
	).Scan(&posID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not insert position")
		return
	}

	// --- 2. Update last_position_id ---
	if _, err := s.DB.ExecContext(r.Context(),
		`UPDATE devices SET last_position_id = $1 WHERE id = $2`, posID, deviceID,
	); err != nil {
		log.Printf("update last_position_id failed (non-fatal): %v", err)
	}

	// Per-device speed limit (global to the device, independent of org).
	var speedLimitKmh *float64
	_ = s.DB.QueryRowContext(r.Context(),
		`SELECT speed_limit_kmh FROM devices WHERE id = $1`, deviceID).Scan(&speedLimitKmh)

	// Orgs this device belongs to (may be empty → unassigned/pending).
	memberOrgs := []string{}
	if rows, err := s.DB.QueryContext(r.Context(),
		`SELECT org_id FROM device_orgs WHERE device_id = $1`, deviceID); err == nil {
		for rows.Next() {
			var o string
			if err := rows.Scan(&o); err == nil {
				memberOrgs = append(memberOrgs, o)
			}
		}
		rows.Close()
	}

	payload := map[string]interface{}{
		"type":        "position",
		"device_id":   deviceID,
		"lat":         req.Lat,
		"lon":         req.Lon,
		"speed":       req.Speed,
		"heading":     req.Heading,
		"battery":     req.Battery,
		"recorded_at": req.RecordedAt,
	}

	// --- 3. Speed violation (checked once, broadcast to all who can see it) ---
	var speedEvent *map[string]interface{}
	if req.Speed != nil && speedLimitKmh != nil && *req.Speed > *speedLimitKmh {
		if _, err := s.DB.ExecContext(r.Context(), `
			INSERT INTO speed_events (device_id, speed, limit_at_time)
			VALUES ($1, $2, $3)`, deviceID, *req.Speed, *speedLimitKmh,
		); err != nil {
			log.Printf("speed_event insert failed: %v", err)
		} else {
			e := map[string]interface{}{
				"type": "speed_violation", "device_id": deviceID,
				"speed": *req.Speed, "limit_at_time": *speedLimitKmh, "occurred_at": time.Now().UTC(),
			}
			speedEvent = &e
		}
	}

	// --- 4. Fan out per org (geofences are per org) + admin firehose ---
	for _, org := range memberOrgs {
		s.broadcast(org, payload)
		for _, gfe := range s.checkGeofences(r, deviceID, org, posID, req.Lat, req.Lon) {
			s.broadcast(org, gfe)
		}
		if speedEvent != nil {
			s.broadcast(org, *speedEvent)
		}
	}
	s.broadcast(AdminChannel, payload)
	if speedEvent != nil {
		s.broadcast(AdminChannel, *speedEvent)
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"position_id": posID})
}

func (s *Server) checkGeofences(r *http.Request, deviceID, orgID string, posID int64, lat, lon float64) []map[string]interface{} {
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT id, name FROM geofences
		WHERE org_id = $1
		  AND ST_Within(
		        ST_SetSRID(ST_MakePoint($2, $3), 4326)::geometry,
		        geom::geometry
		      )`, orgID, lon, lat)
	if err != nil {
		log.Printf("geofence check query failed: %v", err)
		return nil
	}
	defer rows.Close()

	nowInside := map[string]string{}
	for rows.Next() {
		var gid, gname string
		if err := rows.Scan(&gid, &gname); err == nil {
			nowInside[gid] = gname
		}
	}

	prevRows, err := s.DB.QueryContext(r.Context(), `
		SELECT DISTINCT ON (geofence_id) geofence_id, type
		FROM geofence_events
		WHERE device_id = $1
		ORDER BY geofence_id, occurred_at DESC`, deviceID)
	if err != nil {
		log.Printf("geofence prev-state query failed: %v", err)
		return nil
	}
	defer prevRows.Close()

	prevState := map[string]string{}
	for prevRows.Next() {
		var gid, evType string
		if err := prevRows.Scan(&gid, &evType); err == nil {
			prevState[gid] = evType
		}
	}

	var events []map[string]interface{}
	for gid, gname := range nowInside {
		if prevState[gid] == "enter" {
			continue
		}
		if err := s.insertGeofenceEvent(r, deviceID, gid, "enter"); err == nil {
			events = append(events, map[string]interface{}{
				"type": "geofence_enter", "device_id": deviceID,
				"geofence_id": gid, "geofence_name": gname, "occurred_at": time.Now().UTC(),
			})
		}
	}
	for gid, prev := range prevState {
		if prev != "enter" {
			continue
		}
		if _, stillIn := nowInside[gid]; stillIn {
			continue
		}
		var gname string
		_ = s.DB.QueryRowContext(r.Context(),
			`SELECT name FROM geofences WHERE id = $1`, gid).Scan(&gname)
		if err := s.insertGeofenceEvent(r, deviceID, gid, "exit"); err == nil {
			events = append(events, map[string]interface{}{
				"type": "geofence_exit", "device_id": deviceID,
				"geofence_id": gid, "geofence_name": gname, "occurred_at": time.Now().UTC(),
			})
		}
	}
	return events
}

func (s *Server) insertGeofenceEvent(r *http.Request, deviceID, geofenceID, evType string) error {
	_, err := s.DB.ExecContext(r.Context(),
		`INSERT INTO geofence_events (device_id, geofence_id, type) VALUES ($1, $2, $3)`,
		deviceID, geofenceID, evType)
	return err
}

func (s *Server) broadcast(orgID string, payload map[string]interface{}) {
	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("broadcast marshal error: %v", err)
		return
	}
	s.Hub.Broadcast(orgID, b)
}

// GET /v1/positions?device_id=&from=&to= — historical track. Admin can read
// any device; an org only devices assigned to it.
func (s *Server) ListPositions(w http.ResponseWriter, r *http.Request) {
	isAdmin, _ := r.Context().Value(auth.CtxIsAdmin).(bool)
	orgID, _ := r.Context().Value(auth.CtxOrgID).(string)
	q := r.URL.Query()

	deviceID := q.Get("device_id")
	if deviceID == "" {
		writeErr(w, http.StatusBadRequest, "device_id is required")
		return
	}

	var ok string
	if isAdmin {
		if err := s.DB.QueryRowContext(r.Context(),
			`SELECT id FROM devices WHERE id = $1`, deviceID).Scan(&ok); err != nil {
			writeErr(w, http.StatusNotFound, "device not found")
			return
		}
	} else {
		if err := s.DB.QueryRowContext(r.Context(),
			`SELECT device_id FROM device_orgs WHERE device_id = $1 AND org_id = $2`,
			deviceID, orgID).Scan(&ok); err != nil {
			writeErr(w, http.StatusNotFound, "device not found")
			return
		}
	}

	from := q.Get("from")
	to := q.Get("to")
	limit := 500

	args := []interface{}{deviceID}
	where := "device_id = $1"
	n := 2
	if from != "" {
		where += fmt.Sprintf(" AND recorded_at >= $%d", n)
		args = append(args, from)
		n++
	}
	if to != "" {
		where += fmt.Sprintf(" AND recorded_at <= $%d", n)
		args = append(args, to)
		n++
	}

	query := fmt.Sprintf(`
		SELECT id, ST_Y(geom::geometry) AS lat, ST_X(geom::geometry) AS lon,
		       speed, heading, accuracy, battery, recorded_at, received_at
		FROM positions WHERE %s ORDER BY recorded_at DESC LIMIT %d`, where, limit)

	rows, err := s.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not query positions")
		return
	}
	defer rows.Close()

	type posOut struct {
		ID         int64     `json:"id"`
		Lat        float64   `json:"lat"`
		Lon        float64   `json:"lon"`
		Speed      *float64  `json:"speed,omitempty"`
		Heading    *float64  `json:"heading,omitempty"`
		Accuracy   *float64  `json:"accuracy,omitempty"`
		Battery    *float64  `json:"battery,omitempty"`
		RecordedAt time.Time `json:"recorded_at"`
		ReceivedAt time.Time `json:"received_at"`
	}

	out := []posOut{}
	for rows.Next() {
		var p posOut
		var speed, heading, accuracy, battery sql_NullFloat64
		if err := rows.Scan(&p.ID, &p.Lat, &p.Lon,
			&speed, &heading, &accuracy, &battery,
			&p.RecordedAt, &p.ReceivedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan error")
			return
		}
		if speed.Valid {
			p.Speed = &speed.Float64
		}
		if heading.Valid {
			p.Heading = &heading.Float64
		}
		if accuracy.Valid {
			p.Accuracy = &accuracy.Float64
		}
		if battery.Valid {
			p.Battery = &battery.Float64
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, out)
}
