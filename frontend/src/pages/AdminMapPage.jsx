import { useMemo } from 'react'
import { useLiveDevices } from '../useDevices'
import DeviceMap from '../components/DeviceMap'
import { C, Card, Chip, Dot, STATUS_COLOR, STATUS_LABEL } from '../ui'

export default function AdminMapPage() {
  const devices = useLiveDevices()

  const groups = useMemo(() => {
    const g = new Map()
    const unassigned = []
    for (const d of devices) {
      if (!d.orgs || d.orgs.length === 0) { unassigned.push(d); continue }
      for (const o of d.orgs) {
        if (!g.has(o.name)) g.set(o.name, [])
        g.get(o.name).push(d)
      }
    }
    return { g: [...g.entries()].sort((a, b) => a[0].localeCompare(b[0])), unassigned }
  }, [devices])

  return (
    <div style={{ position: 'absolute', inset: 0 }}>
      <DeviceMap devices={devices} subtitle={d => (
        <>{(d.orgs ?? []).length ? d.orgs.map(o => o.name).join(', ') : 'Unassigned'}<br /></>
      )} />
      <Card style={{ position: 'absolute', top: 16, left: 16, width: 300, maxHeight: 'calc(100% - 32px)',
        overflowY: 'auto', padding: 16, zIndex: 500 }}>
        <h2 style={{ margin: '0 0 14px', fontSize: 15, fontWeight: 700 }}>All devices ({devices.length})</h2>
        {groups.g.map(([orgName, list]) => <Section key={orgName} title={orgName} list={list} />)}
        {groups.unassigned.length > 0 && <Section title="Unassigned" list={groups.unassigned} accent={C.warn} />}
        {devices.length === 0 && <p style={{ color: C.muted, fontSize: 13 }}>No devices yet.</p>}
      </Card>
    </div>
  )
}

function Section({ title, list, accent }) {
  return (
    <div style={{ marginBottom: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, margin: '0 0 8px' }}>
        <span style={{ fontSize: 12, fontWeight: 700, color: accent ?? C.accent, textTransform: 'uppercase', letterSpacing: .6 }}>{title}</span>
        <Chip>{list.length}</Chip>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        {list.map(d => (
          <div key={d.id + title} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', borderRadius: 12, background: C.surface2 }}>
            <Dot color={STATUS_COLOR[d.status] ?? STATUS_COLOR.unknown} />
            <div style={{ minWidth: 0 }}>
              <div style={{ fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{d.name}</div>
              <div style={{ fontSize: 11, color: C.muted }}>{STATUS_LABEL[d.status] ?? d.status}{d.last_speed != null ? ` · ${Math.round(d.last_speed)} km/h` : ''}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
