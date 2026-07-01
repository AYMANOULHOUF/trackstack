# Session Log — Backend Fixes, Deployment Debugging, and Android App Bootstrap

Continuation of `read_me.md`. This session covered fixing the backend deployment,
verifying the tracking pipeline end-to-end, and starting the Android app from
scratch (scaffold → build toolchain → auth flow).

---

## Part 1 — Backend deployment fixes

### 1. `go mod download` failing (git not found)
- **Cause:** Both `backend/api-service/Dockerfile` and
  `backend/protocol-gateway/Dockerfile` had `RUN GOPROXY=direct GOSUMDB=off go mod download`,
  which forces fetching modules via raw git instead of the Go module proxy.
  The `golang:1.22-alpine` builder stage has no `git` binary, so this failed.
- **Fix:** Changed both Dockerfiles to plain `RUN go mod download`, letting Go
  use the default proxy (`https://proxy.golang.org,direct`), which doesn't
  need git for public modules like `go-chi/chi`.
- **Gotcha hit:** Fixed `api-service/Dockerfile` first and assumed it fixed
  both — `protocol-gateway/Dockerfile` is a **separate file** and still had
  the broken line until caught by a second build failure.

### 2. Caddy serving nothing on `127.0.0.1:8080`
- **Cause:** `Caddyfile` still had the literal placeholder `YOUR_DOMAIN {`
  never swapped for a real host/`:80`, so Caddy never matched incoming
  requests to `127.0.0.1`/`localhost`.
- **Fix:** Changed site block to `:80 { ... }` for local dev (serves any
  hostname on port 80 inside the container, mapped to host port 8080 via
  compose).
- **Secondary issue:** `frontend/dist` didn't exist yet (only `frontend/src`).
  Needed `npm install && npm run build` inside `frontend/` before Caddy had
  anything to serve.

### 3. `POST /v1/auth/register` → 405 Method Not Allowed
- **Cause:** Caddy directive ordering. Top-level `root`/`try_files` directives
  are sorted by Caddy internally (not execution order in the file), so
  `try_files` ran *before* the `handle /v1/*` block, rewrote the request path
  to `/index.html`, and the API route no longer matched — falling through to
  `file_server`, which only allows `GET`/`HEAD`.
- **Fix:** Nested `root` / `try_files` / `file_server` **inside** the final
  catch-all `handle { }` block so it only runs after `/v1/*` and `/proto/*`
  have had first chance to match (Caddy `handle` blocks act like a switch,
  evaluated top-to-bottom).

### 4. WebSocket `/v1/live` → `NS_ERROR_WEBSOCKET_CONNECTION_REFUSED`
- **Cause:** Browsers can't set custom headers on a native `WebSocket`
  handshake, so the frontend sends the JWT as `?token=...` in the URL. The
  `RequireJWT` middleware's `bearerToken()` helper only checked the
  `Authorization` header, found nothing, returned 401 — which Firefox/Chrome
  surface as a generic "connection refused" rather than a readable 401.
- **Fix:** `bearerToken()` in `internal/auth/middleware.go` now falls back to
  `r.URL.Query().Get("token")` when no `Authorization` header is present.
- **Noted but not acted on:** JWTs in query strings can leak into access
  logs/browser history. Fine for dev; flagged as a hardening item for later
  (e.g. short-lived single-use WS ticket exchange).

---

## Part 2 — End-to-end pipeline verification (curl)

Full flow tested manually against the running backend:

