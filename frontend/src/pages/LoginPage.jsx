import { useState } from 'react'
import { useNavigate, Navigate } from 'react-router-dom'
import { useAuth } from '../AuthContext'
import { C, Card, Btn, Field, Splash } from '../ui'

export default function LoginPage() {
  const { login, user, checking } = useAuth()
  const nav = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // A stored session is validated before the form is drawn; once it is known
  // to be good, go straight to the right home screen.
  if (checking) return <Splash />
  if (user) return <Navigate to={user.role === 'admin' ? '/admin/devices' : '/'} replace />

  async function submit(e) {
    e.preventDefault()
    setError(''); setLoading(true)
    try {
      const role = await login(email, password)
      nav(role === 'admin' ? '/admin/devices' : '/', { replace: true })
    } catch (err) {
      setError(err.response?.data?.error ?? 'Invalid email or password')
      setLoading(false)
    }
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 20 }}>
      <Card style={{ width: 380, padding: 36, animation: 'floatIn .2s ease' }}>
        <h1 style={{ margin: '0 0 6px', fontSize: 26, fontWeight: 800 }}>
          Track<span style={{ color: C.accent }}>Stack</span>
        </h1>
        <p style={{ margin: '0 0 26px', color: C.muted, fontSize: 14 }}>Sign in to your dashboard</p>
        <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <Field type="email" placeholder="Email" value={email} onChange={e => setEmail(e.target.value)}
            autoFocus autoComplete="username" required />
          <Field type="password" placeholder="Password" value={password}
            onChange={e => setPassword(e.target.value)} autoComplete="current-password" required />
          {error && <p style={{ color: C.danger, fontSize: 13, margin: 0 }}>{error}</p>}
          <Btn type="submit" disabled={loading} style={{ justifyContent: 'center', padding: '13px', marginTop: 4 }}>
            {loading ? 'Signing in…' : 'Sign in'}
          </Btn>
        </form>
        <p style={{ margin: '18px 0 0', color: C.muted, fontSize: 11.5, lineHeight: 1.5 }}>
          Administrators manage devices and organizations. Organization accounts only see the
          devices assigned to them.
        </p>
      </Card>
    </div>
  )
}
