# Project: Phone/GPS Tracking Platform — Decision Log

General-purpose, self-hostable location tracking platform. Usable by companies (fleet)
and self-hosting individuals (own devices). Android app is visible (icon, normal
permission prompts, persistent notification while tracking). Designed to later extend
to raw GPS hardware protocols (Traccar-style protocol gateway).

Each entry below = one decision, with the reasoning and the step it was made at.

---

## Step 1 — Scope & legitimacy boundary
- **Decision:** Visible app only, normal Android permission flow, no hiding from
  launcher/app list, no covert/stalkerware behavior.
- **Why:** User confirmed general use (companies + self-hosters), visible app with
  standard permissions. This rules out covert tracking architecture entirely.

## Step 2 — Android background tracking limits (researched)
- Foreground service + persistent notification required for any live location service
  (Android 8+ requirement).
- `ACCESS_BACKGROUND_LOCATION` is a separate runtime permission since Android 10,
  needs prominent in-app disclosure; Play Store requires a policy declaration if
  published there.
- WorkManager viable for periodic/batched location pings (more battery-friendly than
  a constant foreground service).
- Geofencing API available for enter/exit triggers.
- App cannot track after user force-stops it; cannot survive uninstall; cannot hide
  from app list (also out of scope per Step 1).
- Fleet/company-owned devices could later use Android Enterprise (Device Owner /
  Android Management API) for stronger guarantees — noted as a possible future mode,
  not yet decided.

## Step 3 — Backend architecture shape
- **Decision (proposed, pending confirmation):** Two services — API service (REST +
  WebSocket, phone app facing) and a separate Protocol Gateway service (raw TCP/UDP
  listeners for future GPS hardware protocols), sharing one PostgreSQL+PostGIS
  datastore.
- **Why:** Phones speak HTTP/WebSocket; dumb GPS trackers speak raw binary/text
  protocols over TCP/UDP (e.g. GT06, Teltonika, H02). Splitting now avoids a rewrite
  later. Mirrors the architecture used by the open-source project Traccar, noted as a
  possible reference/gateway rather than a from-scratch build.

## Step 4 — Datastore
- **Decision:** PostgreSQL with PostGIS extension.
- **Why:** Geospatial queries (geofence containment, nearest-device, route/track
  history) are native and efficient in PostGIS vs. plain lat/lng columns.

## Step 5 — Transport for phone uploads
- **Decision (proposed):** HTTPS REST for control/config, optional MQTT (e.g.
  Mosquitto) for frequent location pings.
- **Why:** MQTT is lighter on battery/bandwidth than polling HTTPS, and many GPS
  hardware units natively speak MQTT, keeping the future hardware path consistent.

## Step 6 — Deployment
- **Decision:** Docker/Podman containers, composed via docker-compose.yml (and
  Podman-compatible), reverse proxy: Caddy for TLS + routing.
- **Why:** User's explicit requirement — must run as containers, self-hostable.
  Caddy chosen over Traefik/nginx for automatic HTTPS (built-in Let's Encrypt,
  zero-config certs) and a much simpler Caddyfile vs. Traefik's label-heavy or
  nginx's manual-cert config — a good fit for self-hosters who don't want to manage
  certs by hand.

## Step 7 — Language/runtime
- **Decision:** Go for all backend services (API service + protocol gateway).
- **Why:** User chose Go — single static binary, strong concurrency for many
  simultaneous device sockets, low memory footprint, good fit for self-hosted small
  boxes/RPi/VPS.

## Step 8 — Build-from-scratch vs. reference existing OSS (Traccar)
- **Decision:** Defer the choice; design the Protocol Gateway service as a
  swappable/pluggable component so either path stays open.
- **Why:** User wants to decide later. Architecture impact: gateway exposes a stable
  internal interface (decoded event -> common schema -> write to Postgres/MQTT) so it
  can be (a) hand-built per protocol in Go, or (b) replaced/fronted by Traccar later,
  without touching the API service or DB schema.

