package handlers

import (
	"database/sql"
	"net/http"

	"github.com/trackproj/api-service/internal/auth"
)

type authResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Role         string `json:"role"` // "admin" | "org"
	OrgID        string `json:"org_id,omitempty"`
	UserID       string `json:"user_id"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// POST /v1/auth/login — one screen for both admin and org users. The response
// role tells the dashboard where to route. Registration is disabled: the admin
// is seeded from env, org accounts are created by the admin.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var userID, passwordHash string
	var orgID sql.NullString
	var isAdmin bool
	err := s.DB.QueryRowContext(r.Context(),
		`SELECT id, org_id, is_admin, password_hash FROM users WHERE email = $1`, req.Email,
	).Scan(&userID, &orgID, &isAdmin, &passwordHash)
	if err != nil || !auth.CheckPassword(passwordHash, req.Password) {
		writeErr(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	role := "org"
	if isAdmin {
		role = "admin"
	}
	access, _ := s.JWT.IssueAccessToken(userID, orgID.String, isAdmin)
	refresh, _ := s.JWT.IssueRefreshToken(userID, orgID.String, isAdmin)
	writeJSON(w, http.StatusOK, authResponse{
		AccessToken: access, RefreshToken: refresh, Role: role, OrgID: orgID.String, UserID: userID,
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// GET /v1/me - identity of the caller, derived from the access token. The
// dashboard uses it to validate a stored session and to choose the admin or
// organization shell; the role never has to be trusted from the browser.
func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(auth.CtxUserID).(string)
	orgID, _ := r.Context().Value(auth.CtxOrgID).(string)
	isAdmin, _ := r.Context().Value(auth.CtxIsAdmin).(bool)

	role := "org"
	if isAdmin {
		role = "admin"
	}
	resp := map[string]interface{}{
		"user_id":  userID,
		"role":     role,
		"is_admin": isAdmin,
	}
	if orgID != "" {
		resp["org_id"] = orgID
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /v1/auth/refresh - exchange a refresh token for a new access token.
func (s *Server) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims, err := s.JWT.Parse(req.RefreshToken)
	if err != nil || claims.TokenType != "refresh" {
		writeErr(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	access, _ := s.JWT.IssueAccessToken(claims.UserID, claims.OrgID, claims.IsAdmin)
	writeJSON(w, http.StatusOK, map[string]string{"access_token": access})
}
