# TrackStack

**Self-hosted fleet & GPS tracking platform.** A Go API, a React + Leaflet dashboard, a Kotlin Android client, and PostGIS storage — packaged so you can run the whole stack on your own server.

<p>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-19-20232A?style=flat-square&logo=react&logoColor=61DAFB">
  <img alt="Kotlin" src="https://img.shields.io/badge/Kotlin-Compose-7F52FF?style=flat-square&logo=kotlin&logoColor=white">
  <img alt="PostgreSQL" src="https://img.shields.io/badge/PostgreSQL-16%20%2B%20PostGIS-4169E1?style=flat-square&logo=postgresql&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker%20Compose-2496ED?style=flat-square&logo=docker&logoColor=white">
</p>

> **Consent first.** TrackStack is built for devices you own or are explicitly authorized to track — your own phone, your own fleet, or hardware you administer. It is not a tool for covert tracking.

---

## What it does

- **Live map** — positions stream over WebSocket and render on an OpenStreetMap/Leaflet map.
- **Multi-tenant organizations** — a global admin creates organizations and assigns devices; org users only see their own fleet.
- **Device enrollment** — a freshly installed Android client self-enrolls and waits in a pending state until an admin assigns it.
- **Two auth models** — short-lived JWTs with refresh for dashboard users, long-lived revocable tokens for devices.
- **Geofences** — draw polygon geofences on the map and inspect enter/exit events.
- **Speed events & trips** — speed violations are recorded, and trips are segmented automatically from position history.
- **Notifications** — alerts reach the dashboard over WebSocket and can also be delivered by email or browser push.
- **Hardware trackers** — GT06/Concox TCP and HTTP/JSON trackers feed the same ingest pipeline through the protocol gateway.
- **Activity log** — an audit trail of tracking and device activity.
- **Android tracking** — a foreground location service keeps reporting while the app is backgrounded, with an osmdroid map and tracking on/off control.

---

## Architecture

```mermaid
flowchart LR
    Phone["Android client<br/>Kotlin + Compose"] -->|"device token<br/>POST /v1/positions"| API
    Browser["React dashboard<br/>Vite + Leaflet"] -->|"JWT + WebSocket"| Caddy
    Tracker["GT06 / HTTP tracker"] -->|"TCP :5023 or /proto/*"| GW
    Caddy["Caddy :8080"] --> API["Go API<br/>Chi + JWT"]
    Caddy -->|"/proto/*"| GW["Protocol gateway"]
    GW -->|"X-Gateway-Secret"| API
    API --> DB[("PostgreSQL 16<br/>+ PostGIS 3.4")]
    API -.->|"trip worker"| DB
    API -.->|"email / Web Push"| Alerts["SMTP + push services"]
```

| Layer | Implementation |
| --- | --- |
| **API** | Go 1.22, Chi router, `golang-jwt`, Gorilla WebSocket, `lib/pq` |
| **Gateway** | Separate Go service: GT06/Concox TCP adapter, HTTP/JSON adapter, shared secret to the API |
| **Database** | PostgreSQL 16 + PostGIS 3.4, SQL migrations mounted at container init |
| **Web** | React 19, Vite 8, React Router, TanStack Query, Axios, Leaflet + react-leaflet |
| **Android** | Kotlin, Jetpack Compose, foreground location service, osmdroid, OkHttp |
| **Deployment** | Docker Compose, Caddy serving the built frontend and reverse-proxying the API |

---

## Quick start (Docker)

**Requirements:** Docker with Compose v2.

```bash
git clone https://github.com/AYMANOULHOUF/trackstack.git
cd trackstack
cp .env.example .env
# edit .env and set DB_PASSWORD, JWT_SECRET, and ADMIN_PASSWORD
docker compose up --build
```

Open **http://localhost:8080** and sign in with `ADMIN_EMAIL` / `ADMIN_PASSWORD`.

The API is available at `http://localhost:8080/v1`, health check at `/healthz`.
Postgres is exposed on host port `5433` by default (override with `DB_PORT`).

### Environment variables

| Variable | Required | Purpose |
| --- | --- | --- |
| `DB_PASSWORD` | yes | Password for the Postgres user and the API's `DATABASE_URL` |
| `JWT_SECRET` | yes | Signs dashboard access/refresh tokens |
| `ADMIN_PASSWORD` | yes | Password for the seeded global admin; re-applied on every boot |
| `ADMIN_EMAIL` | no | Admin login (default `admin@trackstack.local`) |
| `DB_PORT` | no | Host port for Postgres (default `5433`) |
| `GATEWAY_SECRET` | no | Shared secret for hardware ingest; empty disables it |
| `GT06_PORT` | no | Host port for GT06/Concox trackers (default `5023`) |
| `TRIP_WORKER_INTERVAL` | no | Trip segmentation pass interval (default `30s`) |
| `SMTP_ADDR`, `SMTP_FROM` | no | Enable email alerts (`host:port` and sender address) |
| `SMTP_USER`, `SMTP_PASSWORD` | no | SMTP credentials when the server requires auth |
| `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY` | no | Enable Web Push alerts |
| `VAPID_SUBJECT` | no | VAPID contact, e.g. `mailto:you@example.com` |

