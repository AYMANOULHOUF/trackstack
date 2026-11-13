import { useEffect, useMemo, useRef } from 'react'
import { MapContainer, TileLayer, Polyline, CircleMarker, Marker, Tooltip, useMap } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { C, STATUS_COLOR } from '../ui'
import { TILES } from '../mapTiles'

function Fit({ line }) {
  const map = useMap()
  const fitted = useRef(false)
  useEffect(() => {
    if (fitted.current || line.length === 0) return
    // fitBounds before the card has its final size silently lands on the world
    // view, so re-measure and fit once the layout settles.
    const run = () => {
      map.invalidateSize()
      if (line.length === 1) map.setView(line[0], 16)
      else map.fitBounds(line, { padding: [36, 36], maxZoom: 16 })
    }
    run()
    const t = setTimeout(run, 300)
    fitted.current = true
    return () => clearTimeout(t)
  }, [line, map])
  return null
}

function Recener({ line }) {
  const map = useMap()
  return (
    <button
      onClick={() => {
        if (line.length === 1) map.setView(line[0], 16)
        else if (line.length > 1) map.fitBounds(line, { padding: [36, 36], maxZoom: 16 })
      }}
      title="Recentre on the device"
      style={{ width: 36, height: 36, borderRadius: 8, border: `1px solid ${C.border}`,
        background: C.surface, color: C.text, cursor: 'pointer', fontSize: 16,
        display: 'flex', alignItems: 'center', justifyContent: 'center' }}
    >◎</button>
  )
}

function pin(color) {
  return L.divIcon({
    className: '',
    html: `<div style="position:relative;width:18px;height:18px">
      <span style="position:absolute;inset:0;border-radius:50%;background:${color};
        border:2px solid #0D1117;box-shadow:0 0 0 4px ${color}2E,0 0 14px ${color}88"></span>
      <span style="position:absolute;inset:-7px;border-radius:50%;border:1px solid ${color}55;
        animation:pulse 2.4s ease-in-out infinite"></span>
    </div>`,
    iconSize: [18, 18],
    iconAnchor: [9, 9],
  })
}

export default function TrailMap({ device, points, height = 340 }) {
  const line = useMemo(() => points.map(p => [p.lat, p.lon]), [points])
  const color = STATUS_COLOR[device?.status] ?? C.accent
  const last = line[line.length - 1]

  return (
    <div style={{ height, borderRadius: 12, overflow: 'hidden', border: `1px solid ${C.border}`,
      position: 'relative' }}>
      <MapContainer
        key={device?.id ?? 'none'}
        center={last ?? [20, 0]}
        zoom={last ? 14 : 2}
        zoomControl={false}
        style={{ width: '100%', height: '100%' }}
      >
        <TileLayer attribution={TILES.dark.attribution} url={TILES.dark.url}
          maxNativeZoom={TILES.dark.maxNativeZoom} maxZoom={19} />
        <Fit line={line} />

        {line.length > 1 && (
          <Polyline positions={line} pathOptions={{ color: C.accent, weight: 3, opacity: 0.85 }} />
        )}
        {line.length > 0 && (
          <CircleMarker center={line[0]} radius={5}
            pathOptions={{ color: C.muted, fillColor: C.surface, fillOpacity: 1, weight: 2 }} />
        )}
        {last && (
          <Marker position={last} icon={pin(color)}>
            <Tooltip direction="top" offset={[0, -12]} opacity={1}>
              <span style={{ fontWeight: 600 }}>{device?.name}</span>
            </Tooltip>
          </Marker>
        )}

        <div style={{ position: 'absolute', right: 12, bottom: 12, zIndex: 500 }}>
          <Recener line={line} />
        </div>
      </MapContainer>
    </div>
  )
}
