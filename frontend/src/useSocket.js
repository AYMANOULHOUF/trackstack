import { useEffect, useRef, useCallback } from 'react'

/**
 * useSocket connects to /v1/live (Step 15) and calls onMessage with each
 * parsed JSON event. Reconnects automatically with exponential back-off.
 */
export function useSocket(onMessage) {
  const ws = useRef(null)
  const backoff = useRef(1000)
  const onMessageRef = useRef(onMessage)
  onMessageRef.current = onMessage

  const connect = useCallback(() => {
    const token = localStorage.getItem('access_token')
    if (!token) return

    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const url = `${proto}://${window.location.host}/v1/live?token=${token}`
    const socket = new WebSocket(url)
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
      setTimeout(() => {
        backoff.current = Math.min(backoff.current * 2, 30_000)
        connect()
      }, backoff.current)
    }

    socket.onerror = () => socket.close()
  }, [])

  useEffect(() => {
    connect()
    return () => ws.current?.close()
  }, [connect])
}
