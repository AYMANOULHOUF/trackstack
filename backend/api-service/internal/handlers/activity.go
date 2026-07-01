package handlers

import (
	"log"
	"net/http"
)

// logActivity inserts a row into activity_logs. Errors are logged but
// not fatal — a failed audit entry should not roll back the main operation
// (Step 16 item 11: audit trail, best-effort for non-critical side-effects).
func (s *Server) logActivity(r *http.Request, orgID, actorUserID, action, targetType, targetID string) {
	var actorParam interface{} = actorUserID
	if actorUserID == "" {
		actorParam = nil
	}
	_, err := s.DB.ExecContext(r.Context(), `
		INSERT INTO activity_logs (org_id, actor_user_id, action, target_type, target_id)
		VALUES ($1, $2, $3, $4, $5)`,
		orgID, actorParam, action, targetType, targetID,
	)
	if err != nil {
		log.Printf("activity_log insert failed (non-fatal): action=%s target=%s/%s: %v",
			action, targetType, targetID, err)
	}
}