Generate a strong JWT secret with `openssl rand -hex 32`. Never commit `.env`.

Generate a VAPID key pair from the API module:

```bash
cd backend/api-service
go run ./cmd/vapid
```

Email and Web Push are independent. When a channel is enabled in the dashboard
but its credentials are missing, the API logs why it skipped delivery and the
rest of tracking keeps working.

---

## Local development

### API

```bash
cd backend/api-service
export DATABASE_URL='postgres://trackproj:trackproj@localhost:5432/trackproj?sslmode=disable'
export JWT_SECRET='dev-secret'
export ADMIN_EMAIL='admin@trackstack.local'
export ADMIN_PASSWORD='dev-password'
go run ./cmd/api
```

Apply `backend/migrations/*.sql` to a PostGIS-enabled database before starting the API.

### Dashboard

```bash
cd frontend
npm install
VITE_API_URL=http://localhost:8080/v1 npm run dev
```

The Vite dev server runs on `http://localhost:5173`; set `VITE_API_URL` so API calls reach the Go service.

### Android

Open `android/` in Android Studio, update `BASE_URL` in `android/app/build.gradle.kts` to your machine's LAN address, then build and install. Grant location and notification permissions, then enroll the device.

---

## API overview

| Area | Endpoints |
| --- | --- |
| Public | `POST /v1/auth/login`, `POST /v1/auth/refresh`, `POST /v1/enroll`, `GET /healthz` |
| Device token | `POST /v1/positions`, `POST /v1/tracking-state` |
| Gateway | `POST /v1/gateway/positions` (requires `X-Gateway-Secret`) |
| Dashboard (JWT) | `GET /v1/live` (WebSocket), `/v1/devices`, `/v1/positions`, `/v1/geofences`, `/v1/trips`, `/v1/activity`, `/v1/notifications` |
| Admin only | `/v1/orgs`, `PUT /v1/devices/{id}/orgs`, `POST /v1/devices/{id}/revoke-token` |

---

## Repository layout

```
backend/
  api-service/        Go API (cmd/api, internal/auth, handlers, ws, db, models)
  protocol-gateway/   Go service for hardware protocol adapters
  migrations/         PostgreSQL/PostGIS schema
frontend/             React 19 + Vite dashboard
android/              Kotlin + Jetpack Compose tracking client
docker-compose.yml    db, api-service, protocol-gateway, caddy
Caddyfile             static frontend + API/gateway reverse proxy
```

---

## Hardware trackers

The protocol gateway accepts two kinds of hardware:

- **GT06/Concox** over raw TCP on `GT06_PORT` (default `5023`). The tracker logs
  in with its IMEI; the gateway acknowledges frames and forwards each fix.
- **HTTP/JSON** at `POST /proto/http/positions`, either with a normal device
  bearer token or with `external_id` plus `X-Gateway-Secret`.

Link a tracker to a device by setting its **Hardware ID** (IMEI or serial) in
the admin device editor. That value is stored as `external_id` and must be
unique. Until it is linked, forwarded positions are rejected with `404`.

---

## Trips

A background worker runs every `TRIP_WORKER_INTERVAL`. For each device it reads
fixes newer than a stored cursor and applies the device's stop thresholds:
a trip opens on the first fix above `trip_stop_speed_kmh`, accumulates
great-circle distance, and closes once the device has been stationary for
`trip_stop_duration_seconds`. The cursor lives in `devices.trip_cursor_position_id`,
so restarts never re-segment the same fixes.

The dashboard **Trips** page lists trips per device and draws the recorded
track for the selected trip.

---

## Notifications

Dashboard alerts (speed violations, geofence enter/exit) always stream over the
WebSocket firehose. The **Notifications** page adds two optional delivery
channels per organization:

- **Email** — requires `SMTP_ADDR` and `SMTP_FROM`; `SMTP_USER` /
  `SMTP_PASSWORD` are used when the server authenticates. Port `465` uses
  implicit TLS, other ports use STARTTLS when offered.
- **Web Push** — requires a VAPID key pair and an HTTPS origin (or localhost).
  The page registers `public/sw.js` and stores the browser subscription.

Delivery happens on a queue behind position ingest, so a slow SMTP server never
delays tracking.

---

## Applying migrations to an existing database

Docker's init directory only runs on a fresh volume. If you already have a
`pgdata` volume from an earlier version, apply new migrations manually:

```bash
docker compose exec -T db psql -U trackproj -d trackproj < backend/migrations/0004_trips_notifications_gateway.sql
```

---

## Roadmap

- [x] Automated trip segmentation worker
- [x] Geofence drawing UI
- [x] Email and Web Push notification delivery
- [x] Hardware protocol adapters (GT06 TCP + HTTP/JSON)
- [ ] iOS client (the codebase targets Android; no iOS target yet)

---

## Design notes

The decisions behind the architecture — auth model, background-tracking constraints, datastore choice, transport, and deployment — are documented in [`read_me.md`](read_me.md). The Android bootstrap is covered in [`session_log_android_bootstrap.md`](session_log_android_bootstrap.md).

## Status

TrackStack is under active development. The API, dashboard, and Android client are functional; the roadmap above tracks what is still in progress.
