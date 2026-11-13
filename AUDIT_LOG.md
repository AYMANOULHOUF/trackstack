# TrackStack — Full Repository Audit

**Date:** 2026-11-13
**Branch:** `chore/full-audit` (base commit `79a1e70` on `main`)
**Auditor:** automated coding-agent audit
**Scope:** documentation vs. code, builds/tests, Docker deployment, API ↔ dashboard ↔ Android consistency, security and repository hygiene.

**Outcome:** every runnable check passes. Documentation was corrected to match the
code, the Android branding was fixed, a development-only dashboard URL bug was
fixed, and repository hygiene files were added. **No API behaviour, database
schema, or existing migration was changed.** Unresolved items are listed with
severity at the end.

---

## 1. Ground rules and assumptions

- Work was done only on `chore/full-audit`; `main` was not modified or force-pushed.
- The real `.env` was **never read, printed, edited, or committed**. The Docker
  test used a throwaway env file at `/tmp/opencode/audit.env` with dummy values,
  an isolated Compose project (`trackstack-audit`), and alternate host ports
  (`18080`, `15433`, `15023`). The user's running stack was left untouched.
- No files were deleted. The two stale root docs were moved to `docs/`.
- Code is treated as the source of truth wherever it disagrees with prose.
- Temporary test scripts live outside the repo (`/tmp/opencode/`). The isolated
  audit containers, images, and volumes were removed afterwards; the user's
  volumes were not touched.
- `docs/Android-Java-Migration.docx` is a binary document and was not
  re-authored; only its existence and references were checked.

---

## 2. Claim verification (docs vs. code)

