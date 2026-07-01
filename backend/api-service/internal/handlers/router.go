package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/trackproj/api-service/internal/auth"
)

func NewRouter(s *Server) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	// --- Public ---
	r.Post("/v1/auth/login", s.Login)
	r.Post("/v1/auth/refresh", s.Refresh)
	// Freshly installed phones self-enroll here → created as an unassigned
	// (pending) device that only the admin can see until assigned to an org.
	r.Post("/v1/enroll", s.EnrollDevice)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// --- Device token auth (phone / hardware) ---
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireDeviceToken(s.DB))
		r.Post("/v1/positions", s.IngestPosition)
	})

	// --- JWT auth (dashboard: admin + org) ---
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireJWT(s.JWT))

		r.Get("/v1/live", s.LiveWS)

		// Devices — listing + edits are role-scoped inside the handlers.
		r.Get("/v1/devices", s.ListDevices)
		r.Patch("/v1/devices/{id}", s.UpdateDevice) // rename (org: own devices, admin: any)
		r.Delete("/v1/devices/{id}", s.DeleteDevice) // org: unassign from own org; admin: delete globally

		r.Get("/v1/positions", s.ListPositions)
		r.Get("/v1/geofences", s.ListGeofences)
		r.Post("/v1/geofences", s.CreateGeofence)
		r.Delete("/v1/geofences/{id}", s.DeleteGeofence)
		r.Get("/v1/geofences/{id}/events", s.ListGeofenceEvents)
		r.Get("/v1/trips", s.ListTrips)
		r.Get("/v1/activity", s.ListActivity)

		// --- Admin only ---
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin)

			r.Get("/v1/orgs", s.ListOrgs)
			r.Post("/v1/orgs", s.CreateOrg)
			r.Patch("/v1/orgs/{id}", s.UpdateOrg)
			r.Delete("/v1/orgs/{id}", s.DeleteOrg)

			// Assign a device to zero or more orgs (replaces its membership set).
			r.Put("/v1/devices/{id}/orgs", s.SetDeviceOrgs)
			r.Post("/v1/devices/{id}/revoke-token", s.RevokeDeviceToken)
		})
	})

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
