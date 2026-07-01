import { useEffect, useMemo } from 'react'
import { MapContainer, TileLayer, Marker, Popup, useMap } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { STATUS_COLOR } from '../ui'

function FitBounds({ points }) {
  const map = useMap()
  useEffect(() => {
    if (points.length === 1) map.setView(points[0], 15)
    else if (points.length > 1) map.fitBounds(points, { padding: [60, 60], maxZoom: 15 })
  }, [points, map])
  return null
}

function markerIcon(color) {
  return L.divIcon({
    className: '',
    html: `<div style="width:16px;height:16px;border-radius:50%;background:${color};border:2.5px solid #0b0e14;box-shadow:0 0 0 2px ${color},0 0 10px ${color}"></div>`,
    iconSize: [16, 16], iconAnchor: [8, 8],
  })
}

export default function DeviceMap({ devices, subtitle }) {
  const positioned = useMemo(
    () => devices.filter(d => d.last_lat != null && d.last_lon != null),
    [devices],
  )
  const points = useMemo(() => positioned.map(d => [d.last_lat, d.last_lon]), [positioned])

  return (
    <MapContainer center={[20, 0]} zoom={2} zoomControl={false} style={{ width: '100%', height: '100%' }}>
      <TileLayer
        attribution='&copy; OpenStreetMap &copy; CARTO'
        url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
        subdomains="abcd"
      />
      <FitBounds points={points} />
      {positioned.map(d => {
        const color = STATUS_COLOR[d.status] ?? STATUS_COLOR.unknown
        return (
          <Marker key={d.id} position={[d.last_lat, d.last_lon]} icon={markerIcon(color)}>
            <Popup>
              <strong>{d.name}</strong><br />
              {subtitle?.(d)}
              Status: <span style={{ color }}>{d.status}</span><br />
              {d.last_speed != null && <>Speed: {Math.round(d.last_speed)} km/h<br /></>}
              {d.last_recorded_at && <>Last seen: {new Date(d.last_recorded_at).toLocaleTimeString()}</>}
            </Popup>
          </Marker>
        )
      })}
    </MapContainer>
  )
}