| # | Claim in docs | Document location | Actual value in code/config | Fixed? |
|---|---|---|---|---|
| 1 | Android client is Kotlin + Jetpack Compose | `README.md` intro, Kotlin badge, stack table, repo layout | **Java 17 + XML layouts**; sources in `android/app/src/main/java`, layouts in `res/layout`; no Kotlin source or Compose dependency | **Yes** — README rewritten |
| 2 | App is named “TrackProj” | `AndroidManifest.xml` literal label, `strings.xml`, tracking notification | Product/repo is **TrackStack** | **Yes** — label now `@string/app_name`; strings and notification title say TrackStack |
| 3 | Live map renders OpenStreetMap tiles | `README.md` feature list, `frontend/README.md` | Leaflet map with **keyless Esri** basemaps (`frontend/src/mapTiles.js`) | **Yes** — docs now say Esri |
| 4 | Activity log records “tracking and device activity” | `README.md` feature list | Only `geofence.created`, `geofence.deleted`, and `notifications.updated` call `logActivity`; no position/tracking rows | **Yes** — wording narrowed |
| 5 | Go 1.22 | Badge + stack table | `go 1.22.2` in both `backend/*/go.mod` | Correct (kept) |
| 6 | React 19 | Badge + stack table | `react`/`react-dom` `^19.2.7` | Correct |
| 7 | Vite 8 | Stack table | `vite` `^8.1.1`; after lockfile audit fix the installed build is 8.3.8 | Correct |
| 8 | PostgreSQL 16 + PostGIS 3.4 | Stack table + architecture diagram | `postgis/postgis:16-3.4-alpine` | Correct |
| 9 | Node.js 22 | Requirements + badge | CI `node-version: 22`; `Dockerfile.caddy` `FROM node:22` | Correct |
| 10 | Java 17, `minSdk` 26, compile/target SDK 34 | Local Android section (now explicit) | `android/app/build.gradle.kts`: Java 17, minSdk 26, compileSdk/targetSdk 34 | Correct (detail added) |
| 11 | Caddy runs on `:8080` | Architecture diagram | Caddy listens on container `:80`; Compose publishes host `8080:80` | **Yes** — diagram corrected |
| 12 | `npm install` | README, `frontend/README.md` | Lockfile is tracked and CI uses `npm ci` | **Yes** — docs use `npm ci` |
| 13 | Repository layout | README layout block | Missing `docs/`, `scripts/`, `Dockerfile.caddy`; listed root-level doc paths that moved | **Yes** |
| 14 | Design notes live at repo root | README design-notes section | Moved to `docs/read_me.md` and `docs/session_log_android_bootstrap.md` | **Yes** — links updated |
| 15 | Migration step applies only `0004` | Applying-migrations section | Four migrations exist; `0001`/`0002` are **not** idempotent, so order and “already applied” matter | **Yes** — clarified |
| 16 | CI runs “on every push” | Status section | `push` only on `main`, plus `pull_request` and `workflow_dispatch` | **Yes** — wording corrected |
| 17 | `VITE_API_URL` targets all API traffic | `frontend/README.md` | WebSocket and refresh call hardcoded the page host / `/v1`; Vite dev server has no proxy, so dev live map and token refresh broke when `VITE_API_URL` pointed at another host | **Yes** — code fixed |
| 18 | Admin credentials are seeded once | `.env.example` comment | API upserts the admin on every boot, so rotating `ADMIN_PASSWORD` and restarting updates the login | **Yes** — comment clarified |
| 19 | Documented environment variables | README table vs. Compose + Go | All match: required `DB_PASSWORD`, `JWT_SECRET`, `ADMIN_PASSWORD`; optional `ADMIN_EMAIL`, `DB_PORT`, `GATEWAY_SECRET`, `GT06_PORT`, `TRIP_WORKER_INTERVAL`, `SMTP_*`, `VAPID_*` | Correct |
| 20 | API route overview | README “API overview” vs. `handlers/router.go` | Public, device-token, gateway, JWT, and admin groups all match, including `/v1/live` and `/v1/devices/{id}/revoke-token` | Correct |
| 21 | Host ports | README vs. Compose | Host `8080` → Caddy `80`; host `5433` → Postgres `5432`; host `5023` → GT06 `5023`; API `8080` and gateway `8081` remain internal | Correct after diagram fix |
| 22 | CI action versions | `.github/workflows/ci.yml` | Tags checked against GitHub releases: checkout `v7.0.1`, setup-go `v7.0.0`, setup-node `v7.1.0`, setup-java `v6.2.0`, setup-android `v4.0.4`, upload-artifact `v7.0.2` | Verified |
| 23 | No secrets are committed | Repository | Only `.env.example` is tracked. No private keys, tokens, or real passwords found. `README.md` contains only the literal local example `ADMIN_PASSWORD='dev-password'` | Verified |
| 24 | Dependency audit is clean | `frontend/package-lock.json` | Remaining: **2 moderate** `react-router` advisories. Fixing requires `react-router-dom` v7 (breaking), so it was not applied | Partially fixed |

---

## 3. Build, test, and runtime verification

All commands below were run from a clean dependency state where applicable.

| Check | Command | Result |
|---|---|---|
| API service build | `cd backend/api-service && go build ./...` | **Pass** |
| API service vet | `go vet ./...` | **Pass** |
| API service tests | `go test ./...` | **Pass** — `internal/notify`, `internal/trips`; other packages have no tests |
| Gateway build | `cd backend/protocol-gateway && go build ./...` | **Pass** |
| Gateway vet | `go vet ./...` | **Pass** |
| Gateway tests | `go test ./...` | **Pass** — `internal/protocols/gt06`; other packages have no tests |
| Go modules tidy | `go mod tidy` in both modules | **No changes** — dependencies are tidy |
| Frontend install | `cd frontend && npm ci` | **Pass** |
| Frontend lint | `npm run lint` | **Pass** — 5 warnings, 0 errors |
| Frontend production build | `npm run build` | **Pass** — JS bundle 503.68 kB (chunk-size warning only) |
| Android debug build | `cd android && JAVA_HOME=<JDK17> ./gradlew assembleDebug` | **BUILD SUCCESSFUL** |
| Compose validation | `docker compose --env-file /tmp/opencode/audit.env config` | **Valid** |
| Docs/version check | `python3 scripts/check_docs.py` | **Pass** |
| Dependency audit | `npm audit` | **2 moderate** (`react-router`), no high/critical after lockfile fix |

