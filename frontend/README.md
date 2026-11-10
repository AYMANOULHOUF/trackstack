# TrackStack Dashboard

React 19 + Vite dashboard for [TrackStack](../README.md). It talks to the Go API over REST and a WebSocket, and renders device positions on a Leaflet/OpenStreetMap map.

## Stack

- React 19 + Vite 8
- React Router
- TanStack Query
- Axios (JWT access token + automatic refresh)
- Leaflet + react-leaflet

## Run locally

```bash
npm install
VITE_API_URL=http://localhost:8080/v1 npm run dev
```

The app starts on `http://localhost:5173`. `VITE_API_URL` must point at the API service; without it the app falls back to the relative `/v1` path.

## Scripts

| Command | Description |
| --- | --- |
| `npm run dev` | Start the Vite dev server |
| `npm run build` | Production build to `dist/` |
| `npm run preview` | Preview the production build |
| `npm run lint` | Run Oxlint |

## Structure

```
src/
  api.js            Axios instance, auth header, refresh interceptor
  AuthContext.jsx   Login state and token handling
  useDevices.js     Device queries
  useSocket.js      Live WebSocket connection
  mapTiles.js       Shared basemap definitions
  pages/            Login, admin, and organization views
  components/       Shared UI and the Leaflet map component
```

The organization views cover the live map, devices, geofence drawing, trips, and notification settings. In production the built `dist/` is served by Caddy from the repository root; see the main README for Docker deployment.
