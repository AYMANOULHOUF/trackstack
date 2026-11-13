import { useEffect, useRef, useState } from 'react'
import { C } from '../ui'

// Lightweight SVG charts — no charting dependency, no canvas, crisp on retina.
// Width is measured so strokes and labels never get stretched.

function useWidth() {
  const ref = useRef(null)
  const [width, setWidth] = useState(0)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const ro = new ResizeObserver(([entry]) => setWidth(entry.contentRect.width))
    ro.observe(el)
    setWidth(el.clientWidth)
    return () => ro.disconnect()
  }, [])
  return [ref, width]
}

export function LineChart({ points, series, height = 200, emptyText = 'No fixes in this window yet.' }) {
  const [ref, width] = useWidth()
  const padL = 34, padR = 10, padT = 10, padB = 22
  const w = Math.max(width, 240)
  const innerW = Math.max(10, w - padL - padR)
  const innerH = Math.max(10, height - padT - padB)

  const usable = points.filter(p => p.t)
  if (usable.length < 2) {
    return (
      <div ref={ref} style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <span style={{ color: C.muted, fontSize: 12.5 }}>{emptyText}</span>
      </div>
    )
  }

  const t0 = usable[0].t
  const t1 = usable[usable.length - 1].t
  const span = Math.max(1, t1 - t0)
  const maxV = Math.max(
    1,
    ...series.flatMap(s => usable.map(p => p[s.key]).filter(v => v != null && isFinite(v))),
  )
  const niceMax = Math.ceil(maxV / 5) * 5 || 5

  const x = t => padL + ((t - t0) / span) * innerW
  const y = v => padT + innerH - (Math.max(0, v) / niceMax) * innerH

  const path = key => usable
    .filter(p => p[key] != null && isFinite(p[key]))
    .map((p, i) => `${i === 0 ? 'M' : 'L'}${x(p.t).toFixed(1)},${y(p[key]).toFixed(1)}`)
    .join(' ')

  const areaKey = series[0]?.key
  const areaPts = usable.filter(p => p[areaKey] != null && isFinite(p[areaKey]))
  const area = areaPts.length > 1
    ? `${path(areaKey)} L${x(areaPts[areaPts.length - 1].t).toFixed(1)},${padT + innerH} L${x(areaPts[0].t).toFixed(1)},${padT + innerH} Z`
    : ''

  const ticks = [0, 0.5, 1].map(f => Math.round(niceMax * f))
  const fmtTime = t => new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

  return (
    <div ref={ref} style={{ width: '100%' }}>
      <svg width={w} height={height} style={{ display: 'block', overflow: 'visible' }}>
        <defs>
          <linearGradient id="speedFill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={C.accent} stopOpacity="0.26" />
            <stop offset="100%" stopColor={C.accent} stopOpacity="0" />
          </linearGradient>
        </defs>

        {ticks.map(v => (
          <g key={v}>
            <line x1={padL} x2={w - padR} y1={y(v)} y2={y(v)} stroke={C.grid} strokeWidth="1" />
            <text x={padL - 8} y={y(v) + 3.5} textAnchor="end"
              fontSize="10" fill={C.muted} fontVariant="tabular-nums">{v}</text>
          </g>
        ))}

        {area && <path d={area} fill="url(#speedFill)" />}
        {series.map(s => (
          <path key={s.key} d={path(s.key)} fill="none" stroke={s.color}
            strokeWidth={s.width ?? 2} strokeLinejoin="round" strokeLinecap="round"
            strokeDasharray={s.dash} />
        ))}

        <text x={padL} y={height - 4} fontSize="10" fill={C.muted}>{fmtTime(t0)}</text>
        <text x={w - padR} y={height - 4} fontSize="10" fill={C.muted} textAnchor="end">{fmtTime(t1)}</text>
        <text x={padL + innerW / 2} y={height - 4} fontSize="10" fill={C.muted} textAnchor="middle">
          {new Date((t0 + t1) / 2).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
        </text>
      </svg>
    </div>
  )
}

export function Donut({ slices, centerLabel, centerValue, size = 168 }) {
  const stroke = 20
  const r = (size - stroke) / 2
  const cx = size / 2
  const cy = size / 2
  const circumference = 2 * Math.PI * r
  const sum = slices.reduce((s, x) => s + x.value, 0)

  let offset = 0
  return (
    <div style={{ position: 'relative', width: size, height: size, flexShrink: 0 }}>
      <svg width={size} height={size} style={{ transform: 'rotate(-90deg)' }}>
        <circle cx={cx} cy={cy} r={r} fill="none" stroke={C.surface2} strokeWidth={stroke} />
        {sum > 0 && slices.map((s, i) => {
          const len = (s.value / sum) * circumference
          const el = (
            <circle key={i} cx={cx} cy={cy} r={r} fill="none" stroke={s.color} strokeWidth={stroke}
              strokeDasharray={`${Math.max(0, len - 2)} ${circumference}`}
              strokeDashoffset={-offset} strokeLinecap="butt" />
          )
          offset += len
          return el
        })}
      </svg>
      <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column',
        alignItems: 'center', justifyContent: 'center', pointerEvents: 'none' }}>
        <div style={{ fontSize: 22, fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>{centerValue}</div>
        <div style={{ fontSize: 10.5, color: C.muted, textTransform: 'uppercase', letterSpacing: .6,
          fontWeight: 700 }}>{centerLabel}</div>
      </div>
    </div>
  )
}
