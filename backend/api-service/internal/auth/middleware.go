package auth

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

type ctxKey string

const (
	CtxUserID   ctxKey = "user_id"
	CtxOrgID    ctxKey = "org_id"
	CtxIsAdmin  ctxKey = "is_admin"
	CtxDeviceID ctxKey = "device_id"
)

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer "), true
	}
	// WebSocket clients can't set custom headers, so allow token via query param.
	if tok := r.URL.Query().Get("token"); tok != "" {
		return tok, true
	}
	return "", false
}

// RequireJWT authenticates dashboard requests. On success it injects
// user_id / org_id / is_admin into the request context. The global admin
// has an empty org_id and is_admin = true.
func RequireJWT(issuer *JWTIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := bearerToken(r)
			if !ok {
				http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
				return
			}
			claims, err := issuer.Parse(tok)
			if err != nil || claims.TokenType != "access" {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), CtxUserID, claims.UserID)
			ctx = context.WithValue(ctx, CtxOrgID, claims.OrgID)
			ctx = context.WithValue(ctx, CtxIsAdmin, claims.IsAdmin)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin gates admin-only routes; must run after RequireJWT.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isAdmin, _ := r.Context().Value(CtxIsAdmin).(bool)
		if !isAdmin {
			http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireDeviceToken authenticates phone/hardware uploads against
// devices.api_token_hash (long-lived per-device tokens).
func RequireDeviceToken(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := bearerToken(r)
			if !ok {
				http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
				return
			}
			hash := HashDeviceToken(tok)

			var deviceID string
			err := db.QueryRowContext(r.Context(),
				`SELECT id FROM devices WHERE api_token_hash = $1 AND api_token_revoked_at IS NULL`,
				hash,
			).Scan(&deviceID)
			if err != nil {
				http.Error(w, `{"error":"invalid or revoked device token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), CtxDeviceID, deviceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