### Isolated Docker smoke test

Run with the real Compose file copied to `/tmp/opencode/audit-compose.yml`,
alternate host ports, and dummy credentials. **50/50 checks passed**, covering:

- `/healthz` returns `ok`; Caddy serves the SPA `index.html`.
- Admin login (200) and wrong password (401); org creation (201) and org login.
- Role isolation: org user gets 403 on `/v1/orgs` (GET and POST).
- Device enrollment (201), `dtk_` token format, admin device→org assignment (204).
- Position ingest: no token 401, bogus token 401, valid device token 201; tracking-state 204/401.
- Org-scoped reads: positions, device list, trips, activity, geofences.
- Geofence create/list/events/delete; invalid notification channel returns 400.
- Hardware ingest: wrong gateway secret 401, unknown `external_id` 404, valid secret 201.
- HTTP/JSON adapter `/proto/http/positions`: gateway-secret mode 201, device-token mode 201, no auth 401.
- GT06 TCP: synthetic login frame acknowledged, synthetic location frame acknowledged, position stored.
- CORS preflight returns 204.
- Stored positions were visible from phone, gateway and GT06 paths.

### Dashboard runtime check

Playwright against the isolated stack (`http://127.0.0.1:18080`):

- Admin login succeeded and `/admin/map` rendered.
- Leaflet map present (1366×710), **24/24 Esri tiles loaded**, **1 marker**.
- No console or page errors.
- 16 request failures were recorded during navigation with no visible or
  functional impact (map tiles all completed). These were not individually
  triaged — see *Unverified*.
- The user's own running stack (`trackstack-*`, API on `127.0.0.1:8080`) was not
  restarted, read, or modified.

---

## 4. Cross-client / API consistency

- **Dashboard → API:** all `api.get/post/put/patch/delete` paths exist in the
  router; auth header and refresh flow match `/v1/auth/*`; live socket uses
  `/v1/live?token=…`. Request bodies for orgs, devices, geofences, and
  notifications match the Go request structs.
- **Android → API:** `ApiClient` uses `/v1/enroll`, `/v1/positions`, and
  `/v1/tracking-state` with the device token; the payload fields match the Go
  `ingestRequest`. Enrollment handles the one-time `dtk_` token and stores it
  locally.
- **Gateways → API:** GT06 and HTTP/JSON both forward to
  `/v1/gateway/positions` (secret mode) or `/v1/positions` (device-token mode);
  `external_id` resolution matches the `devices.external_id` column.
- **Ports/auth:** documented ports and auth scopes match Compose, Caddy, and the
  router. `GATEWAY_SECRET` empty disables secret-mode ingest in both services
  (API returns 503; the gateway rejects the secret branch).

---

## 5. Security review and recommendations

### High

1. **Unauthenticated, unlimited device enrollment.** `POST /v1/enroll` is public
   and has no rate limit or invite control. Anyone can create unbounded pending
   device rows. Recommend per-IP rate limiting plus an enrollment code or
   admin-approved flow.
2. **No rate limiting on login or ingest.** `/v1/auth/login`,
   `/v1/enroll`, `/v1/gateway/positions`, and device `/v1/positions` have no
   throttling. Add a per-IP limiter (application middleware or Caddy) to reduce
   brute-force and resource-exhaustion risk.
3. **Permissive CORS.** The API returns `Access-Control-Allow-Origin: *`.
   Bearer tokens are not cookies, so this is not classic CSRF, but any origin
   can call the API and read responses when it has a token. Restrict to the
   dashboard origin(s).
