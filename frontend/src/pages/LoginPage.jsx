import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../AuthContext'
import { C, Card, Btn, Field } from '../ui'

export default function LoginPage() {
  const { login } = useAuth()
  const nav = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function submit(e) {
    e.preventDefault()
    setError(''); setLoading(true)
    try {
      const role = await login(email, password)
      nav(role === 'admin' ? '/admin/devices' : '/', { replace: true })
    } catch (err) {
      setError(err.response?.data?.error ?? 'Something went wrong')
      setLoading(false)
    }
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 20 }}>
      <Card style={{ width: 380, padding: 36, animation: 'floatIn .2s ease' }}>
        <h1 style={{ margin: '0 0 6px', fontSize: 26, fontWeight: 800 }}>
          Track<span style={{ color: C.accent }}>Proj</span>
        </h1>
        <p style={{ margin: '0 0 26px', color: C.muted, fontSize: 14 }}>Sign in to your dashboard</p>
        <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <Field type="email" placeholder="Email" value={email} onChange={e => setEmail(e.target.value)} required />
          <Field type="password" placeholder="Password" value={password} onChange={e => setPassword(e.target.value)} required />
          {error && <p style={{ color: C.danger, fontSize: 13, margin: 0 }}>{error}</p>}
          <Btn type="submit" disabled={loading} style={{ justifyContent: 'center', padding: '13px', marginTop: 4 }}>
            {loading ? 'Signing in…' : 'Sign in'}
          </Btn>
        </form>
      </Card>
    </div>
  )
}
