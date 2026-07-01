package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/trackproj/api-service/internal/auth"
)

// GET /v1/orgs — admin: list every organization with its login email and
// how many devices are assigned to it.
func (s *Server) ListOrgs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT o.id, o.name, o.created_at,
		       COALESCE((SELECT email FROM users WHERE org_id = o.id ORDER BY created_at LIMIT 1), ''),
		       (SELECT count(*) FROM device_orgs WHERE org_id = o.id)
		FROM organizations o
		ORDER BY o.created_at DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list organizations")
		return
	}
	defer rows.Close()

	type orgOut struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		CreatedAt   string `json:"created_at"`
		Email       string `json:"email"`
		DeviceCount int    `json:"device_count"`
	}
	out := []orgOut{}
	for rows.Next() {
		var o orgOut
		if err := rows.Scan(&o.ID, &o.Name, &o.CreatedAt, &o.Email, &o.DeviceCount); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan error")
			return
		}
		out = append(out, o)
	}
	writeJSON(w, http.StatusOK, out)
}

type createOrgRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// POST /v1/orgs — admin: create an organization plus its single login user.
func (s *Server) CreateOrg(w http.ResponseWriter, r *http.Request) {
	var req createOrgRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Email == "" || len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "name, email required; password must be 8+ chars")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback()

	var orgID, createdAt string
	if err := tx.QueryRowContext(r.Context(),
		`INSERT INTO organizations (name) VALUES ($1) RETURNING id, created_at`, req.Name,
	).Scan(&orgID, &createdAt); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create organization")
		return
	}
	if _, err := tx.ExecContext(r.Context(),
		`INSERT INTO users (org_id, email, password_hash, role, is_admin) VALUES ($1, $2, $3, 'admin', false)`,
		orgID, req.Email, hash,
	); err != nil {
		writeErr(w, http.StatusConflict, "email already in use, or db error")
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id": orgID, "name": req.Name, "email": req.Email, "created_at": createdAt, "device_count": 0,
	})
}

type updateOrgRequest struct {
	Name     *string `json:"name,omitempty"`
	Email    *string `json:"email,omitempty"`
	Password *string `json:"password,omitempty"`
}

// PATCH /v1/orgs/{id} — admin: rename org and/or change its login email/password.
func (s *Server) UpdateOrg(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateOrgRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		if _, err := s.DB.ExecContext(r.Context(),
			`UPDATE organizations SET name = $1 WHERE id = $2`, *req.Name, id); err != nil {
			writeErr(w, http.StatusInternalServerError, "could not update org")
			return
		}
	}
	if req.Email != nil {
		if _, err := s.DB.ExecContext(r.Context(),
			`UPDATE users SET email = $1 WHERE org_id = $2`, *req.Email, id); err != nil {
			writeErr(w, http.StatusConflict, "email already in use")
			return
		}
	}
	if req.Password != nil {
		if len(*req.Password) < 8 {
			writeErr(w, http.StatusBadRequest, "password must be 8+ chars")
			return
		}
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "could not hash password")
			return
		}
		if _, err := s.DB.ExecContext(r.Context(),
			`UPDATE users SET password_hash = $1 WHERE org_id = $2`, hash, id); err != nil {
			writeErr(w, http.StatusInternalServerError, "could not update password")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /v1/orgs/{id} — admin: delete org (cascades its users + device
// memberships; devices themselves survive and become unassigned).
func (s *Server) DeleteOrg(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := s.DB.ExecContext(r.Context(), `DELETE FROM organizations WHERE id = $1`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not delete org")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, http.StatusNotFound, "org not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
