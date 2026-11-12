import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { MapContainer, TileLayer, Polyline, CircleMarker, useMap } from 'react-leaflet'
import 'leaflet/dist/leaflet.css'
import api from '../api'
import { TILES } from '../mapTiles'
import { C, Card, Field, Chip } from '../ui'

function FitLine({ line }) {
  const map = useMap()
  useEffect(() => {
    if (line.length === 1) map.setView(line[0], 15)
    else if (line.length > 1) map.fitBounds(line, { padding: [50, 50], maxZoom: 16 })
  }, [line, map])
  return null
}

function TripMap({ deviceId, trip }) {
  const { data: points = [] } = useQuery({
    queryKey: ['trip-points', trip.id],
    queryFn: () => api.get('/positions', {
      params: { device_id: deviceId, from: trip.start_at, to: trip.end_at ?? undefined },
    }).then(r => r.data),
  })

  const line = useMemo(() => [...points].reverse().map(p => [p.lat, p.lon]), [points])

  return (
    <MapContainer center={[31.79, -7.09]} zoom={6} zoomControl={false} style={{ width: '100%', height: '100%' }}>
      <TileLayer attribution={TILES.dark.attribution} url={TILES.dark.url} maxNativeZoom={TILES.dark.maxNativeZoom} maxZoom={19} />
      <FitLine line={line} />
      {line.length > 1 && <Polyline positions={line} pathOptions={{ color: C.accent, weight: 4 }} />}
      {line.length > 0 && <CircleMarker center={line[0]} radius={7}
        pathOptions={{ color: C.ok, fillColor: C.ok, fillOpacity: 1 }} />}
      {line.length > 1 && <CircleMarker center={line[line.length - 1]} radius={7}
        pathOptions={{ color: C.danger, fillColor: C.danger, fillOpacity: 1 }} />}
    </MapContainer>
  )
}

function fmtDuration(start, end) {
  if (!end) return 'in progress'
  const seconds = Math.max(0, (new Date(end) - new Date(start)) / 1000)
  const h = Math.floor(seconds / 3600)
  const m = Math.round((seconds % 3600) / 60)
  return h > 0 ? `${h} h ${m} min` : `${m} min`
}

export default function TripsPage() {
  const [deviceId, setDeviceId] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [selected, setSelected] = useState(null)

  const { data: devices = [] } = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.get('/devices').then(r => r.data),
  })

  useEffect(() => {
    if (!deviceId && devices.length > 0) setDeviceId(devices[0].id)
  }, [devices, deviceId])

  const { data: trips = [], isLoading } = useQuery({
    queryKey: ['trips', deviceId, from, to],
    queryFn: () => api.get('/trips', {
      params: {
        device_id: deviceId,
        from: from ? new Date(from + 'T00:00:00').toISOString() : undefined,
        to: to ? new Date(to + 'T23:59:59').toISOString() : undefined,
      },
    }).then(r => r.data),
    enabled: !!deviceId,
  })

  return (
    <div style={{ display: 'flex', height: '100%' }}>
      <div style={{ width: 380, flexShrink: 0, padding: 22, overflowY: 'auto', borderRight: `1px solid ${C.border}` }}>
        <h1 style={{ fontSize: 20, fontWeight: 800, margin: '0 0 4px' }}>Trips</h1>
        <p style={{ color: C.muted, fontSize: 12, margin: '0 0 18px' }}>
          Segmented automatically from position fixes using each device's stop thresholds.
        </p>

        <label style={{ fontSize: 12, color: C.muted, fontWeight: 600 }}>Device</label>
        <select value={deviceId} onChange={e => { setDeviceId(e.target.value); setSelected(null) }}
          style={{ width: '100%', margin: '6px 0 14px', padding: '10px 12px', borderRadius: 12,
            background: C.surface2, border: `1px solid ${C.border}`, color: C.text, fontSize: 14 }}>
          {devices.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}
          {devices.length === 0 && <option value="">No devices</option>}
        </select>

        <div style={{ display: 'flex', gap: 8, marginBottom: 18 }}>
          <div style={{ flex: 1 }}>
            <label style={{ fontSize: 12, color: C.muted, fontWeight: 600 }}>From</label>
            <Field type="date" value={from} onChange={e => setFrom(e.target.value)} style={{ marginTop: 6 }} />
          </div>
          <div style={{ flex: 1 }}>
            <label style={{ fontSize: 12, color: C.muted, fontWeight: 600 }}>To</label>
            <Field type="date" value={to} onChange={e => setTo(e.target.value)} style={{ marginTop: 6 }} />
          </div>
        </div>

        {isLoading ? <p style={{ color: C.muted }}>Loading…</p> : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {trips.map(t => (
              <div key={t.id} onClick={() => setSelected(t)}
                style={{ padding: '12px 14px', borderRadius: 12, cursor: 'pointer',
                  background: selected?.id === t.id ? C.accentSoft : C.surface2,
                  border: `1px solid ${selected?.id === t.id ? C.accent : C.border}` }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                  <span style={{ fontSize: 13, fontWeight: 600 }}>
                    {new Date(t.start_at).toLocaleString()}
                  </span>
                  {!t.end_at && <Chip color={C.ok} style={{ marginLeft: 'auto' }}>live</Chip>}
                </div>
                <div style={{ fontSize: 12, color: C.muted }}>
                  {fmtDuration(t.start_at, t.end_at)}
                  {t.distance_m != null ? ` · ${(t.distance_m / 1000).toFixed(1)} km` : ''}
                </div>
              </div>
            ))}
            {trips.length === 0 && <p style={{ color: C.muted, fontSize: 13 }}>
              No trips yet. The worker runs every 30 seconds; a trip starts when the device moves above its stop speed.
            </p>}
          </div>
        )}
      </div>

      <div style={{ flex: 1, position: 'relative' }}>
        {selected ? <TripMap key={selected.id} deviceId={deviceId} trip={selected} /> : (
          <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Card style={{ padding: 22, maxWidth: 340, textAlign: 'center' }}>
              <p style={{ margin: 0, color: C.muted, fontSize: 13 }}>
                Select a trip to see its track on the map.
              </p>
            </Card>
          </div>
        )}
      </div>
    </div>
  )
}
