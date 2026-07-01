package handlers

import (
	"net/http"

	"github.com/trackproj/api-service/internal/auth"
)

// AdminChannel is the hub key the global admin subscribes to; every position
// is also fanned out here so the admin sees all devices across all orgs.
const AdminChannel = "__admin__"

// GET /v1/live — org users get their org channel; the admin gets the firehose.
func (s *Server) LiveWS(w http.ResponseWriter, r *http.Request) {
	isAdmin, _ := r.Context().Value(auth.CtxIsAdmin).(bool)
	orgID, _ := r.Context().Value(auth.CtxOrgID).(string)
	channel := orgID
	if isAdmin {
		channel = AdminChannel
	}
	s.Hub.ServeWS(w, r, channel)
}
