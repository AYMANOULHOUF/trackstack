import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../api'
import { C, Card, Btn, Field, Chip, Modal } from '../ui'

const empty = { name: '', email: '', password: '' }

export default function AdminOrgsPage() {
  const qc = useQueryClient()
  const [form, setForm] = useState(null)
  const [error, setError] = useState('')

  const { data: orgs = [], isLoading } = useQuery({
    queryKey: ['orgs'], queryFn: () => api.get('/orgs').then(r => r.data),
  })

  const save = useMutation({
    mutationFn: (f) => f.id
      ? api.patch(`/orgs/${f.id}`, { name: f.name, email: f.email, ...(f.password ? { password: f.password } : {}) })
      : api.post('/orgs', { name: f.name, email: f.email, password: f.password }),
    onSuccess: () => { setForm(null); setError(''); qc.invalidateQueries({ queryKey: ['orgs'] }) },
    onError: (e) => setError(e.response?.data?.error ?? 'Failed'),
  })
  const del = useMutation({
    mutationFn: (id) => api.delete(`/orgs/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['orgs'] }),
  })

  return (
    <div style={{ padding: 28, overflowY: 'auto', height: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 22 }}>
        <div>
          <h1 style={{ fontSize: 22, fontWeight: 800, margin: '0 0 4px' }}>Organizations</h1>
          <p style={{ color: C.muted, fontSize: 13, margin: 0 }}>Create and manage organization accounts and their logins.</p>
        </div>
        <Btn style={{ marginLeft: 'auto' }} onClick={() => { setError(''); setForm({ ...empty }) }}>+ New organization</Btn>
      </div>

      {isLoading ? <p style={{ color: C.muted }}>Loading…</p> : (
        <div style={{ display: 'grid', gap: 12, gridTemplateColumns: 'repeat(auto-fill,minmax(320px,1fr))' }}>
          {orgs.map(o => (
            <Card key={o.id} style={{ padding: 18 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                <div style={{ fontSize: 17, fontWeight: 700 }}>{o.name}</div>
                <Chip color={C.accent} style={{ marginLeft: 'auto' }}>{o.device_count} devices</Chip>
              </div>
              <div style={{ color: C.muted, fontSize: 13, marginBottom: 16 }}>{o.email}</div>
              <div style={{ display: 'flex', gap: 8 }}>
                <Btn variant="ghost" onClick={() => { setError(''); setForm({ id: o.id, name: o.name, email: o.email, password: '' }) }}>Edit</Btn>
                <Btn variant="danger" onClick={() => confirm(`Delete ${o.name}? Its devices become unassigned.`) && del.mutate(o.id)}>Delete</Btn>
              </div>
            </Card>
          ))}
          {orgs.length === 0 && <p style={{ color: C.muted }}>No organizations yet. Create one to get started.</p>}
        </div>
      )}

      <Modal open={!!form} onClose={() => setForm(null)} title={form?.id ? 'Edit organization' : 'New organization'}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Field placeholder="Organization name" value={form?.name ?? ''} onChange={e => setForm({ ...form, name: e.target.value })} />
          <Field type="email" placeholder="Login email" value={form?.email ?? ''} onChange={e => setForm({ ...form, email: e.target.value })} />
          <Field type="password" placeholder={form?.id ? 'New password (leave blank to keep)' : 'Password (8+ chars)'}
            value={form?.password ?? ''} onChange={e => setForm({ ...form, password: e.target.value })} />
          {error && <p style={{ color: C.danger, fontSize: 13, margin: 0 }}>{error}</p>}
        </div>
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 20 }}>
          <Btn variant="ghost" onClick={() => setForm(null)}>Cancel</Btn>
          <Btn onClick={() => save.mutate(form)} disabled={save.isPending}>{save.isPending ? 'Saving…' : 'Save'}</Btn>
        </div>
      </Modal>
    </div>
  )
}
