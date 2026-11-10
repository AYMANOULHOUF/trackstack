package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/trackproj/api-service/internal/auth"
)

type notificationSetting struct {
	Channel string `json:"channel"`
	Enabled bool   `json:"enabled"`
	Target  string `json:"target,omitempty"`
}

type notificationsResponse struct {
	Settings       []notificationSetting `json:"settings"`
	VAPIDPublicKey string                `json:"vapid_public_key,omitempty"`
}

var notificationChannels = []string{"email", "web_push"}

// GET /v1/notifications — org-wide delivery settings, with every supported
// channel present even when it has never been configured.
func (s *Server) GetNotifications(w http.ResponseWriter, r *http.Request) {
	orgID, ok := orgFromContext(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "organization account required")
		return
	}

	byChannel := map[string]notificationSetting{}
	for _, ch := range notificationChannels {
		byChannel[ch] = notificationSetting{Channel: ch}
	}

	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT channel, enabled, COALESCE(target, '')
		FROM notification_settings
		WHERE org_id = $1 AND user_id IS NULL`, orgID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load notification settings")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var s notificationSetting
		if err := rows.Scan(&s.Channel, &s.Enabled, &s.Target); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan error")
			return
		}
		if _, supported := byChannel[s.Channel]; supported {
			byChannel[s.Channel] = s
		}
	}

	out := notificationsResponse{Settings: []notificationSetting{}}
	for _, ch := range notificationChannels {
		out.Settings = append(out.Settings, byChannel[ch])
	}
	if s.Notifier != nil {
		out.VAPIDPublicKey = s.Notifier.VAPIDPublicKey()
	}
	writeJSON(w, http.StatusOK, out)
}

type putNotificationsRequest struct {
	Settings []notificationSetting `json:"settings"`
}

// PUT /v1/notifications — replace the org-wide settings for the supplied
// channels. Channels not present in the request are left untouched.
func (s *Server) PutNotifications(w http.ResponseWriter, r *http.Request) {
	orgID, ok := orgFromContext(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "organization account required")
		return
	}

	var req putNotificationsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Settings) == 0 {
		writeErr(w, http.StatusBadRequest, "settings is required")
		return
	}

	for _, setting := range req.Settings {
		if setting.Channel != "email" && setting.Channel != "web_push" {
			writeErr(w, http.StatusBadRequest, "unsupported channel: "+setting.Channel)
			return
		}
		if setting.Enabled && strings.TrimSpace(setting.Target) == "" {
			writeErr(w, http.StatusBadRequest, setting.Channel+" requires a target")
			return
		}
		if setting.Channel == "email" && setting.Enabled && !strings.Contains(setting.Target, "@") {
			writeErr(w, http.StatusBadRequest, "email target must be an address")
			return
		}
		if setting.Channel == "web_push" && setting.Enabled && !json.Valid([]byte(setting.Target)) {
			writeErr(w, http.StatusBadRequest, "web_push target must be a subscription JSON object")
			return
		}

		target := strings.TrimSpace(setting.Target)
		res, err := s.DB.ExecContext(r.Context(), `
			UPDATE notification_settings
			SET enabled = $1, target = $2
			WHERE org_id = $3 AND user_id IS NULL AND channel = $4`,
			setting.Enabled, target, orgID, setting.Channel)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "could not save notification settings")
			return
		}
		if n, _ := res.RowsAffected(); n > 0 {
			continue
		}
		if _, err := s.DB.ExecContext(r.Context(), `
			INSERT INTO notification_settings (org_id, user_id, channel, enabled, target)
			VALUES ($1, NULL, $2, $3, $4)`,
			orgID, setting.Channel, setting.Enabled, target); err != nil {
			writeErr(w, http.StatusInternalServerError, "could not save notification settings")
			return
		}
	}

	s.logActivity(r, orgID, r.Context().Value(auth.CtxUserID).(string), "notifications.updated", "notification_settings", "")
	s.GetNotifications(w, r)
}

func orgFromContext(r *http.Request) (string, bool) {
	orgID, _ := r.Context().Value(auth.CtxOrgID).(string)
	if orgID == "" {
		return "", false
	}
	return orgID, true
}