4. **JWT in the WebSocket query string.** Browsers cannot set headers on
   `WebSocket`, so `/v1/live?token=…` is accepted. Query strings are commonly
   logged by proxies and Caddy. Access tokens live 15 minutes; recommend a
   short-lived one-time WS ticket or a cookie/subprotocol approach.
5. **No coordinate/range validation on ingest.** `/v1/positions` and
   `/v1/gateway/positions` accept any `lat`/`lon`/`speed`/`heading` values and
   zero `recorded_at`. Validate latitude ±90, longitude ±180, and plausible
   speed/heading before writing to PostGIS; return 400 otherwise.
6. **Refresh tokens cannot be revoked.** Refresh JWTs are stateless with a
   30-day TTL and no `jti`/denylist or per-user token version. A stolen refresh
   token remains valid until expiry. Consider a token-version column or a
   revocation store.

### Medium

1. **Admin can reach org-scoped handlers with an empty org.** The global admin
   has no `org_id`, but geofence/trip/notification/activity routes are not
   wrapped in an org guard. Creating a geofence as admin fails with a foreign-key
   error (`500`); other reads return empty data. Add a `RequireOrg` middleware or
   return an explicit `403` for admin on those routes.
2. **Invalid GeoJSON surfaces as `500`.** PostGIS errors from
   `ST_GeomFromGeoJSON` are returned as internal errors. Validate the payload and
   map malformed geometry to `400`. The same applies to malformed UUID path
   parameters in some handlers.
3. **Changing `ADMIN_EMAIL` adds an admin instead of renaming one.** `seedAdmin`
   upserts by email, so a new address creates a second admin and leaves the old
   account active. Document this or implement an explicit rename/removal step.
4. **Weak fallback JWT secret in direct runs.** The API defaults `JWT_SECRET` to
   `change-me-in-production` when run outside Compose. Compose requires it, but a
   direct `go run` does not. Fail fast when it is unset outside development.
5. **Plain HTTP by default.** `Caddyfile` listens on `:80` with no TLS/HSTS.
   Credentials and bearer tokens travel in clear text unless the operator puts a
   TLS terminator in front. Document HTTPS setup and add HSTS when TLS is enabled.
6. **Android transport and token storage.** The manifest sets
   `android:usesCleartextTraffic="true"` and `allowBackup="true"`; the device
   token is stored in plain `SharedPreferences`. Fine for emulator/dev, but for
   production use HTTPS, disable cleartext, and consider encrypted preferences.

### Low / informational

- No Content-Security-Policy header; Caddy sets `X-Content-Type-Options`,
  `X-Frame-Options`, and `Referrer-Policy` only.
- No request-ID / structured access logs beyond Chi's default logger.
- Enrollment returns the device token in the response body by design; ensure
  proxies do not log bodies.

### Dead code / unused packages

Confirmed unused (no references outside their own package):

- `backend/protocol-gateway/internal/protocols/phone/adapter.go`
- `backend/protocol-gateway/internal/event/event.go`
- `backend/api-service/internal/models/models.go`

These build cleanly but are not wired into either service. Removing them would
reduce confusion; they were left untouched because they are not harmful and the
audit avoided behavioural cleanup.

### Dependency advisories

- `npm audit` after the non-breaking lockfile update: **2 moderate**
  `react-router` advisories. The fix requires `react-router-dom` v7, a breaking
  major upgrade; not applied. Dashboard routing was verified on v6.
- Previously reported high-severity `vite`, `source-map-js`, and
  `follow-redirects` advisories were resolved by `npm audit fix` (lockfile
  only; `package.json` ranges unchanged).

### TODO / FIXME / hardcoded secrets

- No `TODO`, `FIXME`, `XXX`, or `HACK` markers in Go, JS/JSX, Java, XML, YAML,
  or Markdown.
- No hardcoded real credentials or API keys. The only URLs are the documented
  emulator host `10.0.2.2`, localhost development defaults, and public tile/CI
  URLs.
- `.env.example` contains placeholder values only.

---

## 6. Changes made