## Step 9 — Repo / service layout (proposed)
- **Decision (proposed):**
  ```
  /backend
    /api-service        (Go, REST + WebSocket, phone-facing, auth, device mgmt)
    /protocol-gateway    (Go, pluggable per-protocol TCP/UDP listeners, empty/stub
                           for now beyond a "phone" virtual protocol)
    /migrations          (SQL, Postgres+PostGIS schema)
    docker-compose.yml    (postgres+postgis, mosquitto, api-service, protocol-gateway,
                           reverse proxy)
  /android
    (Kotlin app — see Android steps)
  read_me.md
  ```
- **Why:** Keeps the two backend services independently buildable/deployable as
  separate containers per Step 3/Step 6, while sharing one DB schema and one repo for
  now (can split repos later if needed).

## Step 10 — Core data model (proposed, not yet finalized)
- `devices` (id, owner_id/org_id, name, type: phone|hardware, created_at)
- `users` / `organizations` (for multi-tenant: a company self-hosting for its fleet,
  or an individual self-hosting for personal devices)
- `positions` (device_id, geom (PostGIS point), speed, heading, accuracy, battery,
  recorded_at, received_at) — append-only, indexed by device_id + time, and a GIST
  index on geom for spatial queries
- `geofences` (org_id, name, geom (PostGIS polygon))
- `geofence_events` (device_id, geofence_id, type: enter|exit, occurred_at)
- **Why:** Minimal schema covering both phone app and future hardware devices under
  the same `devices`/`positions` tables (protocol gateway just writes into the same
  tables via the common internal event schema from Step 8).

## Step 11 — Map tiles
- **Decision:** OpenStreetMap raw tiles (no API key).
- **Why:** User chose OSM raw tiles. Free, no key/account needed, fits the
  self-hosted/no-external-dependency ethos. Note for later: OSM's tile usage policy
  caps high-volume automated use, so if traffic grows beyond light/medium self-hosted
  use, the project may want to switch to self-hosted tiles (e.g. via a tileserver
  container) down the line — flagged here, not acted on yet.
- **Implementation note:** Dashboard -> Leaflet.js (lightweight, works directly with
  raw OSM tile URLs, no extra server needed). Android -> osmdroid (OSM raster tiles,
  free, no API key, drop-in for Android).

## Step 12 — Auth model
- **Decision:** JWT for dashboard/user sessions; separate long-lived API tokens for
  devices (phones and, later, hardware).
- **Why:** User chose this split. Devices shouldn't go through interactive
  login/refresh flows — a long-lived per-device token (issued once at registration,
  revocable individually) is simpler and safer for unattended background uploads than
  forcing JWT refresh cycles onto a phone or a dumb GPS unit. Dashboard users keep
  normal JWT (with refresh) for session-based login/logout.

## Step 13 — Android tracking architecture
- **Decision:** Foreground service for continuous, live tracking with a persistent
  notification.
