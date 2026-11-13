# Contributing to TrackStack

Thanks for helping improve TrackStack. This file describes the checks to run
and the documentation to update before opening a pull request.

## Before you start

- Do not commit `.env` or any real secret. Copy `.env.example` to `.env` for
  local runs and keep it untracked.
- Use a feature branch and open a pull request against `main`.
- Keep API behaviour, database schema, and existing migrations stable unless
  the change is intentional and documented.

## Build and test

Run the checks that match what you changed. CI runs the same commands.

### Backend (Go 1.22)

```bash
cd backend/api-service
go build ./... && go vet ./... && go test ./...

cd ../protocol-gateway
go build ./... && go vet ./... && go test ./...
```

### Dashboard (Node.js 22)

```bash
cd frontend
npm ci
npm run lint
npm run build
```

### Android (JDK 17, Android SDK 34)

```bash
cd android
./gradlew assembleDebug
```

### Docker

```bash
cp .env.example .env    # fill in throwaway values for a local test
docker compose config
docker compose up --build
curl -fsS http://localhost:8080/healthz
docker compose down     # never use -v on a volume with data you need
```

## Documentation checklist

Update the docs in the same pull request as the code:

- [ ] **README.md** — if a version, service, port, endpoint, environment
      variable, repo layout entry, or roadmap item changed. Keep the version
      badges and the stack table in sync with `go.mod`, `frontend/package.json`,
      and `docker-compose.yml` (CI runs `scripts/check_docs.py`).
- [ ] **README.md API overview** — when adding, removing, or renaming a route
      in `backend/api-service/internal/handlers/router.go`.
- [ ] **`.env.example`** — when the API, gateway, or Compose file reads a new
      environment variable. Document safe defaults and never put real secrets in it.
- [ ] **`frontend/README.md`** — when dashboard scripts, dependencies, or the
      map implementation change.
- [ ] **Migrations** — do not edit an applied migration; add the next numbered
      file instead (`0005_...sql`) and note it in the README's migration section.
- [ ] **Roadmap** — tick an item only when the feature is implemented and
      covered by CI or a documented manual check.

## Commit style

Use short, imperative commit messages with an optional scope, for example:

```
fix(api): authorize trips through device_orgs
docs(readme): describe the Java + XML Android client
```

## Reporting issues

Include the command you ran, the expected and actual result, and the relevant
logs. Redact tokens, passwords, `.env` contents, and device identifiers.
