import { createContext, useContext, useState } from 'react'
import api from './api'

const AuthCtx = createContext(null)

export function AuthProvider({ children }) {
  const [user, setUser] = useState(() => {
    const id = localStorage.getItem('user_id')
    const role = localStorage.getItem('role')
    const org = localStorage.getItem('org_id')
    return id ? { id, role, orgId: org } : null
  })

  async function login(email, password) {
    const { data } = await api.post('/auth/login', { email, password })
    localStorage.setItem('access_token', data.access_token)
    localStorage.setItem('refresh_token', data.refresh_token)
    localStorage.setItem('user_id', data.user_id)
    localStorage.setItem('role', data.role)
    if (data.org_id) localStorage.setItem('org_id', data.org_id)
    else localStorage.removeItem('org_id')
    setUser({ id: data.user_id, role: data.role, orgId: data.org_id })
    return data.role
  }

  function logout() {
    localStorage.clear()
    setUser(null)
  }

  return <AuthCtx.Provider value={{ user, login, logout }}>{children}</AuthCtx.Provider>
}

export const useAuth = () => useContext(AuthCtx)