1. `POST /v1/auth/login` → got `access_token` / `refresh_token`.
2. `POST /v1/devices` (JWT auth) → created a device, got back a
   `dtk_...` **device token** (shown once, matches Step 12's design).
3. `POST /v1/positions` (device-token auth) → `201 Created`,
   `{"position_id":1}` — confirms PostGIS insert path works.
4. `GET /v1/positions?device_id=...` (JWT auth) → returned the stored
   position — confirms read path + `device_id` filter is **required**
   (no implicit "all devices" query).
5. Sent a second position while the dashboard was open in-browser →
   **live marker update confirmed working over WebSocket**, no page
   refresh needed. This validated the Part 1 §4 WebSocket fix.

Conclusion: backend ingest → storage → live push → dashboard render is fully
working end-to-end.

---

## Part 3 — Android app: toolchain bootstrap

Starting from zero — no JDK 17, no Gradle, no Android SDK, no Gradle wrapper
existed on the dev machine (Fedora 44).

1. **Gradle** — not in Fedora repos (`dnf install gradle` → no match).
   Installed via **SDKMAN** instead (`curl -s "https://get.sdkman.io" | bash`,
   then `sdk install gradle`).
2. **Gradle wrapper** — generated inside `android/` via
   `gradle wrapper --gradle-version 8.7`. Produced `gradlew`, `gradlew.bat`,
   `gradle/wrapper/gradle-wrapper.jar`, `gradle-wrapper.properties`.
3. **Java version mismatch** — system default was JDK 25 (too new for
   Gradle 8.7, which supports up to 21). Installed JDK 17 via SDKMAN
   (`sdk install java 17.0.11-tem`) and pinned it **per-project** by adding
   `org.gradle.java.home=/home/ayman/.sdkman/candidates/java/17.0.11-tem`
   to `android/gradle.properties` — system-wide Java untouched.
4. **Android SDK** — none installed. Downloaded Google's command-line tools
   zip manually, arranged into the required
   `~/Android/sdk/cmdline-tools/latest/` layout, then used `sdkmanager`
   (Fedora substituted its own `python3-sdkmanager` package when prompted,
   which still worked) to install `platform-tools`, `platforms;android-34`,
   `build-tools;34.0.0`.
5. Created `android/local.properties` with `sdk.dir=/home/ayman/Android/sdk`
   so Gradle can find the SDK.
6. **First successful build:** `./gradlew assembleDebug` → `BUILD SUCCESSFUL`.

---

## Part 4 — Android app: scaffold contents

```
android/
├── settings.gradle.kts
├── build.gradle.kts               (root, plugin versions)
├── gradle.properties              (JVM args + pinned JDK 17 home)
├── local.properties                (sdk.dir, machine-specific, not committed)
├── gradlew / gradlew.bat / gradle/wrapper/...
└── app/
    ├── build.gradle.kts            (namespace com.trackproj.app, minSdk 26,
    │                                 compileSdk/targetSdk 34, deps: core-ktx,
    │                                 appcompat, material, lifecycle-service,
    │                                 play-services-location, okhttp, osmdroid)
    └── src/main/
        ├── AndroidManifest.xml     (permissions: INTERNET, FINE/COARSE/
        │                            BACKGROUND location, FOREGROUND_SERVICE(+LOCATION),
        │                            POST_NOTIFICATIONS; usesCleartextTraffic=true
        │                            for dev HTTP; declares LoginActivity as
        │                            launcher, MainActivity as secondary,
        │                            LocationTrackingService as a foregroundServiceType="location"
        │                            service — service class not yet implemented)
        ├── res/
        │   ├── mipmap-anydpi-v26/ic_launcher.xml (+ _round variant)
        │   ├── drawable/ic_launcher_foreground.xml (placeholder vector icon)
        │   ├── values/colors.xml
        │   └── layout/activity_login.xml (email/password/login button/status text)
        └── java/com/trackproj/app/
            ├── MainActivity.kt         (placeholder post-login screen)
            ├── LoginActivity.kt        (launcher activity — see Part 5)
            └── auth/
                ├── TokenStore.kt       (SharedPreferences wrapper: access/refresh
                │                        token, device id/token, isLoggedIn(),
                │                        hasDevice(), clear())
                └── ApiClient.kt        (OkHttp-based: login(), registerDevice();
                                          base URL defaults to http://localhost:8080)
```

---

## Part 5 — Android app: auth flow (implemented & verified working)

- `LoginActivity` is the launcher activity. On create, if `TokenStore` already
  has both an access token and a device token, it skips straight to
  `MainActivity` (persistent login).
- On login button tap: runs `ApiClient.login()` and `ApiClient.registerDevice()`
  on a background `Thread` (required — network calls are illegal on the main
  thread), then posts results back to the main thread via a `Handler` to
  update UI / navigate.
- On success: stores `access_token`, `refresh_token`, `device_id`,
  `device_token` (the `dtk_...` value) in `TokenStore`, then navigates to
  `MainActivity`.
- Device name sent at registration: `"${Build.MANUFACTURER} ${Build.MODEL}"`
  (identifies the physical phone in the dashboard's device list).

### Bugs hit and fixed while building this
- **Path bug:** `TokenStore.kt` / `ApiClient.kt` were first created while
  still sitting inside `trackproj/android/`, but the `mkdir -p android/app/...`
  command still had the `android/` prefix — created a phantom
  `android/android/app/...` nested path. Kotlin compiler then failed with
  `Unresolved reference: auth` since the files weren't where the manifest/
  package expected. Fixed by locating the misplaced files and `mv`-ing them
  to the correct path.
- **Cleartext HTTP blocked:** Added `android:usesCleartextTraffic="true"` to
  the manifest — required since the dev API is plain HTTP, not HTTPS.
- **`adb reverse` not persistent:** The phone can't reach the dev machine's
  `localhost:8080` directly (it means "the phone itself" to the phone). Used
  `adb reverse tcp:8080 tcp:8080` to forward the phone's localhost:8080 to
  the dev machine over USB. **This mapping is lost on ADB server restart /
  device reconnect** and had to be re-run mid-session after a
  `failed to connect to localhost/127.0.0.1:8080` error on the phone.

### Physical device testing setup (for reference)
- Enabled Developer Options (tap Build Number x7) + USB debugging.
- `adb devices` initially showed `unauthorized` — resolved by accepting the
  "Allow USB debugging?" prompt on the phone (had to kill/restart the adb
  server once when the prompt didn't fire on first plug-in).
- Installed via `adb install app/build/outputs/apk/debug/app-debug.apk`
  (`-r` flag for reinstall over an existing install).

### Verified working end-to-end on physical device
Login screen → real login against backend → device auto-registered → tokens
stored on-device → navigated to post-login screen. Confirmed via manual test
on the phone.

---

## What's NOT built yet (Android)

- **`LocationTrackingService`** — declared in the manifest
  (`foregroundServiceType="location"`) but the Kotlin class doesn't exist yet.
  This is the actual tracker: foreground service + persistent notification +
  periodic location capture (`FusedLocationProviderClient` from
  play-services-location, already a dependency) + `POST /v1/positions` using
  the stored `device_token`.
- **Runtime permission requests** — manifest declares
  `ACCESS_FINE_LOCATION` / `ACCESS_BACKGROUND_LOCATION` /
  `POST_NOTIFICATIONS`, but no in-app runtime permission prompts exist yet.
  Android 10+ requires background location to be requested *separately* and
  *after* foreground location is already granted (two-step flow), plus a
  rationale screen if targeting Play Store policy later.
- **Battery optimization exemption** — not yet requested; without it, Android
  may kill the background service more aggressively on some OEM skins
  (especially Samsung/Xiaomi/etc.).
- **MainActivity is still a placeholder** — no real "tracking is on/off"
  UI, no map view (osmdroid is a dependency but unused so far), no way to
  start/stop the service from the app.
- **Token refresh** — `refresh_token` is stored but nothing in `ApiClient`
  uses it yet; a 401 on any future authenticated call (besides the WS/device
  flows) will currently just fail rather than silently refreshing.
- **`local.properties`** is machine-specific (contains a local filesystem
  path) — confirm it's in `.gitignore` before committing, since it'll break
  on any other machine.
- **`adb reverse` is dev-only** — this whole setup assumes phone + dev
  machine connected via USB. Real-world usage will need the phone hitting a
  real public/LAN address (or the eventual production domain via Caddy),
  not `localhost`.

---

## Suggested next steps (in order)

1. Implement `LocationTrackingService`: foreground service, notification
   channel, `FusedLocationProviderClient` location updates, POST to
   `/v1/positions` on each update using `TokenStore.deviceToken`.
2. Runtime permission flow in `MainActivity` (foreground location first,
   then background location, then notifications on Android 13+).
3. Wire a start/stop tracking toggle in `MainActivity` UI.
4. Battery optimization exemption prompt.
5. Swap `local.properties` into `.gitignore` if not already.
6. Later: token refresh handling in `ApiClient`, real app icon/branding,
   osmdroid map view of the device's own trail.
