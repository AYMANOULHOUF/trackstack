import { useEffect, useMemo, useState } from 'react'
import { useQuery, useQueries } from '@tanstack/react-query'
import api from '../api'
import { useLiveDevices } from '../useDevices'
import { C, Card, Chip, Dot, Select, Stat, STATUS_COLOR, STATUS_LABEL } from '../ui'
import { LineChart, Donut } from '../components/charts'
import TrailMap from '../components/TrailMap'

// ------------------------------------------------------------------ helpers

const RANGES = [
  ['3h', 'Last 3 h', 3 * 3600e3],
  ['24h', 'Last 24 h', 24 * 3600e3],
  ['7d', 'Last 7 days', 7 * 86400e3],
]

function ago(iso) {
  if (!iso) return 'never'
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000)
  if (s < 60) return `${Math.round(s)} s ago`
  if (s < 3600) return `${Math.round(s / 60)} min ago`
  if (s < 86400) return `${Math.round(s / 3600)} h ago`
  return `${Math.round(s / 86400)} d ago`
}

function clock(iso) {
  return new Date(iso).toLocaleString([], {
    month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit',
  })
}

function haversine(a, b) {
  const R = 6371000
  const dLat = (b.lat - a.lat) * Math.PI / 180
  const dLon = (b.lon - a.lon) * Math.PI / 180
  const la1 = a.lat * Math.PI / 180
  const la2 = b.lat * Math.PI / 180
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(la1) * Math.cos(la2) * Math.sin(dLon / 2) ** 2
  return 2 * R * Math.asin(Math.min(1, Math.sqrt(h)))
}

function speedKmh(p) {
  // The API contract is km/h (speed limits and alerts are km/h).
  return p.speed ?? 0
}

// ------------------------------------------------------------------ pieces

function CardHead({ label, title, right }) {
  return (
    <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, padding: '14px 16px 0' }}>
      <div style={{ minWidth: 0 }}>
        <div style={{ fontSize: 10.5, letterSpacing: .8, textTransform: 'uppercase', color: C.muted,
          fontWeight: 700, marginBottom: 2 }}>{label}</div>
        <div style={{ fontSize: 14, fontWeight: 700, whiteSpace: 'nowrap', overflow: 'hidden',
          textOverflow: 'ellipsis' }}>{title}</div>
      </div>
      <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>{right}</div>
    </div>
  )
}

function DeviceGlyph({ status }) {
  const color = STATUS_COLOR[status] ?? C.accent
  return (
    <div style={{ width: 116, height: 116, position: 'relative', flexShrink: 0 }}>
      <div style={{ position: 'absolute', inset: 0, borderRadius: '50%',
        border: `1px dashed ${C.border}` }} />
      <div style={{ position: 'absolute', inset: 12, borderRadius: '50%',
        background: `radial-gradient(circle at 50% 40%, ${color}22, transparent 70%)`,
        border: `1px solid ${color}44` }} />
      <svg viewBox="0 0 24 24" width="48" height="48"
        style={{ position: 'absolute', left: 34, top: 34 }}>
        <path d="M12 2a7 7 0 0 0-7 7c0 5.25 7 13 7 13s7-7.75 7-13a7 7 0 0 0-7-7z"
          fill="none" stroke={color} strokeWidth="1.6" strokeLinejoin="round" />
        <circle cx="12" cy="9" r="2.4" fill={color} />
      </svg>
      <div style={{ position: 'absolute', left: '50%', bottom: 6, transform: 'translateX(-50%)',
        width: 8, height: 8, borderRadius: '50%', background: color,
        boxShadow: `0 0 0 3px ${color}22, 0 0 14px ${color}` }} />
    </div>
  )
}

function StatusPill({ status, tracking }) {
  const color = STATUS_COLOR[status] ?? C.muted
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 7, padding: '5px 11px',
      borderRadius: 999, background: `${color}17`, border: `1px solid ${color}3D`,
      color, fontSize: 12, fontWeight: 700 }}>
      <Dot color={color} pulse={status === 'moving'} />
      {STATUS_LABEL[status] ?? status}
      {!tracking && status !== 'paused' && <span style={{ color: C.muted, fontWeight: 600 }}>· off</span>}
    </span>
  )
}

// ------------------------------------------------------------------ page

