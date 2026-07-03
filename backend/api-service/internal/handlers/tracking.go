package handlers

import (
	"net/http"

	"github.com/trackproj/api-service/internal/auth"
)

type trackingStateRequest struct {
	Active bool `json:"active"`
}

// POST /v1/tracking-state — device-token authenticated. Phone reports whether
// it is currently tracking. Used so the dashboard can show "Paused" instead
// of the generic "Stationary" status when a user manually stops tracking.
func (s *Server) SetTrackingState(w http.ResponseWriter, r *http.Request) {
	deviceID := r.Context().Value(auth.CtxDeviceID).(string)
	var req trackingStateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, err := s.DB.ExecContext(r.Context(),
		`UPDATE devices SET tracking_active = $1 WHERE id = $2`, req.Active, deviceID); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not update tracking state")
		return
	}

	// Broadcast state change so live-connected clients see it immediately.
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
		"type":            "tracking_state",
		"device_id":       deviceID,
		"tracking_active": req.Active,
	}
	for _, org := range memberOrgs {
		s.broadcast(org, payload)
	}
	s.broadcast(AdminChannel, payload)

	w.WriteHeader(http.StatusNoContent)
}