- **Why:** User chose continuous live mode. Matches Step 1/Step 2 — visible app,
  normal permissions, persistent notification is mandatory for this anyway under
  Android 8+ for any location-emitting foreground service, so this is the most
  battery-honest and policy-compliant choice (no attempt to hide tracking activity
  from the user, which also wasn't desired per Step 1).

## Step 14 — Build order
- **Decision:** API service + Android app first; web dashboard afterward.
- **Why:** User asked for the best call given the plan. The dashboard (Leaflet + OSM
  tiles, Step 11) is just a consumer of the API's REST/WebSocket endpoints and the
  `positions`/`devices` data — building it before the API exists would mean mocking
  data and redoing it later. Building API -> Android app first means there's a real
  device producing real location data to view by the time the dashboard is built,
  making the dashboard buildable in one pass against real, working endpoints instead
  of guesses.

## Step 15 — API service endpoint list & schema (proposed, ready to implement)
- **Decision:** Concrete v1 surface below, building directly on the Step 10 schema
  and Step 12 auth split.

  **Auth / users (JWT)**
  - `POST /v1/auth/register` — create org + first admin user
  - `POST /v1/auth/login` — returns JWT access + refresh token
  - `POST /v1/auth/refresh`

  **Device management (JWT, dashboard-side)**
  - `POST /v1/devices` — register a device, returns the device's long-lived API token
    (shown once, hashed at rest)
  - `GET /v1/devices` — list org's devices
  - `DELETE /v1/devices/{id}` — also revokes its token
  - `POST /v1/devices/{id}/revoke-token` — rotate without deleting device

  **Position ingest (device API token)**
  - `POST /v1/positions` — single or batched array of points (lat, lon, speed,
    heading, accuracy, battery, recorded_at) — used by both the Android app and,
    later, the protocol gateway's internal writer
  - `GET /v1/devices/{id}/positions?from&to` — history query (JWT)
  - `WS /v1/live` — WebSocket, JWT-authenticated, streams new positions for the org's
    devices as they arrive (dashboard live view)

  **Geofences (JWT)**
  - `POST /v1/geofences`, `GET /v1/geofences`, `DELETE /v1/geofences/{id}`
  - geofence enter/exit detection done server-side on position ingest, written to
    `geofence_events`, also pushed over the `/v1/live` WebSocket

- **Why:** Keeps the device-facing surface (`POST /v1/positions`, token-authenticated)
  completely separate from the dashboard-facing surface (JWT), matching Step 12.
  `POST /v1/positions` is intentionally protocol-agnostic (just points in, regardless
  of source) so the future protocol-gateway (Step 8/9) can write to the exact same
  endpoint/internal function instead of needing its own ingest path.

## Step 16 — Feature list (user-specified) & architectural implications
- **Decision:** Confirmed feature set below, each mapped to what it requires in the
  existing plan.

  1. **pgAdmin4 for pulling user data** — add `pgadmin4` as another container in
     `docker-compose.yml`, pointed at the same Postgres instance. Admin-only, not
     exposed publicly by default (internal network / Caddy basic-auth gate or simply
     not routed through the public Caddyfile).
  2. **Map with all connected devices** — dashboard feature, consumes `GET
     /v1/devices` + `WS /v1/live` (Step 15), rendered with Leaflet + OSM tiles
     (Step 11).
  3. **Per-vehicle dynamic speed limit, changeable anytime, with notification +
     logging on violation** — schema addition: `devices.speed_limit_kmh` (nullable,
     editable via dashboard/API anytime). Ingest-time check on `POST /v1/positions`:
     if `position.speed > device.speed_limit_kmh`, write a `speed_events` row and
     push a notification over `/v1/live` (and later a dedicated notification
     channel — see Step 18 placeholder). New table: `speed_events` (device_id,
     speed, limit_at_time, occurred_at).
  4. **iOS expansion later** — confirms the API/auth design (token-based device
     auth, protocol-agnostic `/v1/positions`) already supports this; no app-specific
     coupling exists in the backend, so a future iOS app is just another client.
     No action now, design stays compatible by default.
  5. **Support physical GPS trackers/protocols, reusing code from established open
     protocol implementations** — confirms Step 8/9 (pluggable `protocol-gateway`).
     Concretely: vendor in well-tested protocol parsers (e.g. ports/adaptations of
     Traccar's individual protocol decoders, which are organized per-protocol and
     mostly self-contained) rather than re-implementing binary protocols from raw
     spec docs — lower risk of subtle parsing bugs. Each ported protocol decoder
     outputs to the same common internal event struct (Step 8) feeding the same
     `/v1/positions`-equivalent internal write path.
  6. **Log last known position before a device disconnects** — already covered by
     the append-only `positions` table (Step 10); "last position" = latest row per
     device, no extra table needed, just an indexed query (`MAX(recorded_at)` per
     `device_id`) or a `devices.last_position_id` denormalized pointer for fast
     dashboard lookups.
  7. **Vehicle/device status: moving / stopped / offline** — derived, not stored
     directly: "offline" = no position received within a configurable timeout
     (e.g. device-level `offline_after_seconds`); "moving" vs "stopped" = derived
     from latest reported speed (and/or distance delta between last two points)
     against a small threshold. Computed at query/display time, not written to DB,
     to avoid stale-status bugs.
  8. **Trip history** — new table `trips` (device_id, start_at, end_at, start_geom,
     end_geom, distance_m), built by a background job in `api-service` that
     segments a device's `positions` into trips (a trip starts on movement after a
     stop period, ends on a stop period exceeding a threshold).
  9. **Route playback** — dashboard feature: given a `trip_id` or a time range, pull
     ordered `positions` rows and animate marker movement along the route on the
     Leaflet map. No new backend endpoint beyond the existing `GET
     /v1/devices/{id}/positions?from&to` (Step 15).
  10. **Each tracked device can have a name** — already in schema (`devices.name`,
      Step 10), editable via dashboard/API, no new work.
  11. **Activity logs** — new table `activity_logs` (org_id, actor_user_id,
      action, target_type, target_id, occurred_at) — audit trail for dashboard
      actions (device added/removed/renamed, speed limit changed, geofence
      edited, user login, token rotated, etc.).
  12. **Dark mode** — frontend-only (dashboard), CSS/theme toggle, no backend impact.
  13. **Add/manage/remove devices** — already covered by `POST/GET/DELETE
      /v1/devices` (Step 15); "manage" extended here to include renaming and
      setting the new `speed_limit_kmh` field from item 3.
  14. **Web responsive dashboard** — confirms Step 14 (dashboard built after API +
      Android app); implementation note: plain responsive CSS (flex/grid) rather
      than a heavy component framework, to keep the self-hosted footprint light —
      open decision on exact frontend framework, not yet made.

