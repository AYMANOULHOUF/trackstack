package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/trackproj/api-service/internal/auth"
)

type enrollRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// POST /v1/enroll — public. A freshly installed phone registers itself and
// receives its device token once. It starts unassigned (no org), visible only
// to the admin until assigned to one or more orgs.
func (s *Server) EnrollDevice(w http.ResponseWriter, r *http.Request) {
	var req enrollRequest
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Type != "phone" && req.Type != "hardware" {
		req.Type = "phone"
	}
	plaintext, err := auth.GenerateDeviceToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not generate device token")
		return
	}
	hash := auth.HashDeviceToken(plaintext)

	var id, createdAt string
	err = s.DB.QueryRowContext(r.Context(), `
		INSERT INTO devices (name, type, api_token_hash)
		VALUES ($1, $2, $3) RETURNING id, created_at`,
		req.Name, req.Type, hash,
	).Scan(&id, &createdAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not enroll device")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id": id, "name": req.Name, "type": req.Type, "created_at": createdAt, "api_token": plaintext,
	})
}

type orgRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type deviceOut struct {
	ID                      string   `json:"id"`
	Name                    string   `json:"name"`
	Type                    string   `json:"type"`
	SpeedLimitKmh           *float64 `json:"speed_limit_kmh,omitempty"`
	TripStopSpeedKmh        float64  `json:"trip_stop_speed_kmh"`
	TripStopDurationSeconds int      `json:"trip_stop_duration_seconds"`
	OfflineAfterSeconds     int      `json:"offline_after_seconds"`
	CreatedAt               string   `json:"created_at"`
	LastRecordedAt          *string  `json:"last_recorded_at,omitempty"`
	LastSpeed               *float64 `json:"last_speed,omitempty"`
	LastLat                 *float64 `json:"last_lat,omitempty"`
	LastLon                 *float64 `json:"last_lon,omitempty"`
	Status                  string   `json:"status"`
	TrackingActive          bool     `json:"tracking_active"`
	Orgs                    []orgRef `json:"orgs"`
	Assigned                bool     `json:"assigned"`
}

const deviceCols = `d.id, d.name, d.type, d.speed_limit_kmh, d.trip_stop_speed_kmh,
	d.trip_stop_duration_seconds, d.offline_after_seconds, d.tracking_active, d.created_at,
	p.recorded_at, p.speed, ST_Y(p.geom::geometry), ST_X(p.geom::geometry)`

// GET /v1/devices — admin sees every device (with org memberships + unassigned
// flag); an org sees only devices assigned to it.
func (s *Server) ListDevices(w http.ResponseWriter, r *http.Request) {
	isAdmin, _ := r.Context().Value(auth.CtxIsAdmin).(bool)
	orgID, _ := r.Context().Value(auth.CtxOrgID).(string)

	var query string
	var args []interface{}
	if isAdmin {
		query = `SELECT ` + deviceCols + ` FROM devices d
			LEFT JOIN positions p ON p.id = d.last_position_id
			ORDER BY d.created_at DESC`
	} else {
		query = `SELECT ` + deviceCols + ` FROM devices d
			JOIN device_orgs dou ON dou.device_id = d.id AND dou.org_id = $1
			LEFT JOIN positions p ON p.id = d.last_position_id
			ORDER BY d.created_at DESC`
		args = append(args, orgID)
	}

	rows, err := s.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list devices")
		return
	}
	defer rows.Close()

	byID := map[string]*deviceOut{}
	out := []*deviceOut{}
	for rows.Next() {
		d := &deviceOut{Orgs: []orgRef{}}
		var lastRecordedAt sql_NullTime
		var lastSpeed, lastLat, lastLon sql_NullFloat64
		var trackingActive bool
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.SpeedLimitKmh, &d.TripStopSpeedKmh,
			&d.TripStopDurationSeconds, &d.OfflineAfterSeconds, &trackingActive, &d.CreatedAt,
			&lastRecordedAt, &lastSpeed, &lastLat, &lastLon); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan error")
			return
		}
		d.TrackingActive = trackingActive
		d.Status = "unknown"
		if lastRecordedAt.Valid {
			ts := lastRecordedAt.Time.Format("2006-01-02T15:04:05Z07:00")
			d.LastRecordedAt = &ts
			d.Status = deriveStatus(lastRecordedAt, d.OfflineAfterSeconds, d.TripStopSpeedKmh, lastSpeed, trackingActive)
		} else if !trackingActive {
			d.Status = "paused"
		}
		if lastSpeed.Valid {
			d.LastSpeed = &lastSpeed.Float64
		}
		if lastLat.Valid {
			d.LastLat = &lastLat.Float64
		}
		if lastLon.Valid {
			d.LastLon = &lastLon.Float64
		}
		byID[d.ID] = d
		out = append(out, d)
	}

	if isAdmin {
		mrows, err := s.DB.QueryContext(r.Context(), `
			SELECT dou.device_id, o.id, o.name
			FROM device_orgs dou JOIN organizations o ON o.id = dou.org_id`)
		if err == nil {
			defer mrows.Close()
			for mrows.Next() {
				var did string
				var ref orgRef
				if err := mrows.Scan(&did, &ref.ID, &ref.Name); err == nil {
					if d, ok := byID[did]; ok {
						d.Orgs = append(d.Orgs, ref)
					}
				}
			}
		}
	}
	for _, d := range out {
		if isAdmin {
			d.Assigned = len(d.Orgs) > 0
		} else {
			d.Assigned = true
		}
	}

	writeJSON(w, http.StatusOK, out)
}