1. **Documentation corrected to match the code**
   - `README.md`: Java + XML Android client, Android/Node badges, Esri basemaps,
     accurate activity-log scope, Caddy `:80`/host `:8080` diagram, stack table,
     repository layout, local requirements, `npm ci`, migration order and
     idempotency, `docs/` links, CI scope wording.
   - `frontend/README.md`: Esri basemaps, `npm ci`, `VITE_API_URL` now covers the
     live WebSocket and refresh call, `mapTiles.js` wording.
   - `docs/read_me.md` and `docs/session_log_android_bootstrap.md`: moved from
     the repository root and given historical-context notes; content preserved.
   - `.env.example`: clarified that the admin is upserted on every boot.

2. **Dashboard development fix**
   - `frontend/src/useSocket.js`: derives the WebSocket URL from
     `VITE_API_URL` (not the page host), and stops reconnecting after unmount.
   - `frontend/src/api.js`: token refresh uses the same base URL as the API
     client.

3. **Android branding fix**
   - Manifest uses `@string/app_name`; `app_name` and permission strings are
     “TrackStack”; the foreground notification title is “TrackStack” (was
     “TrackProj”).

4. **Repository hygiene**
   - Added `LICENSE` (MIT, “TrackStack contributors” — chosen because no license
     existed; confirm the desired license before publishing).
   - Added `CONTRIBUTING.md` with build/test commands and a documentation
     checklist.
   - Added `scripts/check_docs.py` and a CI `docs` job that verifies the README’s
     Go/React/Vite/Postgres/PostGIS/Node versions against `go.mod`,
     `frontend/package.json`, `docker-compose.yml`, CI, and `Dockerfile.caddy`.
   - Expanded `.gitignore` (`.gradle/`, `frontend/.vite/`,
     `frontend/coverage/`, `android/.kotlin/`, `*.log`, `*.hprof`).
   - Applied non-breaking `npm audit fix` (lockfile only).

No API handler, request/response shape, database schema, or existing migration
was modified.

---

## 7. Unverified / not run

- **GitHub Actions was not executed on GitHub.** Workflow YAML was reviewed and
  every action tag was confirmed to exist, but the jobs themselves were not run
  in this environment.
- **Android on a physical device.** Only `assembleDebug` and the existing
  emulator were exercised; background location, notification permission flows,
  and battery-optimisation prompts were not re-tested on hardware.
- **Email and Web Push delivery.** No SMTP server or VAPID keys were configured;
  dashboard settings and API validation were tested, actual delivery was not.
- **Real GT06 hardware.** The TCP adapter was tested with synthetic login and
  location frames, not a physical tracker.
- **Playwright request failures.** 16 failed network requests were recorded
  during dashboard navigation while all map tiles completed and no console
  errors occurred. They were not individually triaged.
- **`docs/Android-Java-Migration.docx`.** The binary document was not reviewed
  in detail; only its presence and references were checked.
- **Router v7 upgrade.** Not attempted because it is a breaking major upgrade.
- **Test coverage.** Only `internal/notify`, `internal/trips`, and the GT06
  adapter have Go tests; handlers, auth, WebSocket, and the gateway forwarder do
  not. No frontend unit/e2e suite existed before this audit; the added check is
  runtime Playwright/manual only.
- **Branch protection / merge settings.** Not inspected or changed.

---

## 8. Suggested GitHub metadata (manual)

Repository description:

> Self-hosted fleet & GPS tracking: Go API, React dashboard, Java/XML Android
> app, PostGIS, Caddy, Docker Compose.

Topics: `go`, `react`, `vite`, `leaflet`, `postgis`, `postgresql`,
`docker-compose`, `self-hosted`, `gps-tracking`, `fleet-management`, `android`,
`java`, `xml-layouts`, `osmdroid`, `gt06`, `rest-api`, `websocket`.

Remove topics `kotlin` and `jetpack-compose`, which no longer describe the app.
