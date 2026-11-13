import { useEffect, useRef, useCallback } from 'react'

// The live socket shares the API base URL so the Vite dev server (which has no
// proxy configured) can still reach the Go service when VITE_API_URL is set.
function liveUrl(token) {
  const base = import.meta.env.VITE_API_URL ?? '/v1'
  const url = new URL(base, window.location.href)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.pathname = url.pathname.replace(/\/+$/, '') + '/live'
  url.search = `?token=${encodeURIComponent(token)}`
  url.hash = ''
  return url.toString()
}

/**
 * useSocket connects to the /live WebSocket endpoint and calls onMessage with
 * each parsed JSON event. Reconnects automatically with exponential back-off.
 */
export function useSocket(onMessage) {
  const ws = useRef(null)
  const backoff = useRef(1000)
  const reconnectTimer = useRef(null)
  const stopped = useRef(false)
  const onMessageRef = useRef(onMessage)
  onMessageRef.current = onMessage

  const connect = useCallback(() => {
    if (stopped.current) return
    const token = localStorage.getItem('access_token')
    if (!token) return

    const socket = new WebSocket(liveUrl(token))
    ws.current = socket

    socket.onopen = () => {
      backoff.current = 1000
    }

    socket.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data)
        onMessageRef.current(msg)
      } catch {
        // ignore malformed frames
      }
    }

    socket.onclose = () => {
      ws.current = null
      if (stopped.current) return
      const delay = backoff.current
      backoff.current = Math.min(backoff.current * 2, 30_000)
      reconnectTimer.current = setTimeout(connect, delay)
    }

    socket.onerror = () => socket.close()
  }, [])

  useEffect(() => {
    stopped.current = false
    connect()
    return () => {
      stopped.current = true
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      ws.current?.close()
    }
  }, [connect])
}
