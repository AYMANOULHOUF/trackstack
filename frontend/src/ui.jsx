// TrackStack design tokens — charcoal-green dashboard theme.
// Base surfaces are GitHub-dark-like so dense telemetry stays readable; the
// three data accents carry meaning: mint = good/moving, coral = alert/loss,
// gold = neutral highlight/stationary.
export const C = {
  bg: '#0D1117',
  surface: '#161B22',
  surface2: '#21262D',
  hover: '#21262D',
  border: '#30363D',
  text: '#E6EDF3',
  muted: '#8B949E',
  accent: '#10B981',
  accentSoft: 'rgba(16,185,129,.14)',
  danger: '#F85149',
  ok: '#10B981',
  warn: '#F59E0B',
  offline: '#8B949E',
  grid: '#21262D',
}

export const STATUS_COLOR = {
  moving: C.ok,
  stopped: C.warn,
  paused: C.muted,
  offline: C.danger,
  unknown: '#6E7681',
}

export const STATUS_LABEL = {
  moving: 'Moving',
  stopped: 'Stationary',
  paused: 'Tracking off',
  offline: 'Offline',
  unknown: 'No data',
}

export function Card({ children, style, ...rest }) {
  return <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: 14,
    boxShadow: '0 1px 0 rgba(255,255,255,.02), 0 8px 24px rgba(0,0,0,.28)', ...style }} {...rest}>{children}</div>
}

export function Btn({ children, variant = 'primary', style, ...rest }) {
  const base = { border: 'none', borderRadius: 8, padding: '8px 14px', fontSize: 13,
    fontWeight: 600, cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: 7,
    transition: 'background .15s, border-color .15s, filter .15s' }
  const variants = {
    primary: { background: C.accent, color: '#04150E' },
    ghost: { background: 'transparent', color: C.text, border: `1px solid ${C.border}` },
    soft: { background: C.accentSoft, color: C.accent },
    danger: { background: 'rgba(248,81,73,.14)', color: C.danger },
  }
  return <button style={{ ...base, ...variants[variant], ...style }}
    onMouseDown={e => e.currentTarget.style.filter = 'brightness(.92)'}
    onMouseUp={e => e.currentTarget.style.filter = 'none'}
    onMouseLeave={e => e.currentTarget.style.filter = 'none'} {...rest}>{children}</button>
}

export function Field({ style, ...rest }) {
  return <input style={{ padding: '9px 12px', background: C.surface2, border: `1px solid ${C.border}`,
    borderRadius: 8, color: C.text, fontSize: 14, outline: 'none', width: '100%', ...style }}
    onFocus={e => e.target.style.borderColor = C.accent}
    onBlur={e => e.target.style.borderColor = C.border} {...rest} />
}

export function Select({ style, children, ...rest }) {
  return <select style={{ padding: '8px 12px', background: C.surface2, border: `1px solid ${C.border}`,
    borderRadius: 8, color: C.text, fontSize: 13, fontWeight: 600, outline: 'none', cursor: 'pointer',
    ...style }} {...rest}>{children}</select>
}

export function Chip({ children, color, style }) {
  return <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 11,
    fontWeight: 600, letterSpacing: .2, padding: '3px 9px', borderRadius: 999,
    background: color ? `${color}1F` : C.surface2,
    color: color ?? C.muted, border: `1px solid ${color ? color + '3D' : C.border}`, ...style }}>{children}</span>
}

export function Dot({ color, size = 9, pulse = false }) {
  return <span style={{ width: size, height: size, borderRadius: '50%', background: color,
    display: 'inline-block', flexShrink: 0, boxShadow: `0 0 0 ${pulse ? 3 : 0}px ${color}33, 0 0 10px ${color}66`,
    animation: pulse ? 'pulse 2s ease-in-out infinite' : undefined }} />
}

export function Modal({ open, onClose, title, children, width = 460 }) {
  if (!open) return null
  return (
    <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(1,4,9,.72)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000, backdropFilter: 'blur(2px)' }}>
      <Card onClick={e => e.stopPropagation()} style={{ width, maxWidth: '92vw', padding: 24,
        animation: 'floatIn .18s ease' }}>
        {title && <h2 style={{ margin: '0 0 16px', fontSize: 18, fontWeight: 700 }}>{title}</h2>}
        {children}
      </Card>
    </div>
  )
}

// Small label above a value — used across dashboard tiles.
export function Stat({ label, value, unit, hint, color, style }) {
  return (
    <div style={{ minWidth: 0, ...style }}>
      <div style={{ fontSize: 10.5, letterSpacing: .7, textTransform: 'uppercase', color: C.muted,
        fontWeight: 700, marginBottom: 4 }}>{label}</div>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
        <span style={{ fontSize: 24, fontWeight: 700, color: color ?? C.text, lineHeight: 1.1,
          fontVariantNumeric: 'tabular-nums' }}>{value}</span>
        {unit && <span style={{ fontSize: 12, color: C.muted, fontWeight: 600 }}>{unit}</span>}
      </div>
      {hint && <div style={{ fontSize: 11.5, color: C.muted, marginTop: 3 }}>{hint}</div>}
    </div>
  )
}
