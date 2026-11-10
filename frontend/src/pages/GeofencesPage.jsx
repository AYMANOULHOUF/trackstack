import { useEffect, useMemo, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { MapContainer, TileLayer, Polygon, Polyline, CircleMarker, useMapEvents, useMap } from 'react-leaflet'
import 'leaflet/dist/leaflet.css'
import api from '../api'
import { TILES } from '../mapTiles'
import { C, Card, Btn, Field, Chip, Modal } from '../ui'

function ringToLatLngs(geojson) {
  try {
    const g = JSON.parse(geojson)
    const ring = g?.coordinates?.[0] ?? []
    return ring.map(([lon, lat]) => [lat, lon])
  } catch {
    return []
  }
}

function FitGeofences({ polygons }) {
  const map = useMap()
  useEffect(() => {
    if (polygons.length === 0) return
    const all = polygons.flat()
    if (all.length > 0) map.fitBounds(all, { padding: [50, 50], maxZoom: 14 })
  }, [polygons, map])
  return null
}

function DrawLayer({ drawing, points, setPoints, geofences, onSelect }) {
  useMapEvents({
    click(e) {
      if (drawing) setPoints(prev => [...prev, [e.latlng.lat, e.latlng.lng]])
    },
  })

  return (
    <>
      {geofences.map(g => {
        const positions = ringToLatLngs(g.geojson)
        if (positions.length === 0) return null
        return (
          <Polygon
            key={g.id}
            positions={positions}
            interactive={!drawing}
            eventHandlers={{ click: () => onSelect(g) }}
            pathOptions={{ color: C.accent, weight: 2, fillColor: C.accent, fillOpacity: 0.12 }}
          />
        )
      })}
      {points.length > 0 && (
        <>
          <Polyline positions={points} pathOptions={{ color: C.warn, weight: 3, dashArray: '6 6' }} />
          {points.map((p, i) => (
            <CircleMarker key={i} center={p} radius={5}
              pathOptions={{ color: C.warn, fillColor: C.warn, fillOpacity: 1 }} />
          ))}
        </>
      )}
    </>
  )
}

export default function GeofencesPage() {
  const qc = useQueryClient()
  const [drawing, setDrawing] = useState(false)
  const [points, setPoints] = useState([])
  const [name, setName] = useState('')
  const [eventsFor, setEventsFor] = useState(null)

  const { data: geofences = [], isLoading } = useQuery({
    queryKey: ['geofences'],
    queryFn: () => api.get('/geofences').then(r => r.data),
  })

  const create = useMutation({
    mutationFn: ({ name, geojson }) => api.post('/geofences', { name, geojson }),
    onSuccess: () => {
      setDrawing(false)
      setPoints([])
      setName('')
      qc.invalidateQueries({ queryKey: ['geofences'] })
    },
  })

  const remove = useMutation({
    mutationFn: id => api.delete(`/geofences/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['geofences'] }),
  })

  const { data: events = [], isLoading: loadingEvents } = useQuery({
    queryKey: ['geofence-events', eventsFor?.id],
    queryFn: () => api.get(`/geofences/${eventsFor.id}/events`).then(r => r.data),
    enabled: !!eventsFor,
  })

  const polygons = useMemo(() => geofences.map(g => ringToLatLngs(g.geojson)).filter(p => p.length), [geofences])

  function finish() {
    if (points.length < 3 || !name.trim()) return
    const ring = points.map(([lat, lon]) => [lon, lat])
    ring.push(ring[0])
    create.mutate({ name: name.trim(), geojson: JSON.stringify({ type: 'Polygon', coordinates: [ring] }) })
  }

  function cancel() {
    setDrawing(false)
    setPoints([])
    setName('')
  }

  return (
    <div style={{ position: 'absolute', inset: 0 }}>
      <MapContainer center={[31.79, -7.09]} zoom={6} zoomControl={false} style={{ width: '100%', height: '100%' }}>
        <TileLayer attribution={TILES.dark.attribution} url={TILES.dark.url} subdomains={TILES.dark.subdomains} />
        <FitGeofences polygons={polygons} />
        <DrawLayer
          drawing={drawing}
          points={points}
          setPoints={setPoints}
          geofences={geofences}
          onSelect={setEventsFor}
        />
      </MapContainer>

      <Card style={{ position: 'absolute', top: 16, left: 16, width: 310, maxHeight: 'calc(100% - 32px)',
        overflowY: 'auto', padding: 16, zIndex: 500 }}>
        <h2 style={{ margin: '0 0 4px', fontSize: 15, fontWeight: 700 }}>Geofences</h2>
        <p style={{ margin: '0 0 14px', color: C.muted, fontSize: 12 }}>
          Click the map to place corners, then name and save the polygon.
        </p>

        {drawing ? (
          <div style={{ marginBottom: 16 }}>
            <Field placeholder="Geofence name" value={name} onChange={e => setName(e.target.value)} />
            <div style={{ color: C.muted, fontSize: 12, margin: '8px 0 12px' }}>
              {points.length} corner{points.length === 1 ? '' : 's'} placed
            </div>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              <Btn onClick={finish} disabled={points.length < 3 || !name.trim()}
                style={{ opacity: points.length < 3 || !name.trim() ? 0.5 : 1 }}>Save</Btn>
              <Btn variant="ghost" onClick={() => setPoints(prev => prev.slice(0, -1))}
                disabled={points.length === 0}>Undo</Btn>
              <Btn variant="ghost" onClick={cancel}>Cancel</Btn>
            </div>
          </div>
        ) : (
          <Btn variant="soft" onClick={() => setDrawing(true)} style={{ marginBottom: 16 }}>
            Draw geofence
          </Btn>
        )}

        {isLoading ? <p style={{ color: C.muted, fontSize: 13 }}>Loading…</p> : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {geofences.map(g => (
              <div key={g.id} style={{ padding: '10px 12px', borderRadius: 12, background: C.surface2 }}>
                <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 4 }}>{g.name}</div>
                <div style={{ fontSize: 11, color: C.muted, marginBottom: 8 }}>
                  {new Date(g.created_at).toLocaleDateString()}
                </div>
                <div style={{ display: 'flex', gap: 6 }}>
                  <Btn variant="ghost" onClick={() => setEventsFor(g)}
                    style={{ padding: '5px 10px', fontSize: 12 }}>Events</Btn>
                  <Btn variant="danger" onClick={() => confirm(`Delete ${g.name}?`) && remove.mutate(g.id)}
                    style={{ padding: '5px 10px', fontSize: 12 }}>Delete</Btn>
                </div>
              </div>
            ))}
            {geofences.length === 0 && <p style={{ color: C.muted, fontSize: 13 }}>No geofences yet.</p>}
          </div>
        )}
      </Card>

      <Modal open={!!eventsFor} onClose={() => setEventsFor(null)} title={eventsFor ? `Events · ${eventsFor.name}` : ''} width={520}>
        {loadingEvents ? <p style={{ color: C.muted }}>Loading…</p> : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, maxHeight: 360, overflowY: 'auto' }}>
            {events.map(e => (
              <div key={e.id} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '9px 11px',
                borderRadius: 10, background: C.surface2 }}>
                <Chip color={e.type === 'enter' ? C.ok : C.warn}>{e.type}</Chip>
                <span style={{ fontSize: 13 }}>{e.device_name}</span>
                <span style={{ marginLeft: 'auto', fontSize: 12, color: C.muted }}>
                  {new Date(e.occurred_at).toLocaleString()}
                </span>
              </div>
            ))}
            {events.length === 0 && <p style={{ color: C.muted, fontSize: 13 }}>No enter/exit events yet.</p>}
          </div>
        )}
      </Modal>
    </div>
  )
}
