import { useEffect, useRef, useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate, NavLink } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './AuthContext'
import { C } from './ui'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import OrgMapPage from './pages/OrgMapPage'
import OrgDevicesPage from './pages/OrgDevicesPage'
import GeofencesPage from './pages/GeofencesPage'
import TripsPage from './pages/TripsPage'
import NotificationsPage from './pages/NotificationsPage'
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
  const navRef = useRef(null)
  const [navMore, setNavMore] = useState(false)
  const links = user?.role === 'admin'
    ? [['/admin/map', 'Map'], ['/admin/devices', 'Devices'], ['/admin/orgs', 'Organizations']]
    : [['/', 'Dashboard'], ['/map', 'Map'], ['/devices', 'Devices'], ['/geofences', 'Geofences'], ['/trips', 'Trips'], ['/notifications', 'Notifications']]

  // On phones the six links do not fit, so the bar scrolls. Only show the
  // edge fade while there is actually something hidden to the right.
  useEffect(() => {
    const el = navRef.current
    if (!el) return
    const measure = () => setNavMore(el.scrollWidth - el.clientWidth - el.scrollLeft > 8)
    measure()
    el.addEventListener('scroll', measure, { passive: true })
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => {
      el.removeEventListener('scroll', measure)
      ro.disconnect()
    }
  }, [])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <div style={{ position: 'relative', flexShrink: 0 }}>
        <nav ref={navRef} style={S.nav} className="app-nav">
          <span style={S.brand}>Track<span style={{ color: C.accent }}>Stack</span></span>
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
        {navMore && <div className="app-nav-fade" aria-hidden="true" />}
      </div>
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
            <Route path="/" element={<RequireRole role="org"><Layout><DashboardPage /></Layout></RequireRole>} />
            <Route path="/map" element={<RequireRole role="org"><Layout><OrgMapPage /></Layout></RequireRole>} />
            <Route path="/devices" element={<RequireRole role="org"><Layout><OrgDevicesPage /></Layout></RequireRole>} />
            <Route path="/geofences" element={<RequireRole role="org"><Layout><GeofencesPage /></Layout></RequireRole>} />
            <Route path="/trips" element={<RequireRole role="org"><Layout><TripsPage /></Layout></RequireRole>} />
            <Route path="/notifications" element={<RequireRole role="org"><Layout><NotificationsPage /></Layout></RequireRole>} />

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
    background: C.surface, borderBottom: `1px solid ${C.border}`, flexShrink: 0,
    overflowX: 'auto' },
  brand: { fontWeight: 800, fontSize: 17, letterSpacing: .2, flexShrink: 0 },
  badge: { fontSize: 10, fontWeight: 700, letterSpacing: 1, color: C.accent,
    background: C.accentSoft, padding: '3px 8px', borderRadius: 999 },
  link: { color: C.muted, textDecoration: 'none', fontSize: 14, fontWeight: 600,
    padding: '7px 14px', borderRadius: 8, transition: 'background .15s, color .15s',
    whiteSpace: 'nowrap', flexShrink: 0 },
  linkActive: { color: C.text, background: C.surface2 },
  logout: { marginLeft: 'auto', background: 'transparent', border: `1px solid ${C.border}`,
    color: C.muted, cursor: 'pointer', fontSize: 13, padding: '7px 14px', borderRadius: 999,
    whiteSpace: 'nowrap', flexShrink: 0 },
}
