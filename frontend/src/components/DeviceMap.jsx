import { useEffect, useMemo, useState, useRef } from 'react'
import { MapContainer, TileLayer, Marker, Popup, useMap } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { STATUS_COLOR, STATUS_LABEL, C } from '../ui'

const TILES = {
  dark: {
    url: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
    subdomains: 'abcd',
    attribution: '&copy; OpenStreetMap &copy; CARTO',
  },
  light: {
    url: 'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
    subdomains: 'abcd',
    attribution: '&copy; OpenStreetMap &copy; CARTO',
  },
}

function FitBounds({ points, disabled }) {
  const map = useMap()
  const didFit = useRef(false)
  useEffect(() => {
    if (disabled) return
    if (didFit.current) return
    if (points.length === 1) { map.setView(points[0], 15); didFit.current = true }
    else if (points.length > 1) { map.fitBounds(points, { padding: [60, 60], maxZoom: 15 }); didFit.current = true }
  }, [points, map, disabled])
  return null
}

function Follower({ device }) {
  const map = useMap()
  useEffect(() => {
    if (!device || device.last_lat == null || device.last_lon == null) return
    map.panTo([device.last_lat, device.last_lon], { animate: true })
  }, [device?.last_lat, device?.last_lon, device?.id, map])
  return null
}

function markerIcon(color, isFollowing) {
  const ring = isFollowing ? `0 0 0 4px #ffffff, 0 0 0 6px ${color}, 0 0 12px ${color}` : `0 0 0 2px ${color}, 0 0 10px ${color}`
  return L.divIcon({
    className: '',
    html: `<div style="width:16px;height:16px;border-radius:50%;background:${color};border:2.5px solid #0b0e14;box-shadow:${ring}"></div>`,
    iconSize: [16, 16], iconAnchor: [8, 8],
  })
}

export default function DeviceMap({ devices, subtitle }) {
  const [tile, setTile] = useState('dark')
  const [followId, setFollowId] = useState(null)

  const positioned = useMemo(
    () => devices.filter(d => d.last_lat != null && d.last_lon != null),
    [devices],
  )
  const points = useMemo(() => positioned.map(d => [d.last_lat, d.last_lon]), [positioned])
  const followed = useMemo(() => positioned.find(d => d.id === followId), [positioned, followId])

  return (
    <MapContainer center={[20, 0]} zoom={2} zoomControl={false} style={{ width: '100%', height: '100%' }}>
      <TileLayer key={tile} attribution={TILES[tile].attribution} url={TILES[tile].url} subdomains={TILES[tile].subdomains} />
      <FitBounds points={points} disabled={!!followId} />
      {followed && <Follower device={followed} />}
      {positioned.map(d => {
        const color = STATUS_COLOR[d.status] ?? STATUS_COLOR.unknown
        const isFollowing = d.id === followId
        return (
          <Marker key={d.id} position={[d.last_lat, d.last_lon]} icon={markerIcon(color, isFollowing)}>
            <Popup>
              <div style={{ minWidth: 180 }}>
                <div style={{ fontWeight: 700, fontSize: 14, marginBottom: 6 }}>{d.name}</div>
                {subtitle?.(d)}
                <div style={{ fontSize: 12, marginBottom: 4 }}>Status: <span style={{ color, fontWeight: 600 }}>{STATUS_LABEL[d.status] ?? d.status}</span></div>
                {d.last_speed != null && <div style={{ fontSize: 12, marginBottom: 4 }}>Speed: {Math.round(d.last_speed)} km/h</div>}
                {d.last_recorded_at && <div style={{ fontSize: 12, marginBottom: 10 }}>Last seen: {new Date(d.last_recorded_at).toLocaleTimeString()}</div>}
                <button
                  onClick={() => setFollowId(isFollowing ? null : d.id)}
                  style={{
                    width: '100%', padding: '7px 10px', borderRadius: 999, cursor: 'pointer', fontSize: 13, fontWeight: 600,
                    border: 'none', background: isFollowing ? '#ef4444' : '#7c6cff', color: '#fff',
                  }}>
                  {isFollowing ? 'Stop following' : 'Follow'}
                </button>
              </div>
            </Popup>
          </Marker>
        )
      })}
      <div style={{ position: 'absolute', top: 16, right: 16, zIndex: 500, display: 'flex', flexDirection: 'column', gap: 8 }}>
        <button
          onClick={() => setTile(tile === 'dark' ? 'light' : 'dark')}
          title={tile === 'dark' ? 'Switch to light map' : 'Switch to dark map'}
          style={{
            width: 42, height: 42, borderRadius: 12, border: `1px solid ${C.border}`, background: C.surface,
            color: C.text, cursor: 'pointer', fontSize: 18, boxShadow: '0 4px 20px rgba(0,0,0,.35)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
          {tile === 'dark' ? '☀' : '☾'}
        </button>
        {followId && (
          <button
            onClick={() => setFollowId(null)}
            title="Stop following"
            style={{
              padding: '0 14px', height: 42, borderRadius: 12, border: `1px solid ${C.border}`,
              background: C.surface, color: '#ef4444', cursor: 'pointer', fontSize: 13, fontWeight: 600,
              boxShadow: '0 4px 20px rgba(0,0,0,.35)',
            }}>
            Stop following
          </button>
        )}
      </div>
    </MapContainer>
  )
}