export default function DashboardPage() {
  const devices = useLiveDevices()
  const [deviceId, setDeviceId] = useState('')
  const [range, setRange] = useState('24h')

  useEffect(() => {
    if (!deviceId && devices.length) setDeviceId(devices[0].id)
    if (deviceId && devices.length && !devices.some(d => d.id === deviceId)) {
      setDeviceId(devices[0].id)
    }
  }, [devices, deviceId])

  const device = devices.find(d => d.id === deviceId)
  const rangeMs = (RANGES.find(r => r[0] === range) ?? RANGES[1])[2]
  const fromIso = useMemo(() => new Date(Date.now() - rangeMs).toISOString(), [rangeMs])

  const { data: rawPositions = [], isLoading } = useQuery({
    queryKey: ['dash-positions', deviceId, fromIso],
    queryFn: () => api.get('/positions', { params: { device_id: deviceId, from: fromIso } })
      .then(r => r.data),
    enabled: !!deviceId,
    refetchInterval: 20_000,
  })

  const { data: trips = [] } = useQuery({
    queryKey: ['dash-trips', deviceId, fromIso],
    queryFn: () => api.get('/trips', { params: { device_id: deviceId, from: fromIso } })
      .then(r => r.data),
    enabled: !!deviceId,
  })

  const { data: geofences = [] } = useQuery({
    queryKey: ['geofences'],
    queryFn: () => api.get('/geofences').then(r => r.data),
  })

  const gfQueries = useQueries({
    queries: geofences.slice(0, 8).map(g => ({
      queryKey: ['dash-gf-events', g.id],
      queryFn: () => api.get(`/geofences/${g.id}/events`)
        .then(r => r.data.map(e => ({ ...e, fence: g.name }))),
      staleTime: 60_000,
    })),
  })

  // Ascending copy for the chart, map and distance maths.
  const points = useMemo(
    () => [...rawPositions].sort((a, b) => new Date(a.recorded_at) - new Date(b.recorded_at)),
    [rawPositions],
  )

  const chartPoints = useMemo(() => points.map(p => ({
    t: new Date(p.recorded_at).getTime(),
    speed: speedKmh(p),
    accuracy: p.accuracy ?? null,
  })), [points])

  const distanceM = useMemo(() => {
    let total = 0
    for (let i = 1; i < points.length; i++) {
      const dt = (new Date(points[i].recorded_at) - new Date(points[i - 1].recorded_at)) / 1000
      if (dt > 0 && dt <= 600) total += haversine(points[i - 1], points[i])
    }
    return total
  }, [points])

  const distribution = useMemo(() => {
    const stopSpeed = device?.trip_stop_speed_kmh ?? 2
    const offlineAfter = device?.offline_after_seconds ?? 300
    let moving = 0, idle = 0, silent = 0
    for (let i = 1; i < points.length; i++) {
      const dt = (new Date(points[i].recorded_at) - new Date(points[i - 1].recorded_at)) / 1000
      if (dt <= 0) continue
      if (dt > offlineAfter) silent += dt
      else if (speedKmh(points[i - 1]) > stopSpeed) moving += dt
      else idle += dt
    }
    return { moving, idle, silent, total: moving + idle + silent }
  }, [points, device])

  const events = useMemo(() => {
    const out = []
    const cutoff = Date.now() - rangeMs

    for (const q of gfQueries) {
      for (const e of q.data ?? []) {
        if (e.device_id !== deviceId) continue
        const ts = new Date(e.occurred_at).getTime()
        if (ts < cutoff) continue
        out.push({
          id: `gf-${e.id}`, ts, kind: e.type === 'enter' ? 'good' : 'warn',
          title: e.type === 'enter' ? `Entered ${e.fence}` : `Left ${e.fence}`,
          detail: 'Geofence',
        })
      }
    }

    for (const t of trips) {
      const start = new Date(t.start_at).getTime()
      if (start >= cutoff) {
        out.push({ id: `trip-s-${t.id}`, ts: start, kind: 'neutral',
          title: 'Trip started', detail: 'Movement' })
      }
      if (t.end_at) {
        const end = new Date(t.end_at).getTime()
        if (end >= cutoff) {
          out.push({ id: `trip-e-${t.id}`, ts: end, kind: 'neutral',
            title: t.distance_m != null ? `Trip ended · ${(t.distance_m / 1000).toFixed(1)} km` : 'Trip ended',
            detail: 'Movement' })
        }
      }
    }

    const limit = device?.speed_limit_kmh
    if (limit != null) {
      let lastAlert = 0
      for (const p of points) {
        const v = speedKmh(p)
        const ts = new Date(p.recorded_at).getTime()
        if (v > limit && ts - lastAlert > 5 * 60e3) {
          lastAlert = ts
          out.push({ id: `spd-${p.id}`, ts, kind: 'bad',
            title: `Speed limit exceeded · ${Math.round(v)} km/h`, detail: `Limit ${Math.round(limit)} km/h` })
        }
      }
    }

    if (device?.status === 'offline' && device.last_recorded_at) {
      out.push({
        id: 'offline', ts: new Date(device.last_recorded_at).getTime() + (device.offline_after_seconds ?? 300) * 1000,
        kind: 'bad', title: 'Signal lost', detail: 'No position received',
      })
    }

    return out.sort((a, b) => b.ts - a.ts).slice(0, 40)
  }, [gfQueries, trips, points, device, deviceId, rangeMs])

  const last = points[points.length - 1]
  const lastSpeed = device?.last_speed != null ? device.last_speed : (last ? speedKmh(last) : null)
  const overLimit = device?.speed_limit_kmh != null && lastSpeed != null && lastSpeed > device.speed_limit_kmh
  const noSignal = device?.status === 'offline'

  const kindColor = { good: C.ok, warn: C.warn, bad: C.danger, neutral: C.muted }

  const fmtDur = s => s >= 3600 ? `${(s / 3600).toFixed(1)}` : `${Math.round(s / 60)}`

  return (
    <div style={{ height: '100%', overflowY: 'auto', padding: 16 }}>
      <div style={{ maxWidth: 1480, margin: '0 auto' }}>

        {/* toolbar */}
        <div style={{ display: 'flex', alignItems: 'flex-end', gap: 14, flexWrap: 'wrap', marginBottom: 14 }}>
          <div>
            <div style={{ fontSize: 11, letterSpacing: .9, textTransform: 'uppercase', color: C.accent,
              fontWeight: 800, marginBottom: 3 }}>Device overview</div>
            <h1 style={{ margin: 0, fontSize: 22, fontWeight: 800, letterSpacing: -.2 }}>
              Telemetry &amp; location
            </h1>
            <div style={{ color: C.muted, fontSize: 12, marginTop: 2 }}>
              Any GPS source — phones, asset tags, vehicle units. Same view for all of them.
            </div>
          </div>

          <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}
            className="dash-controls">
            <Select value={deviceId} onChange={e => setDeviceId(e.target.value)} style={{ minWidth: 190 }}>
              {devices.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}
              {devices.length === 0 && <option value="">No devices</option>}
            </Select>

            <div style={{ display: 'flex', background: C.surface, border: `1px solid ${C.border}`,
              borderRadius: 8, padding: 2 }}>
              {RANGES.map(([key, label]) => (
                <button key={key} onClick={() => setRange(key)}
                  title={label}
                  style={{ border: 'none', cursor: 'pointer', padding: '6px 11px', borderRadius: 6,
                    fontSize: 12, fontWeight: 700,
                    background: range === key ? C.surface2 : 'transparent',
                    color: range === key ? C.text : C.muted }}>
                  {key}
                </button>
              ))}
            </div>
          </div>
        </div>

        {devices.length === 0 && (
          <Card style={{ padding: 22 }}>
            <div style={{ fontWeight: 700, marginBottom: 4 }}>No devices yet</div>
            <div style={{ color: C.muted, fontSize: 13 }}>
              An administrator has to assign a device to your organization. Phones create
              themselves the first time the app opens; they appear here once assigned.
            </div>
          </Card>
        )}

        {devices.length > 0 && (
          <div className="dash-grid">

            {/* status */}
            <Card className="dash-span-4" style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <div style={{ fontSize: 13, fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap' }}>{device?.name ?? 'Device'}</div>
                <Chip style={{ marginLeft: 'auto', textTransform: 'uppercase', letterSpacing: .6 }}>
                  {device?.type ?? 'gps'}
                </Chip>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
                <DeviceGlyph status={device?.status} />
                <div style={{ minWidth: 0 }}>
                  <StatusPill status={device?.status} tracking={device?.tracking_active} />
                  <div style={{ color: C.muted, fontSize: 12, marginTop: 8 }}>
                    Last fix {ago(device?.last_recorded_at)}
                  </div>
                  <div style={{ color: C.muted, fontSize: 11.5, marginTop: 2,
                    fontVariantNumeric: 'tabular-nums' }}>
                    {device?.last_lat != null
                      ? `${device.last_lat.toFixed(5)}, ${device.last_lon.toFixed(5)}`
                      : 'No coordinates yet'}
                  </div>
                </div>
              </div>

              {(noSignal || overLimit) && (
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '8px 10px',
                  borderRadius: 8, background: 'rgba(248,81,73,.10)', border: '1px solid rgba(248,81,73,.35)',
                  color: C.danger, fontSize: 12, fontWeight: 600 }}>
                  <span style={{ fontSize: 14 }}>▲</span>
                  {noSignal
                    ? `No signal — last fix ${ago(device?.last_recorded_at)}`
                    : `Over the ${Math.round(device.speed_limit_kmh)} km/h limit`}
                </div>
              )}

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 10,
                borderTop: `1px solid ${C.border}`, paddingTop: 12 }}>
                <Stat label="Speed" value={lastSpeed != null ? Math.round(lastSpeed) : '—'} unit="km/h" />
                <Stat label="Distance" value={(distanceM / 1000).toFixed(1)} unit="km" />
                <Stat label="Fixes" value={points.length} hint={range} />
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: 10, color: C.muted, fontSize: 11.5,
                borderTop: `1px solid ${C.border}`, paddingTop: 10 }}>
                <span>Accuracy {last?.accuracy != null ? `${Math.round(last.accuracy)} m` : '—'}</span>
                <span style={{ opacity: .5 }}>·</span>
                <span>{device?.offline_after_seconds ? `Offline after ${device.offline_after_seconds}s` : 'Live'}</span>
              </div>
            </Card>

            {/* telemetry chart */}
            <Card className="dash-span-8" style={{ padding: 16, display: 'flex', flexDirection: 'column' }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, marginBottom: 6 }}>
                <div>
                  <div style={{ fontSize: 10.5, letterSpacing: .8, textTransform: 'uppercase', color: C.muted,
                    fontWeight: 700 }}>Telemetry</div>
                  <div style={{ fontSize: 14, fontWeight: 700 }}>Speed &amp; fix accuracy</div>
                </div>
                <div style={{ marginLeft: 'auto', display: 'flex', gap: 14, alignItems: 'center' }}>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 11.5, color: C.muted }}>
                    <span style={{ width: 14, height: 2, background: C.accent, borderRadius: 2 }} />
                    Speed km/h
                  </span>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 11.5, color: C.muted }}>
                    <span style={{ width: 14, height: 2, background: C.muted, borderRadius: 2 }} />
                    Accuracy m
                  </span>
                </div>
              </div>

              {isLoading
                ? <div className="skeleton" style={{ height: 200, marginTop: 6 }} />
                : (
                  <LineChart
                    points={chartPoints}
                    height={200}
                    series={[
                      { key: 'speed', color: C.accent, width: 2 },
                      { key: 'accuracy', color: C.muted, width: 1.4, dash: '4 3' },
                    ]}
                  />
                )}

              <div style={{ display: 'flex', gap: 18, marginTop: 8, color: C.muted, fontSize: 11.5,
                borderTop: `1px solid ${C.border}`, paddingTop: 10 }}>
                <span>Peak {chartPoints.length ? Math.round(Math.max(...chartPoints.map(p => p.speed))) : 0} km/h</span>
                <span>Avg {chartPoints.length
                  ? Math.round(chartPoints.reduce((s, p) => s + p.speed, 0) / chartPoints.length)
                  : 0} km/h</span>
                <span style={{ marginLeft: 'auto' }}>
                  {points.length ? `${points.length} fixes · ${clock(points[0].recorded_at)} → ${clock(points[points.length - 1].recorded_at)}` : 'No data'}
                </span>
              </div>
            </Card>

            {/* map */}
            <Card className="dash-span-12" style={{ padding: 14 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 10 }}>
                <div>
                  <div style={{ fontSize: 10.5, letterSpacing: .8, textTransform: 'uppercase', color: C.muted,
                    fontWeight: 700 }}>Location</div>
                  <div style={{ fontSize: 14, fontWeight: 700 }}>{device?.name ?? 'Device'} in real time</div>
                </div>
                <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Chip color={last ? C.accent : C.muted}>
                    <Dot color={last ? C.accent : C.muted} size={7} /> {last ? 'live trail' : 'no fixes'}
                  </Chip>
                </div>
              </div>
              <TrailMap device={device} points={points} height={320} />
            </Card>

            {/* events */}
            <Card className="dash-span-7" style={{ padding: '14px 0 0', display: 'flex', flexDirection: 'column' }}>
              <CardHead label="Activity" title="Events" right={
                <Chip>{events.length}</Chip>
              } />
              <div style={{ overflow: 'auto', maxHeight: 320, marginTop: 10 }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12.5 }}>
                  <thead>
                    <tr style={{ color: C.muted, textAlign: 'left', fontSize: 10.5,
                      textTransform: 'uppercase', letterSpacing: .6 }}>
                      <th style={{ padding: '6px 16px', fontWeight: 700 }}>Event</th>
                      <th style={{ padding: '6px 10px', fontWeight: 700 }}>Type</th>
                      <th style={{ padding: '6px 16px', fontWeight: 700, textAlign: 'right' }}>When</th>
                    </tr>
                  </thead>
                  <tbody>
                    {events.map(e => (
                      <tr key={e.id} style={{
                        background: e.kind === 'bad' ? 'rgba(248,81,73,.07)' : 'transparent',
                        borderTop: `1px solid ${C.border}`,
                      }}>
                        <td style={{ padding: '8px 16px' }}>
                          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
                            <span style={{ width: 3, height: 14, borderRadius: 2,
                              background: kindColor[e.kind] }} />
                            <span style={{ fontWeight: 600 }}>{e.title}</span>
                          </span>
                        </td>
                        <td style={{ padding: '8px 10px', color: C.muted }}>{e.detail}</td>
                        <td style={{ padding: '8px 16px', color: C.muted, textAlign: 'right',
                          fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' }}>
                          {clock(new Date(e.ts).toISOString())}
                        </td>
                      </tr>
                    ))}
                    {events.length === 0 && (
                      <tr><td colSpan={3} style={{ padding: '26px 16px', color: C.muted, textAlign: 'center' }}>
                        Nothing happened in this window. Events appear when the device moves,
                        crosses a geofence or breaks a speed limit.
                      </td></tr>
                    )}
                  </tbody>
                </table>
              </div>
            </Card>

            {/* distribution */}
            <Card className="dash-span-5" style={{ padding: 16 }}>
              <CardHead label="Uptime" title="Time distribution" right={<Chip>{range}</Chip>} />
              <div style={{ display: 'flex', alignItems: 'center', gap: 18, marginTop: 14 }}>
                <Donut
                  size={158}
                  slices={[
                    { label: 'Moving', value: distribution.moving, color: C.ok },
                    { label: 'Stationary', value: distribution.idle, color: C.warn },
                    { label: 'No signal', value: distribution.silent, color: C.muted },
                  ]}
                  centerLabel={distribution.total > 3600 ? 'hours' : 'minutes'}
                  centerValue={distribution.total > 0 ? fmtDur(distribution.total) : '0'}
                />
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10, flex: 1, minWidth: 0 }}>
                  {[
                    ['Moving', distribution.moving, C.ok],
                    ['Stationary', distribution.idle, C.warn],
                    ['No signal', distribution.silent, C.muted],
                  ].map(([label, value, color]) => (
                    <div key={label}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 7, fontSize: 12 }}>
                        <span style={{ width: 8, height: 8, borderRadius: 2, background: color }} />
                        <span style={{ fontWeight: 600 }}>{label}</span>
                        <span style={{ marginLeft: 'auto', color: C.muted, fontVariantNumeric: 'tabular-nums' }}>
                          {distribution.total ? `${Math.round(value / distribution.total * 100)}%` : '—'}
                        </span>
                      </div>
                      <div style={{ color: C.muted, fontSize: 11, marginLeft: 15 }}>
                        {value > 3600 ? `${(value / 3600).toFixed(1)} h` : `${Math.round(value / 60)} min`}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
              <div style={{ marginTop: 14, paddingTop: 10, borderTop: `1px solid ${C.border}`,
                color: C.muted, fontSize: 11.5 }}>
                Estimated from fix intervals. Gaps longer than the device's offline threshold count as no signal.
              </div>
            </Card>
          </div>
        )}
      </div>
    </div>
  )
}
