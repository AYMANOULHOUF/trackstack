import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../api'
import { C, Card, Btn, Field, Chip, Dot, Modal, STATUS_COLOR, STATUS_LABEL } from '../ui'

export default function AdminDevicesPage() {
  const qc = useQueryClient()
  const [assign, setAssign] = useState(null)
  const [rename, setRename] = useState(null)

  const { data: devices = [], isLoading, isFetching } = useQuery({
    queryKey: ['devices'], queryFn: () => api.get('/devices').then(r => r.data), refetchInterval: 20_000,
  })

  // Force an immediate refresh of the device list (e.g. after deleting a bugged
  // device so a fresh re-enroll shows up without waiting for the 20s poll).
  const rescan = () => {
    qc.invalidateQueries({ queryKey: ['devices'] })
    qc.invalidateQueries({ queryKey: ['orgs'] })
  }
  const { data: orgs = [] } = useQuery({ queryKey: ['orgs'], queryFn: () => api.get('/orgs').then(r => r.data) })

  const saveAssign = useMutation({
    mutationFn: ({ id, orgIds }) => api.put(`/devices/${id}/orgs`, { org_ids: [...orgIds] }),
    onSuccess: () => { setAssign(null); qc.invalidateQueries({ queryKey: ['devices'] }); qc.invalidateQueries({ queryKey: ['orgs'] }) },
  })
  const saveRename = useMutation({
    mutationFn: ({ id, name }) => api.patch(`/devices/${id}`, { name }),
    onSuccess: () => { setRename(null); qc.invalidateQueries({ queryKey: ['devices'] }) },
  })
  const del = useMutation({
    mutationFn: (id) => api.delete(`/devices/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['devices'] }),
  })

  const unassigned = devices.filter(d => !d.assigned)
  const assigned = devices.filter(d => d.assigned)
  const toggle = (set, id) => { const n = new Set(set); n.has(id) ? n.delete(id) : n.add(id); return n }

  const Row = (d) => (
    <Card key={d.id} style={{ padding: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8 }}>
        <Dot color={STATUS_COLOR[d.status] ?? STATUS_COLOR.unknown} />
        <div style={{ fontSize: 16, fontWeight: 700 }}>{d.name}</div>
        {!d.assigned && <Chip color={C.warn} style={{ marginLeft: 'auto' }}>Pending</Chip>}
      </div>
      <div style={{ color: C.muted, fontSize: 12, marginBottom: 10 }}>
        {d.type} · {STATUS_LABEL[d.status] ?? d.status}{d.last_speed != null ? ` · ${Math.round(d.last_speed)} km/h` : ''}
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 14, minHeight: 22 }}>
        {(d.orgs ?? []).map(o => <Chip key={o.id} color={C.accent}>{o.name}</Chip>)}
        {(d.orgs ?? []).length === 0 && <span style={{ color: C.muted, fontSize: 12 }}>Not in any organization</span>}
      </div>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <Btn onClick={() => setAssign({ device: d, orgIds: new Set((d.orgs ?? []).map(o => o.id)) })}>Assign</Btn>
        <Btn variant="ghost" onClick={() => setRename({ id: d.id, name: d.name })}>Rename</Btn>
        <Btn variant="danger" onClick={() => confirm(`Delete ${d.name} everywhere?`) && del.mutate(d.id)}>Delete</Btn>
      </div>
    </Card>
  )

  return (
    <div style={{ padding: 28, overflowY: 'auto', height: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12, marginBottom: 22 }}>
        <div>
          <h1 style={{ fontSize: 22, fontWeight: 800, margin: '0 0 4px' }}>Devices</h1>
          <p style={{ color: C.muted, fontSize: 13, margin: 0 }}>
            Every enrolled device. Assign pending devices to one or more organizations.
          </p>
        </div>
        <Btn variant="soft" onClick={rescan} disabled={isFetching}
          style={{ flexShrink: 0, opacity: isFetching ? .6 : 1, cursor: isFetching ? 'default' : 'pointer' }}>
          {isFetching ? 'Rescanning…' : '\u21bb Rescan'}
        </Btn>
      </div>

      {isLoading ? <p style={{ color: C.muted }}>Loading…</p> : (
        <>
          {unassigned.length > 0 && (
            <>
              <h2 style={{ fontSize: 14, fontWeight: 700, color: C.warn, margin: '0 0 12px', letterSpacing: .4 }}>PENDING · {unassigned.length}</h2>
              <div style={{ display: 'grid', gap: 12, gridTemplateColumns: 'repeat(auto-fill,minmax(300px,1fr))', marginBottom: 28 }}>
                {unassigned.map(Row)}
              </div>
            </>
          )}
          <h2 style={{ fontSize: 14, fontWeight: 700, color: C.muted, margin: '0 0 12px', letterSpacing: .4 }}>ASSIGNED · {assigned.length}</h2>
          <div style={{ display: 'grid', gap: 12, gridTemplateColumns: 'repeat(auto-fill,minmax(300px,1fr))' }}>
            {assigned.map(Row)}
          </div>
          {devices.length === 0 && <p style={{ color: C.muted }}>No devices enrolled yet.</p>}
        </>
      )}

      <Modal open={!!assign} onClose={() => setAssign(null)} title={`Assign "${assign?.device.name}"`}>
        <p style={{ color: C.muted, fontSize: 13, margin: '0 0 14px' }}>Select the organizations that can see this device.</p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, maxHeight: 300, overflowY: 'auto' }}>
          {orgs.map(o => {
            const on = assign?.orgIds.has(o.id)
            return (
              <div key={o.id} onClick={() => setAssign({ ...assign, orgIds: toggle(assign.orgIds, o.id) })}
                style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px', borderRadius: 12,
                  cursor: 'pointer', background: on ? 'rgba(124,108,255,.16)' : C.surface2,
                  border: `1px solid ${on ? C.accent : C.border}` }}>
                <div style={{ width: 18, height: 18, borderRadius: 6, border: `2px solid ${on ? C.accent : C.muted}`,
                  background: on ? C.accent : 'transparent', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: 12 }}>{on ? '✓' : ''}</div>
                <span style={{ fontSize: 14 }}>{o.name}</span>
              </div>
            )
          })}
          {orgs.length === 0 && <p style={{ color: C.muted, fontSize: 13 }}>No organizations yet — create one first.</p>}
        </div>
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 20 }}>
          <Btn variant="ghost" onClick={() => setAssign(null)}>Cancel</Btn>
          <Btn onClick={() => saveAssign.mutate({ id: assign.device.id, orgIds: assign.orgIds })}>Save</Btn>
        </div>
      </Modal>

      <Modal open={!!rename} onClose={() => setRename(null)} title="Rename device">
        <Field value={rename?.name ?? ''} onChange={e => setRename({ ...rename, name: e.target.value })} placeholder="Device name" />
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 20 }}>
          <Btn variant="ghost" onClick={() => setRename(null)}>Cancel</Btn>
          <Btn onClick={() => rename?.name && saveRename.mutate(rename)}>Save</Btn>
        </div>
      </Modal>
    </div>
  )
}
