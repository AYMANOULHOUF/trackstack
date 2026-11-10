import { useEffect, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../api'
import { C, Card, Btn, Field, Chip } from '../ui'

function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(base64)
  return Uint8Array.from([...raw].map(c => c.charCodeAt(0)))
}

function Toggle({ on, onChange }) {
  return (
    <button type="button" onClick={() => onChange(!on)}
      style={{ width: 44, height: 24, borderRadius: 999, border: 'none', cursor: 'pointer',
        background: on ? C.accent : C.border, position: 'relative', flexShrink: 0 }}>
      <span style={{ position: 'absolute', top: 3, left: on ? 23 : 3, width: 18, height: 18,
        borderRadius: '50%', background: '#fff', transition: 'left .15s' }} />
    </button>
  )
}

export default function NotificationsPage() {
  const qc = useQueryClient()
  const [emailEnabled, setEmailEnabled] = useState(false)
  const [emailTarget, setEmailTarget] = useState('')
  const [pushEnabled, setPushEnabled] = useState(false)
  const [pushTarget, setPushTarget] = useState('')
  const [pushBusy, setPushBusy] = useState(false)
  const [message, setMessage] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => api.get('/notifications').then(r => r.data),
  })

  useEffect(() => {
    if (!data?.settings) return
    const email = data.settings.find(s => s.channel === 'email')
    const push = data.settings.find(s => s.channel === 'web_push')
    if (email) { setEmailEnabled(email.enabled); setEmailTarget(email.target ?? '') }
    if (push) { setPushEnabled(push.enabled); setPushTarget(push.target ?? '') }
  }, [data])

  const save = useMutation({
    mutationFn: settings => api.put('/notifications', { settings }),
    onSuccess: () => {
      setMessage('Settings saved.')
      qc.invalidateQueries({ queryKey: ['notifications'] })
    },
    onError: err => setMessage(err.response?.data?.error ?? 'Could not save settings.'),
  })

  function saveAll() {
    save.mutate([
      { channel: 'email', enabled: emailEnabled, target: emailTarget },
      { channel: 'web_push', enabled: pushEnabled, target: pushTarget },
    ])
  }

  async function enableBrowserPush() {
    setPushBusy(true)
    setMessage('')
    try {
      if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
        throw new Error('This browser does not support Web Push.')
      }
      if (!data?.vapid_public_key) {
        throw new Error('The server has no VAPID public key. Set VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY.')
      }
      const registration = await navigator.serviceWorker.register('/sw.js')
      const existing = await registration.pushManager.getSubscription()
      const subscription = existing ?? await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(data.vapid_public_key),
      })
      const json = JSON.stringify(subscription)
      setPushTarget(json)
      setPushEnabled(true)
      await api.put('/notifications', {
        settings: [
          { channel: 'email', enabled: emailEnabled, target: emailTarget },
          { channel: 'web_push', enabled: true, target: json },
        ],
      })
      qc.invalidateQueries({ queryKey: ['notifications'] })
      setMessage('Browser notifications enabled.')
    } catch (err) {
      setMessage(err.message ?? 'Could not enable browser notifications.')
    } finally {
      setPushBusy(false)
    }
  }

  async function disableBrowserPush() {
    setPushBusy(true)
    try {
      const registration = await navigator.serviceWorker.getRegistration()
      const subscription = await registration?.pushManager.getSubscription()
      await subscription?.unsubscribe()
      setPushEnabled(false)
      setPushTarget('')
      await api.put('/notifications', {
        settings: [
          { channel: 'email', enabled: emailEnabled, target: emailTarget },
          { channel: 'web_push', enabled: false, target: '' },
        ],
      })
      qc.invalidateQueries({ queryKey: ['notifications'] })
      setMessage('Browser notifications disabled.')
    } catch (err) {
      setMessage(err.message ?? 'Could not disable browser notifications.')
    } finally {
      setPushBusy(false)
    }
  }

  return (
    <div style={{ padding: 28, overflowY: 'auto', height: '100%' }}>
      <h1 style={{ fontSize: 22, fontWeight: 800, margin: '0 0 4px' }}>Notifications</h1>
      <p style={{ color: C.muted, fontSize: 13, margin: '0 0 22px', maxWidth: 620 }}>
        Alerts are sent when a device exceeds its speed limit or enters or leaves a geofence.
        Dashboard alerts always appear on the live map; the channels below add email and browser push.
      </p>

      {isLoading ? <p style={{ color: C.muted }}>Loading…</p> : (
        <div style={{ display: 'grid', gap: 14, maxWidth: 720 }}>
          <Card style={{ padding: 20 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 6 }}>
              <h2 style={{ margin: 0, fontSize: 16, fontWeight: 700 }}>Email</h2>
              <Chip color={emailEnabled ? C.ok : C.muted}>{emailEnabled ? 'enabled' : 'off'}</Chip>
              <div style={{ marginLeft: 'auto' }}><Toggle on={emailEnabled} onChange={setEmailEnabled} /></div>
            </div>
            <p style={{ color: C.muted, fontSize: 12, margin: '0 0 12px' }}>
              Requires SMTP_ADDR, SMTP_FROM and optional SMTP_USER / SMTP_PASSWORD on the API service.
            </p>
            <Field type="email" placeholder="ops@example.com" value={emailTarget}
              onChange={e => setEmailTarget(e.target.value)} disabled={!emailEnabled} />
          </Card>

          <Card style={{ padding: 20 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 6 }}>
              <h2 style={{ margin: 0, fontSize: 16, fontWeight: 700 }}>Web Push</h2>
              <Chip color={pushEnabled ? C.ok : C.muted}>{pushEnabled ? 'enabled' : 'off'}</Chip>
            </div>
            <p style={{ color: C.muted, fontSize: 12, margin: '0 0 14px' }}>
              Subscribes this browser. Push works only over HTTPS (or localhost) and needs VAPID keys on the API service.
            </p>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              {pushEnabled ? (
                <Btn variant="danger" onClick={disableBrowserPush} disabled={pushBusy}>
                  {pushBusy ? 'Working…' : 'Disable in this browser'}
                </Btn>
              ) : (
                <Btn onClick={enableBrowserPush} disabled={pushBusy}>
                  {pushBusy ? 'Working…' : 'Enable in this browser'}
                </Btn>
              )}
            </div>
          </Card>

          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Btn onClick={saveAll} disabled={save.isPending}>
              {save.isPending ? 'Saving…' : 'Save settings'}
            </Btn>
            {message && <span style={{ fontSize: 13, color: C.muted }}>{message}</span>}
          </div>
        </div>
      )}
    </div>
  )
}
