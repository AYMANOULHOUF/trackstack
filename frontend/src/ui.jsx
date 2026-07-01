// Shared floaty dark design system, matching the mobile app: dark surfaces,
// soft shadows, rounded corners, purple accent.
export const C = {
  bg: '#0b0e14', surface: '#151925', surface2: '#1c2130', border: '#252b3b',
  text: '#e6e9f0', muted: '#8b93a7', accent: '#7c6cff', accentSoft: 'rgba(124,108,255,.16)',
  danger: '#f87171', ok: '#34d399', warn: '#fbbf24', offline: '#6b7280',
}

export const STATUS_COLOR = { moving: C.ok, stopped: C.warn, offline: C.offline, unknown: C.offline }

export function Card({ children, style, ...rest }) {
  return <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: 20,
    boxShadow: '0 10px 40px rgba(0,0,0,.35)', ...style }} {...rest}>{children}</div>
}

export function Btn({ children, variant = 'primary', style, ...rest }) {
  const base = { border: 'none', borderRadius: 999, padding: '10px 20px', fontSize: 14,
    fontWeight: 600, cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: 8,
    transition: 'filter .15s, background .15s' }
  const variants = {
    primary: { background: C.accent, color: '#fff' },
    ghost: { background: 'transparent', color: C.text, border: `1px solid ${C.border}` },
    soft: { background: C.accentSoft, color: C.accent2 ?? C.accent },
    danger: { background: 'rgba(248,113,113,.14)', color: C.danger },
  }
  return <button style={{ ...base, ...variants[variant], ...style }}
    onMouseDown={e => e.currentTarget.style.filter = 'brightness(.9)'}
    onMouseUp={e => e.currentTarget.style.filter = 'none'}
    onMouseLeave={e => e.currentTarget.style.filter = 'none'} {...rest}>{children}</button>
}

export function Field({ style, ...rest }) {
  return <input style={{ padding: '11px 14px', background: C.surface2, border: `1px solid ${C.border}`,
    borderRadius: 12, color: C.text, fontSize: 14, outline: 'none', width: '100%', ...style }}
    onFocus={e => e.target.style.borderColor = C.accent}
    onBlur={e => e.target.style.borderColor = C.border} {...rest} />
}

export function Chip({ children, color, style }) {
  return <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12,
    padding: '3px 10px', borderRadius: 999, background: color ? `${color}22` : C.surface2,
    color: color ?? C.muted, border: `1px solid ${color ? color + '44' : C.border}`, ...style }}>{children}</span>
}

export function Dot({ color, size = 10 }) {
  return <span style={{ width: size, height: size, borderRadius: '50%', background: color,
    display: 'inline-block', flexShrink: 0, boxShadow: `0 0 8px ${color}88` }} />
}

export function Modal({ open, onClose, title, children, width = 460 }) {
  if (!open) return null
  return (
    <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(3,5,10,.62)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000, backdropFilter: 'blur(2px)' }}>
      <Card onClick={e => e.stopPropagation()} style={{ width, maxWidth: '92vw', padding: 26,
        animation: 'floatIn .18s ease' }}>
        {title && <h2 style={{ margin: '0 0 18px', fontSize: 19, fontWeight: 700 }}>{title}</h2>}
        {children}
      </Card>
    </div>
  )
}
