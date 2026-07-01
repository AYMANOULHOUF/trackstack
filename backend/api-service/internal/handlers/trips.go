package handlers

import (
	"fmt"
	"net/http"

	"github.com/trackproj/api-service/internal/auth"
)

// GET /v1/trips?device_id=&from=&to=&limit=
func (s *Server) ListTrips(w http.ResponseWriter, r *http.Request) {
	orgID := r.Context().Value(auth.CtxOrgID).(string)
	q := r.URL.Query()
	deviceID := q.Get("device_id")
	if deviceID == "" {
		writeErr(w, http.StatusBadRequest, "device_id is required")
		return
	}

	// Confirm ownership.
	var check string
	if err := s.DB.QueryRowContext(r.Context(),
		`SELECT id FROM devices WHERE id = $1 AND org_id = $2`, deviceID, orgID,
	).Scan(&check); err != nil {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}

	args := []interface{}{deviceID}
	where := "device_id = $1"
	n := 2
	if from := q.Get("from"); from != "" {
		where += fmt.Sprintf(" AND start_at >= $%d", n)
		args = append(args, from)
		n++
	}
	if to := q.Get("to"); to != "" {
		where += fmt.Sprintf(" AND start_at <= $%d", n)
		args = append(args, to)
	}

	rows, err := s.DB.QueryContext(r.Context(), fmt.Sprintf(`
		SELECT id, device_id, start_at, end_at, distance_m
		FROM trips WHERE %s ORDER BY start_at DESC LIMIT 200`, where), args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list trips")
		return
	}
	defer rows.Close()

	type tripOut struct {
		ID        int64    `json:"id"`
		DeviceID  string   `json:"device_id"`
		StartAt   string   `json:"start_at"`
		EndAt     *string  `json:"end_at,omitempty"`
		DistanceM *float64 `json:"distance_m,omitempty"`
	}
	out := []tripOut{}
	for rows.Next() {
		var t tripOut
		var endAt sql_NullTime
		var distM sql_NullFloat64
		if err := rows.Scan(&t.ID, &t.DeviceID, &t.StartAt, &endAt, &distM); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan error")
			return
		}
		if endAt.Valid {
			s := endAt.Time.Format("2006-01-02T15:04:05Z")
			t.EndAt = &s
		}
		if distM.Valid {
			t.DistanceM = &distM.Float64
		}
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/activity?limit= — org-scoped audit log (Step 16 item 11).
func (s *Server) ListActivity(w http.ResponseWriter, r *http.Request) {
	orgID := r.Context().Value(auth.CtxOrgID).(string)

	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT id, actor_user_id, action, target_type, target_id, occurred_at
		FROM activity_logs
		WHERE org_id = $1
		ORDER BY occurred_at DESC
		LIMIT 500`, orgID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list activity")
		return
	}
	defer rows.Close()

	type logOut struct {
		ID          int64   `json:"id"`
		ActorUserID *string `json:"actor_user_id,omitempty"`
		Action      string  `json:"action"`
		TargetType  string  `json:"target_type,omitempty"`
		TargetID    string  `json:"target_id,omitempty"`
		OccurredAt  string  `json:"occurred_at"`
	}
	out := []logOut{}
	for rows.Next() {
		var l logOut
		var actorID sql_NullString
		var tt, tid sql_NullString
		if err := rows.Scan(&l.ID, &actorID, &l.Action, &tt, &tid, &l.OccurredAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan error")
			return
		}
		if actorID.Valid {
			l.ActorUserID = &actorID.String
		}
		if tt.Valid {
			l.TargetType = tt.String
		}
		if tid.Valid {
			l.TargetID = tid.String
		}
		out = append(out, l)
	}
	writeJSON(w, http.StatusOK, out)
}

type sql_NullString struct {
	String string
	Valid  bool
}

func (n *sql_NullString) Scan(src interface{}) error {
	if src == nil {
		n.Valid = false
		return nil
	}
	s, ok := src.(string)
	if !ok {
		n.Valid = false
		return nil
	}
	n.String = s
	n.Valid = true
	return nil
}
