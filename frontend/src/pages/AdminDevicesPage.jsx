import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../api'
import { C, Card, Btn, Field, Chip, Dot, Modal, Select, STATUS_COLOR, STATUS_LABEL } from '../ui'

export default function AdminDevicesPage() {
  const qc = useQueryClient()
  const [assign, setAssign] = useState(null)
  const [rename, setRename] = useState(null)
  const [create, setCreate] = useState(null) // { name, type, orgIds, token?, deviceId?, error? }
  const [copied, setCopied] = useState(false)

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

  // Enrol a device from the dashboard, then put it straight into the chosen
  // organizations in the same step. The API only returns the token once, so the
  // modal switches to a "copy it now" state instead of closing.
  const createDevice = useMutation({
    mutationFn: async ({ name, type, orgIds }) => {
      const { data } = await api.post('/enroll', { name, type })
      let assignFailed = false
      if (orgIds.size > 0) {
        try {
          await api.put(`/devices/${data.id}/orgs`, { org_ids: [...orgIds] })
        } catch {
          assignFailed = true
        }
      }
      return { ...data, assignFailed }
    },
    onSuccess: (data) => {
      setCopied(false)
      // If the modal was dismissed while the request was in flight, do not
      // resurrect it with the token.
      setCreate(c => c ? { ...c, token: data.api_token, deviceId: data.id, assignFailed: data.assignFailed } : c)
      qc.invalidateQueries({ queryKey: ['devices'] })
      qc.invalidateQueries({ queryKey: ['orgs'] })
    },
    onError: (e) => setCreate(c => c ? ({
      ...c, error: e.response?.data?.error ?? 'Could not create the device',
    }) : c),
  })
  const saveRename = useMutation({
    mutationFn: ({ id, name, externalId }) => {
      // Always send external_id so clearing the field removes the hardware link.
      return api.patch(`/devices/${id}`, { name, external_id: externalId ?? '' })
    },
    onSuccess: () => { setRename(null); qc.invalidateQueries({ queryKey: ['devices'] }) },
  })
  const del = useMutation({
    mutationFn: (id) => api.delete(`/devices/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['devices'] }),
  })

  const unassigned = devices.filter(d => !d.assigned)
  const assigned = devices.filter(d => d.assigned)
  const toggle = (set, id) => {
    const n = new Set(set)
    if (n.has(id)) n.delete(id)
    else n.add(id)
    return n
  }

  const openCreate = () => {
    setCopied(false)
    setCreate({ name: '', type: 'phone', orgIds: new Set(), error: '' })
  }
  const closeCreate = () => {
    setCreate(null)
    setCopied(false)
  }
  const copyToken = async () => {
    if (!create?.token) return
    try {
      await navigator.clipboard.writeText(create.token)
    } catch {
      // Clipboard can be blocked outside a secure context; the token is still selectable.
    }
    setCopied(true)
  }

  const Row = (d) => (
    <Card key={d.id} style={{ padding: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8 }}>
        <Dot color={STATUS_COLOR[d.status] ?? STATUS_COLOR.unknown} />
        <div style={{ fontSize: 16, fontWeight: 700 }}>{d.name}</div>
        {!d.assigned && <Chip color={C.warn} style={{ marginLeft: 'auto' }}>Pending</Chip>}
      </div>
      <div style={{ color: C.muted, fontSize: 12, marginBottom: 10 }}>
        {d.type} · {STATUS_LABEL[d.status] ?? d.status}{d.last_speed != null ? ` · ${Math.round(d.last_speed)} km/h` : ''}
        {d.external_id ? ` · ID ${d.external_id}` : ''}
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 14, minHeight: 22 }}>
        {(d.orgs ?? []).map(o => <Chip key={o.id} color={C.accent}>{o.name}</Chip>)}
        {(d.orgs ?? []).length === 0 && <span style={{ color: C.muted, fontSize: 12 }}>Not in any organization</span>}
      </div>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <Btn onClick={() => setAssign({ device: d, orgIds: new Set((d.orgs ?? []).map(o => o.id)) })}>Assign</Btn>
        <Btn variant="ghost" onClick={() => setRename({ id: d.id, name: d.name, externalId: d.external_id ?? '' })}>Edit</Btn>
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
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>
          <Btn onClick={openCreate}>+ New device</Btn>
          <Btn variant="soft" onClick={rescan} disabled={isFetching}
            style={{ opacity: isFetching ? .6 : 1, cursor: isFetching ? 'default' : 'pointer' }}>
            {isFetching ? 'Rescanning…' : '\u21bb Rescan'}
          </Btn>
        </div>
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

      <Modal open={!!create} onClose={closeCreate} title={create?.token ? 'Device created' : 'New device'}>
        {create?.token ? (
          <>
            <p style={{ color: C.muted, fontSize: 13, margin: '0 0 12px' }}>
              <strong style={{ color: C.text }}>{create.name}</strong> is enrolled. Copy the API token now —
              it is shown only once.
            </p>

            <code style={{ display: 'block', padding: '11px 12px', borderRadius: 8,
              background: C.surface2, border: `1px solid ${C.border}`, color: C.text,
              fontSize: 12, fontFamily: 'monospace', wordBreak: 'break-all' }}>{create.token}</code>

            <div style={{ marginTop: 10 }}>
              <Btn variant="soft" onClick={copyToken}>{copied ? 'Copied ✓' : 'Copy token'}</Btn>
            </div>

            {create.assignFailed && (
              <p style={{ color: C.warn, fontSize: 12.5, margin: '12px 0 0' }}>
                The device was created but could not be assigned — use Assign to add it to an organization.
              </p>
            )}

            <p style={{ color: C.muted, fontSize: 12, margin: '12px 0 0' }}>
              {create.type === 'hardware'
                ? 'Set the hardware ID (IMEI or serial) with Edit so the protocol gateway can match this tracker.'
                : 'Paste this token into the TrackStack app, or send it as a Bearer token to POST /v1/positions.'}
            </p>

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 20 }}>
              <Btn onClick={closeCreate}>Done</Btn>
            </div>
          </>
        ) : (
          <>
            <label style={{ fontSize: 12, color: C.muted, fontWeight: 600 }}>Name</label>
            <Field autoFocus value={create?.name ?? ''} placeholder="e.g. Van unit 12"
              onChange={e => setCreate({ ...create, name: e.target.value, error: '' })}
              style={{ margin: '6px 0 14px' }} />

            <label style={{ fontSize: 12, color: C.muted, fontWeight: 600 }}>Type</label>
            <Select value={create?.type ?? 'phone'}
              onChange={e => setCreate({ ...create, type: e.target.value })}
              style={{ margin: '6px 0 4px', width: '100%' }}>
              <option value="phone">Phone — TrackStack app</option>
              <option value="hardware">Hardware — GT06 / HTTP tracker</option>
            </Select>
            <p style={{ color: C.muted, fontSize: 11.5, margin: '0 0 14px' }}>
              {create?.type === 'hardware'
                ? 'You will set the IMEI or serial after creating it.'
                : 'Phones enrol themselves on first launch — create one here only to assign it up front.'}
            </p>

            <label style={{ fontSize: 12, color: C.muted, fontWeight: 600 }}>
              Assign to organizations <span style={{ fontWeight: 400 }}>(optional)</span>
            </label>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8, maxHeight: 220,
              overflowY: 'auto', marginTop: 6 }}>
              {orgs.map(o => {
                const on = create?.orgIds.has(o.id)
                return (
                  <div key={o.id} onClick={() => setCreate({ ...create, orgIds: toggle(create.orgIds, o.id) })}
                    style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px', borderRadius: 12,
                      cursor: 'pointer', background: on ? C.accentSoft : C.surface2,
                      border: `1px solid ${on ? C.accent : C.border}` }}>
                    <div style={{ width: 18, height: 18, borderRadius: 6, border: `2px solid ${on ? C.accent : C.muted}`,
                      background: on ? C.accent : 'transparent', display: 'flex', alignItems: 'center',
                      justifyContent: 'center', color: '#04150E', fontSize: 12 }}>{on ? '✓' : ''}</div>
                    <span style={{ fontSize: 14 }}>{o.name}</span>
                  </div>
                )
              })}
              {orgs.length === 0 && (
                <p style={{ color: C.muted, fontSize: 13, margin: 0 }}>
                  No organizations yet — the device will wait as pending until you assign it.
                </p>
              )}
            </div>

            {create?.error && <p style={{ color: C.danger, fontSize: 13, margin: '14px 0 0' }}>{create.error}</p>}

            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 20 }}>
              <Btn variant="ghost" onClick={closeCreate}>Cancel</Btn>
              <Btn disabled={!create?.name?.trim() || createDevice.isPending}
                style={{ opacity: !create?.name?.trim() || createDevice.isPending ? .55 : 1 }}
                onClick={() => createDevice.mutate({
                  name: create.name.trim(), type: create.type, orgIds: create.orgIds,
                })}>{createDevice.isPending ? 'Creating…' : 'Create device'}</Btn>
            </div>
          </>
        )}
      </Modal>

      <Modal open={!!assign} onClose={() => setAssign(null)} title={`Assign "${assign?.device.name}"`}>
        <p style={{ color: C.muted, fontSize: 13, margin: '0 0 14px' }}>Select the organizations that can see this device.</p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, maxHeight: 300, overflowY: 'auto' }}>
          {orgs.map(o => {
            const on = assign?.orgIds.has(o.id)
            return (
              <div key={o.id} onClick={() => setAssign({ ...assign, orgIds: toggle(assign.orgIds, o.id) })}
                style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px', borderRadius: 12,
                  cursor: 'pointer', background: on ? C.accentSoft : C.surface2,
                  border: `1px solid ${on ? C.accent : C.border}` }}>
                <div style={{ width: 18, height: 18, borderRadius: 6, border: `2px solid ${on ? C.accent : C.muted}`,
                  background: on ? C.accent : 'transparent', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#04150E', fontSize: 12 }}>{on ? '✓' : ''}</div>
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

      <Modal open={!!rename} onClose={() => setRename(null)} title="Edit device">
        <label style={{ fontSize: 12, color: C.muted, fontWeight: 600 }}>Name</label>
        <Field value={rename?.name ?? ''} onChange={e => setRename({ ...rename, name: e.target.value })}
          placeholder="Device name" style={{ margin: '6px 0 14px' }} />
        <label style={{ fontSize: 12, color: C.muted, fontWeight: 600 }}>Hardware ID (IMEI or serial)</label>
        <Field value={rename?.externalId ?? ''} onChange={e => setRename({ ...rename, externalId: e.target.value })}
          placeholder="e.g. 123456789012345" style={{ marginTop: 6 }} />
        <p style={{ color: C.muted, fontSize: 12, margin: '8px 0 0' }}>
          Used by the protocol gateway to match GT06/HTTP trackers to this device.
        </p>
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 20 }}>
          <Btn variant="ghost" onClick={() => setRename(null)}>Cancel</Btn>
          <Btn onClick={() => rename?.name && saveRename.mutate(rename)}>Save</Btn>
        </div>
      </Modal>
    </div>
  )
}
