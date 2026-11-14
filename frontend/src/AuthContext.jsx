import { createContext, useContext, useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import api from './api'

const AuthCtx = createContext(null)

const readSession = (data) => ({
  id: data.user_id,
  role: data.role,
  orgId: data.org_id ?? null,
})

export function AuthProvider({ children }) {
  const qc = useQueryClient()
  const [user, setUser] = useState(null)
  // While a stored token is being checked we must not render the login page
  // (or any protected screen) yet.
  const [checking, setChecking] = useState(() => !!localStorage.getItem('access_token'))

  // The role always comes from the API. Reading it from localStorage would let
  // anyone unlock the admin area by editing a stored string.
  useEffect(() => {
    if (!localStorage.getItem('access_token')) {
      setChecking(false)
      return
    }
    let alive = true
    api.get('/me')
      .then(({ data }) => { if (alive) setUser(readSession(data)) })
      .catch(() => {
        if (!alive) return
        localStorage.clear()
        setUser(null)
      })
      .finally(() => { if (alive) setChecking(false) })
    return () => { alive = false }
  }, [])

  async function login(email, password) {
    const { data } = await api.post('/auth/login', { email, password })
    localStorage.setItem('access_token', data.access_token)
    localStorage.setItem('refresh_token', data.refresh_token)
    setUser(readSession(data))
    // Never let a previous session's cached devices leak into this one.
    qc.clear()
    return data.role
  }

  function logout() {
    localStorage.clear()
    setUser(null)
    qc.clear()
  }

  return (
    <AuthCtx.Provider value={{ user, checking, login, logout }}>
      {children}
    </AuthCtx.Provider>
  )
}

export const useAuth = () => useContext(AuthCtx)