type setDeviceOrgsRequest struct {
	OrgIDs []string `json:"org_ids"`
}

// PUT /v1/devices/{id}/orgs — admin: replace a device's org membership set.
func (s *Server) SetDeviceOrgs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setDeviceOrgsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(r.Context(), `DELETE FROM device_orgs WHERE device_id = $1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not update assignment")
		return
	}
	for _, orgID := range req.OrgIDs {
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO device_orgs (device_id, org_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			id, orgID); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid org id")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type updateDeviceRequest struct {
	Name                    *string  `json:"name,omitempty"`
	SpeedLimitKmh           *float64 `json:"speed_limit_kmh,omitempty"`
	TripStopSpeedKmh        *float64 `json:"trip_stop_speed_kmh,omitempty"`
	TripStopDurationSeconds *int     `json:"trip_stop_duration_seconds,omitempty"`
}

// PATCH /v1/devices/{id} — admin edits any field; org renames only its own.
func (s *Server) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	isAdmin, _ := r.Context().Value(auth.CtxIsAdmin).(bool)
	orgID, _ := r.Context().Value(auth.CtxOrgID).(string)
	id := chi.URLParam(r, "id")

	var req updateDeviceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var res interface{ RowsAffected() (int64, error) }
	var err error
	if isAdmin {
		res, err = s.DB.ExecContext(r.Context(), `
			UPDATE devices SET
				name = COALESCE($1, name),
				speed_limit_kmh = COALESCE($2, speed_limit_kmh),
				trip_stop_speed_kmh = COALESCE($3, trip_stop_speed_kmh),
				trip_stop_duration_seconds = COALESCE($4, trip_stop_duration_seconds)
			WHERE id = $5`,
			req.Name, req.SpeedLimitKmh, req.TripStopSpeedKmh, req.TripStopDurationSeconds, id)
	} else {
		res, err = s.DB.ExecContext(r.Context(), `
			UPDATE devices SET name = COALESCE($1, name)
			WHERE id = $2 AND EXISTS (
				SELECT 1 FROM device_orgs WHERE device_id = $2 AND org_id = $3)`,
			req.Name, id, orgID)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not update device")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /v1/devices/{id} — admin deletes globally; org unassigns from itself.
func (s *Server) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	isAdmin, _ := r.Context().Value(auth.CtxIsAdmin).(bool)
	orgID, _ := r.Context().Value(auth.CtxOrgID).(string)
	id := chi.URLParam(r, "id")

	var res interface{ RowsAffected() (int64, error) }
	var err error
	if isAdmin {
		res, err = s.DB.ExecContext(r.Context(), `DELETE FROM devices WHERE id = $1`, id)
	} else {
		res, err = s.DB.ExecContext(r.Context(),
			`DELETE FROM device_orgs WHERE device_id = $1 AND org_id = $2`, id, orgID)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not remove device")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/devices/{id}/revoke-token — admin: rotate a device's token.
func (s *Server) RevokeDeviceToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	plaintext, err := auth.GenerateDeviceToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not generate device token")
		return
	}
	hash := auth.HashDeviceToken(plaintext)
	res, err := s.DB.ExecContext(r.Context(), `
		UPDATE devices SET api_token_hash = $1, api_token_revoked_at = NULL WHERE id = $2`, hash, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not rotate token")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"api_token": plaintext})
}
