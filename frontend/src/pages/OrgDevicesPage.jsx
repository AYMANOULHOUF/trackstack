import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../api'
import { C, Card, Btn, Field, Dot, Modal, STATUS_COLOR } from '../ui'

export default function OrgDevicesPage() {
  const qc = useQueryClient()
  const [editing, setEditing] = useState(null) // {id, name}
  const { data: devices = [], isLoading } = useQuery({
    queryKey: ['devices'], queryFn: () => api.get('/devices').then(r => r.data),
  })

  const rename = useMutation({
    mutationFn: ({ id, name }) => api.patch(`/devices/${id}`, { name }),
    onSuccess: () => { setEditing(null); qc.invalidateQueries({ queryKey: ['devices'] }) },
  })
  const remove = useMutation({
    mutationFn: (id) => api.delete(`/devices/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['devices'] }),
  })

  return (
    <div style={{ padding: 28, overflowY: 'auto', height: '100%' }}>
      <h1 style={{ fontSize: 22, fontWeight: 800, margin: '0 0 4px' }}>Devices</h1>
      <p style={{ color: C.muted, fontSize: 13, margin: '0 0 22px' }}>
        Devices assigned to your organization by the administrator. You can rename or remove them.
      </p>

      {isLoading ? <p style={{ color: C.muted }}>Loading…</p> : (
        <div style={{ display: 'grid', gap: 12, gridTemplateColumns: 'repeat(auto-fill,minmax(300px,1fr))' }}>
          {devices.map(d => (
            <Card key={d.id} style={{ padding: 16 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 10 }}>
                <Dot color={STATUS_COLOR[d.status] ?? STATUS_COLOR.unknown} />
                <div style={{ fontSize: 16, fontWeight: 700 }}>{d.name}</div>
              </div>
              <div style={{ color: C.muted, fontSize: 13, marginBottom: 14 }}>
                {d.type} · {d.status}{d.last_speed != null ? ` · ${Math.round(d.last_speed)} km/h` : ''}
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <Btn variant="ghost" onClick={() => setEditing({ id: d.id, name: d.name })}>Rename</Btn>
                <Btn variant="danger" onClick={() => confirm(`Remove ${d.name} from your organization?`) && remove.mutate(d.id)}>Remove</Btn>
              </div>
            </Card>
          ))}
          {devices.length === 0 && <p style={{ color: C.muted }}>No devices assigned yet.</p>}
        </div>
      )}

      <Modal open={!!editing} onClose={() => setEditing(null)} title="Rename device">
        <Field value={editing?.name ?? ''} onChange={e => setEditing({ ...editing, name: e.target.value })}
          placeholder="Device name" />
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 20 }}>
          <Btn variant="ghost" onClick={() => setEditing(null)}>Cancel</Btn>
          <Btn onClick={() => editing?.name && rename.mutate(editing)}>Save</Btn>
        </div>
      </Modal>
    </div>
  )
}
