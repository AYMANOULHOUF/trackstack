import { useLiveDevices } from '../useDevices'
import DeviceMap from '../components/DeviceMap'
import { C, Card, Dot, STATUS_COLOR, STATUS_LABEL } from '../ui'

export default function OrgMapPage() {
  const devices = useLiveDevices()

  return (
    <div style={{ position: 'absolute', inset: 0 }}>
      <DeviceMap devices={devices} />
      <Card style={{ position: 'absolute', top: 16, left: 16, width: 280, maxHeight: 'calc(100% - 32px)',
        overflowY: 'auto', padding: 16, zIndex: 500 }}>
        <h2 style={{ margin: '0 0 4px', fontSize: 15, fontWeight: 700 }}>Your devices</h2>
        <p style={{ margin: '0 0 14px', color: C.muted, fontSize: 12 }}>{devices.length} total</p>
        {devices.length === 0 && <p style={{ color: C.muted, fontSize: 13 }}>
          No devices assigned to you yet. The administrator assigns devices to your organization.
        </p>}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {devices.map(d => (
            <div key={d.id} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '9px 10px',
              borderRadius: 12, background: C.surface2 }}>
              <Dot color={STATUS_COLOR[d.status] ?? STATUS_COLOR.unknown} />
              <div style={{ minWidth: 0 }}>
                <div style={{ fontSize: 14, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{d.name}</div>
                <div style={{ fontSize: 12, color: C.muted }}>{STATUS_LABEL[d.status] ?? d.status}{d.last_speed != null ? ` · ${Math.round(d.last_speed)} km/h` : ''}</div>
              </div>
            </div>
          ))}
        </div>
      </Card>
    </div>
  )
}