- **Why:** Folding all of this in now (rather than after scaffolding) means the DB
  schema (Step 10) and API surface (Step 15) get extended once, with the new tables
  (`speed_events`, `trips`, `activity_logs`) and column (`speed_limit_kmh`,
  `offline_after_seconds`) included from the first migration instead of bolted on
  via a second migration round.

## Step 17 — Dashboard frontend framework
- **Decision:** React.
- **Why:** User chose React over plain JS/Svelte/Vue, trading the lighter footprint
  of the alternatives for the largest ecosystem of free admin-dashboard
  templates/components, libraries (map wrappers, charts for speed/trip data, dark
  mode toggles), and easiest long-term hiring/contributor pool if this grows beyond
  a solo project.
- **Implication going forward:** dashboard ships as its own container — static
  build (Vite + React) served by Caddy, or its own lightweight Node/static server —
  not a Go-templated server-rendered page. Will need a small build pipeline
  (`npm run build` -> static assets) added to the Docker setup.

## Step 18 — Speed violation notification delivery
- **Decision:** All channels, configurable — in-dashboard (WebSocket), email (SMTP),
  and Web Push (browser), with per-org/per-user settings for which are enabled.
- **Why:** User chose maximum flexibility. Implications: new table
  `notification_settings` (org_id or user_id, channel, enabled, target e.g. email
  address or push subscription); requires an SMTP client config (self-hosters supply
  their own relay credentials via env vars — no bundled email service) and a VAPID
  keypair for Web Push (generated at deploy time, self-hosted, no third-party push
  service needed). `speed_events` (Step 16) becomes the trigger source: on insert,
  api-service fans out to whichever channels are enabled for that org/user.

## Step 19 — Trip segmentation thresholds
- **Decision:** Configurable per-device.
- **Why:** User chose per-device control, fitting the same pattern as the per-device
  `speed_limit_kmh` (Step 16 item 3). New columns on `devices`: `trip_stop_speed_kmh`
  (below this = considered stationary, default e.g. 2) and
  `trip_stop_duration_seconds` (how long stationary before a trip is considered
  ended, default e.g. 300 = 5 min). Both editable anytime via dashboard/API, same as
  speed limit. The trip-segmentation background job (Step 16 item 8) reads these two
  fields per device instead of using a single global constant.

## Step 20 — Scaffold approach
- **Decision:** Full scaffold (Option A) — docker-compose.yml, Caddyfile, Go modules
  for api-service + protocol-gateway, complete Postgres/PostGIS migration covering
  every table/column decided through Step 19, and a React app skeleton, all at once.
