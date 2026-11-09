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
- **Geofences** — create polygon geofences and inspect enter/exit events.
- **Speed events & trips** — speed violations are recorded, and trip history is available per device.
- **Activity log** — an audit trail of tracking and device activity.
- **Android tracking** — a foreground location service keeps reporting while the app is backgrounded, with an osmdroid map and tracking on/off control.

---

## Architecture

```mermaid
flowchart LR
    Phone["Android client<br/>Kotlin + Compose"] -->|"device token<br/>POST /v1/positions"| API
    Browser["React dashboard<br/>Vite + Leaflet"] -->|"JWT + WebSocket"| Caddy
    Caddy["Caddy :8080"] --> API["Go API<br/>Chi + JWT"]
    Caddy -->|"/proto/*"| GW["Protocol gateway"]
    API --> DB[("PostgreSQL 16<br/>+ PostGIS 3.4")]
    GW --> API
```

| Layer | Implementation |
| --- | --- |
| **API** | Go 1.22, Chi router, `golang-jwt`, Gorilla WebSocket, `lib/pq` |
| **Gateway** | Separate Go service reserved for hardware protocol adapters, proxied at `/proto/*` |
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

Generate a strong JWT secret with `openssl rand -hex 32`. Never commit `.env`.

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
| Dashboard (JWT) | `GET /v1/live` (WebSocket), `/v1/devices`, `/v1/positions`, `/v1/geofences`, `/v1/trips`, `/v1/activity` |
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

## Roadmap

- [ ] Automated trip segmentation worker
- [ ] Geofence drawing UI (API exists; map editor pending)
- [ ] Email and Web Push notification delivery
- [ ] Hardware protocol adapters (the gateway is scaffolded)
- [ ] iOS client

---

## Design notes

The decisions behind the architecture — auth model, background-tracking constraints, datastore choice, transport, and deployment — are documented in [`read_me.md`](read_me.md). The Android bootstrap is covered in [`session_log_android_bootstrap.md`](session_log_android_bootstrap.md).

## Status

TrackStack is under active development. The API, dashboard, and Android client are functional; the roadmap above tracks what is still in progress.
