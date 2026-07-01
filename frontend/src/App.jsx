import { BrowserRouter, Routes, Route, Navigate, NavLink } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './AuthContext'
import { C } from './ui'
import LoginPage from './pages/LoginPage'
import OrgMapPage from './pages/OrgMapPage'
import OrgDevicesPage from './pages/OrgDevicesPage'
import AdminOrgsPage from './pages/AdminOrgsPage'
import AdminDevicesPage from './pages/AdminDevicesPage'
import AdminMapPage from './pages/AdminMapPage'

const qc = new QueryClient()

function RequireRole({ role, children }) {
  const { user } = useAuth()
  if (!user) return <Navigate to="/login" replace />
  if (role && user.role !== role) {
    return <Navigate to={user.role === 'admin' ? '/admin/devices' : '/'} replace />
  }
  return children
}

function Layout({ children }) {
  const { user, logout } = useAuth()
  const links = user?.role === 'admin'
    ? [['/admin/map', 'Map'], ['/admin/devices', 'Devices'], ['/admin/orgs', 'Organizations']]
    : [['/', 'Map'], ['/devices', 'Devices']]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <nav style={S.nav}>
        <span style={S.brand}>Track<span style={{ color: C.accent }}>Proj</span></span>
        {user?.role === 'admin' && <span style={S.badge}>ADMIN</span>}
        <div style={{ display: 'flex', gap: 4, marginLeft: 8 }}>
          {links.map(([to, label]) => (
            <NavLink key={to} to={to} end style={({ isActive }) => ({
              ...S.link, ...(isActive ? S.linkActive : {}),
            })}>{label}</NavLink>
          ))}
        </div>
        <button onClick={logout} style={S.logout}>Sign out</button>
      </nav>
      <div style={{ flex: 1, overflow: 'hidden', position: 'relative' }}>{children}</div>
    </div>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={qc}>
      <AuthProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<LoginPage />} />

            {/* Org */}
            <Route path="/" element={<RequireRole role="org"><Layout><OrgMapPage /></Layout></RequireRole>} />
            <Route path="/devices" element={<RequireRole role="org"><Layout><OrgDevicesPage /></Layout></RequireRole>} />

            {/* Admin */}
            <Route path="/admin/map" element={<RequireRole role="admin"><Layout><AdminMapPage /></Layout></RequireRole>} />
            <Route path="/admin/devices" element={<RequireRole role="admin"><Layout><AdminDevicesPage /></Layout></RequireRole>} />
            <Route path="/admin/orgs" element={<RequireRole role="admin"><Layout><AdminOrgsPage /></Layout></RequireRole>} />

            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  )
}

const S = {
  nav: { display: 'flex', alignItems: 'center', gap: 12, padding: '0 20px', height: 58,
    background: C.surface, borderBottom: `1px solid ${C.border}`, flexShrink: 0 },
  brand: { fontWeight: 800, fontSize: 17, letterSpacing: .2 },
  badge: { fontSize: 10, fontWeight: 700, letterSpacing: 1, color: C.accent,
    background: 'rgba(124,108,255,.16)', padding: '3px 8px', borderRadius: 999 },
  link: { color: C.muted, textDecoration: 'none', fontSize: 14, fontWeight: 600,
    padding: '7px 14px', borderRadius: 999 },
  linkActive: { color: '#fff', background: 'rgba(124,108,255,.16)' },
  logout: { marginLeft: 'auto', background: 'transparent', border: `1px solid ${C.border}`,
    color: C.muted, cursor: 'pointer', fontSize: 13, padding: '7px 14px', borderRadius: 999 },
}