- **Why:** User chose to generate the full working shell in one pass rather than
  splitting backend-first or schema-only. All decisions through Step 19 are settled,
  so there's no open design risk in generating everything together.

---

## Step 20 build log — scaffold in progress

Tracking actual implementation progress against the Step 20 scaffold plan, one
entry per finished piece, so the log stays accurate to what exists in the repo
vs. what's still planned.

### Sandbox/toolchain note
- `proxy.golang.org` and `golang.org` are not reachable from the build sandbox
  (network allowlist only includes `github.com`/`codeload.github.com`, not Go's
  module proxy or vanity-import redirector). Worked around with
  `GOPROXY=direct GOSUMDB=off` (fetches modules via git over github.com
  directly) plus two `replace` directives in `backend/api-service/go.mod`
  pointing `golang.org/x/net` and `golang.org/x/crypto` at their
  `github.com/golang/*` mirrors. This is a sandbox-only workaround — on a
  normal dev machine or CI with default network access, the standard
  `GOPROXY=https://proxy.golang.org` will work and the replace directives are
  harmless no-ops (same code, same versions). Flagging here in case it causes
  confusion later, no Step renumbering needed.

### Migration (`backend/migrations/0001_init.sql`) — done
- Full schema covering every table/column decided through Step 19:
  `organizations`, `users`, `devices` (incl. `speed_limit_kmh`,
  `trip_stop_speed_kmh`, `trip_stop_duration_seconds`,
  `offline_after_seconds`, `last_position_id`), `positions` (GEOGRAPHY point +
  GIST index), `geofences` (GEOGRAPHY polygon + GIST index),
  `geofence_events`, `speed_events`, `trips`, `activity_logs`,
  `notification_settings`. `devices.last_position_id` FK added after
  `positions` exists to avoid a forward reference.

### api-service Go module — in progress
- `go.mod` initialized (`github.com/trackproj/api-service`), deps: `lib/pq`
  (Postgres driver), `golang-jwt/jwt/v5`, `gorilla/websocket`, `go-chi/chi/v5`
  (router), `golang.org/x/crypto/bcrypt` (password hashing).
- `internal/db` — connection pool helper (`db.Connect`).
- `internal/models` — all domain structs matching the migration
  (`Device`, `Position`, `Geofence`, `SpeedEvent`, `Trip`, `ActivityLog`,
  etc.), plus `PositionIngest` as the protocol-agnostic ingest shape called
  out in Step 15/Step 8 so the future protocol-gateway can reuse it.
