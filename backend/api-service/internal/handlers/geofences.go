package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/trackproj/api-service/internal/auth"
)

type createGeofenceRequest struct {
	Name    string `json:"name"`
	GeoJSON string `json:"geojson"` // GeoJSON Polygon string
}

// POST /v1/geofences
func (s *Server) CreateGeofence(w http.ResponseWriter, r *http.Request) {
	orgID := r.Context().Value(auth.CtxOrgID).(string)
	userID, _ := r.Context().Value(auth.CtxUserID).(string)

	var req createGeofenceRequest
	if err := decodeJSON(r, &req); err != nil || req.Name == "" || req.GeoJSON == "" {
		writeErr(w, http.StatusBadRequest, "name and geojson are required")
		return
	}

	var id, createdAt string
	err := s.DB.QueryRowContext(r.Context(), `
		INSERT INTO geofences (org_id, name, geom)
		VALUES ($1, $2, ST_GeomFromGeoJSON($3)::geography)
		RETURNING id, created_at`,
		orgID, req.Name, req.GeoJSON,
	).Scan(&id, &createdAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create geofence: "+err.Error())
		return
	}

	s.logActivity(r, orgID, userID, "geofence.created", "geofence", id)
	writeJSON(w, http.StatusCreated, map[string]string{
		"id": id, "org_id": orgID, "name": req.Name, "created_at": createdAt,
	})
}

// GET /v1/geofences — list all geofences for the org, returning GeoJSON.
func (s *Server) ListGeofences(w http.ResponseWriter, r *http.Request) {
	orgID := r.Context().Value(auth.CtxOrgID).(string)

	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT id, name, ST_AsGeoJSON(geom::geometry), created_at
		FROM geofences WHERE org_id = $1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list geofences")
		return
	}
	defer rows.Close()

	type gfOut struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		GeoJSON   string `json:"geojson"`
		CreatedAt string `json:"created_at"`
	}
	out := []gfOut{}
	for rows.Next() {
		var g gfOut
		if err := rows.Scan(&g.ID, &g.Name, &g.GeoJSON, &g.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan error")
			return
		}
		out = append(out, g)
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /v1/geofences/{id}
func (s *Server) DeleteGeofence(w http.ResponseWriter, r *http.Request) {
	orgID := r.Context().Value(auth.CtxOrgID).(string)
	userID, _ := r.Context().Value(auth.CtxUserID).(string)
	id := chi.URLParam(r, "id")

	res, err := s.DB.ExecContext(r.Context(),
		`DELETE FROM geofences WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not delete geofence")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, http.StatusNotFound, "geofence not found")
		return
	}
	s.logActivity(r, orgID, userID, "geofence.deleted", "geofence", id)
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/geofences/{id}/events — recent enter/exit events for a geofence.
func (s *Server) ListGeofenceEvents(w http.ResponseWriter, r *http.Request) {
	orgID := r.Context().Value(auth.CtxOrgID).(string)
	gfID := chi.URLParam(r, "id")

	// Confirm ownership.
	var check string
	if err := s.DB.QueryRowContext(r.Context(),
		`SELECT id FROM geofences WHERE id = $1 AND org_id = $2`, gfID, orgID,
	).Scan(&check); err != nil {
		writeErr(w, http.StatusNotFound, "geofence not found")
		return
	}

	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT e.id, e.device_id, d.name, e.type, e.occurred_at
		FROM geofence_events e
		JOIN devices d ON d.id = e.device_id
		WHERE e.geofence_id = $1
		ORDER BY e.occurred_at DESC
		LIMIT 200`, gfID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list events")
		return
	}
	defer rows.Close()

	type evOut struct {
		ID         int64  `json:"id"`
		DeviceID   string `json:"device_id"`
		DeviceName string `json:"device_name"`
		Type       string `json:"type"`
		OccurredAt string `json:"occurred_at"`
	}
	out := []evOut{}
	for rows.Next() {
		var e evOut
		if err := rows.Scan(&e.ID, &e.DeviceID, &e.DeviceName, &e.Type, &e.OccurredAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan error")
			return
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}