- `internal/auth` — JWT issue/parse for dashboard sessions (15min access /
  30-day refresh, matching Step 12's "JWT with refresh" for users);
  device-token generation + SHA-256 hashing (`dtk_...` plaintext shown once,
  hash stored, per Step 12); bcrypt password hashing; HTTP middleware
  (`RequireJWT`, `RequireDeviceToken`) implementing the Step 12 auth split at
  the transport layer.
- `internal/ws` — `Hub` for `/v1/live`, broadcasts JSON messages to dashboard
  clients scoped by `org_id` (per-org isolation), used for live positions,
  geofence events, and speed-violation pushes (Step 15, Step 18).
- `internal/handlers/auth.go` — `POST /v1/auth/register` (org + first admin
  user in one transaction), `POST /v1/auth/login`, `POST /v1/auth/refresh`.
  Done.
- Still to do: device management handlers, position ingest (+ inline
  geofence enter/exit and speed-violation checks per Step 16 items 3/7),
  geofence CRUD, trips/activity-logs/notification-settings handlers, router
  wiring + `cmd/api/main.go`, protocol-gateway stub, docker-compose.yml +
  Caddyfile, React dashboard skeleton.

---
*(Append further steps below as decisions are made.)*

### Completed in this session (continued from above)

**handlers/nulltypes.go** — nullable SQL scanner helpers (`sql_NullTime`,
`sql_NullString`, `sql_NullFloat64`) + `deriveStatus` (moving/stopped/offline
computed at query time, never stored — Step 16 item 7). Done.

**handlers/activity.go** — best-effort `logActivity` helper used by every
mutating handler (Step 16 item 11). Done.

**handlers/devices.go** — `POST /v1/devices` (create + return plaintext token
once), `GET /v1/devices` (list with computed status + last position via
`last_position_id`), `PATCH /v1/devices/{id}` (name / speed limit / trip
thresholds, Step 16 item 3 + Step 19), `DELETE`, `POST .../revoke-token`. Done.

**handlers/positions.go** — `POST /v1/positions` (device-token auth; inserts
GEOGRAPHY point; updates `last_position_id`; inline PostGIS ST_Within
geofence enter/exit state-machine against `geofence_events` prev-row; inline
speed violation check + `speed_events` insert; WS broadcast for position /
geofence / speed events); `GET /v1/positions` (history, from/to/device_id
filters, 500-row cap). Done.

**handlers/geofences.go** — `POST /v1/geofences` (ST_GeomFromGeoJSON),
`GET /v1/geofences` (ST_AsGeoJSON), `DELETE /v1/geofences/{id}`,
`GET /v1/geofences/{id}/events`. Done.

**handlers/trips.go** — `GET /v1/trips` (device + date filter),
`GET /v1/activity` (org-scoped audit log). Done.

**handlers/live.go** — `GET /v1/live` WebSocket upgrade → `ws.Hub`. Done.

**handlers/router.go** — Chi router; three middleware groups: public, device-
token (POST /v1/positions), JWT (all dashboard routes); CORS for dev. Done.

**cmd/api/main.go** — reads `DATABASE_URL` / `JWT_SECRET` / `LISTEN_ADDR`
from env; connects DB; wires `JWTIssuer` + `Hub` + `Server`; starts HTTP
server with sensible timeouts. Done.

**`go build ./...` exits 0 for api-service.** Done.

**protocol-gateway** — `internal/event/event.go` (PositionEvent normalized
shape), `internal/protocols/phone/adapter.go` (forwards to api-service,
extension point for batch upload / alt encodings), `cmd/gateway/main.go`
(Chi router, /healthz, placeholder hardware adapter mounts). Compiles clean.

**Dockerfiles** — multi-stage Go builder → alpine for both services. Done.

**Caddyfile** — TLS (Let's Encrypt), `/v1/*` → api-service, `/proto/*` →
protocol-gateway, SPA fallback, gzip, security headers. Replace `YOUR_DOMAIN`
before deploy. Done.

**docker-compose.yml** — `db` (postgis:16, migrations auto-run on first
start), `api-service`, `protocol-gateway`, `caddy` (ports 80/443, mounts
`frontend/dist`). Healthchecks wired. `.env.example` provided. Done.

**React dashboard** — Vite + React 18; `api.js` (axios + auto-refresh on
401); `AuthContext.jsx` (login/register/logout, localStorage tokens);
`useSocket.js` (WS hook, exponential backoff reconnect); `LoginPage.jsx`;
`DashboardPage.jsx` (Leaflet map, live dot markers coloured by status, WS
position updates, geofence/speed toast notifications, device sidebar);
`DevicesPage.jsx` (add device → show token once, rotate token, delete,
settings table); `App.jsx` (QueryClientProvider + AuthProvider + Router +
protected routes + Layout nav). **`npm run build` exits 0**, 461 kB JS /
144 kB gzipped. Done.

### What remains (not yet built)
- **Trip segmentation worker** — background process applying
  `trip_stop_speed_kmh` + `trip_stop_duration_seconds` thresholds to write
  completed `trips` rows (Step 19). Schema exists, nothing writes to it yet.
- **Notification delivery** — `notification_settings` table exists; email /
  web-push delivery on geofence/speed events not wired (Step 18 backend).
- **Geofence editor UI** — draw polygons on the map, POST to `/v1/geofences`
  (Step 17 frontend).
- **Android app** — Step 8 decisions logged; app scaffold not started.
- **History/replay page** — playback a device track for a time range.
