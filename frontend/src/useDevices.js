import { useCallback } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import api from './api'
import { useSocket } from './useSocket'

// Shared live device feed: initial + 30s poll, plus live WS position patches.
export function useLiveDevices() {
  const qc = useQueryClient()
  const query = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.get('/devices').then(r => r.data),
    refetchInterval: 30_000,
  })

  const onMessage = useCallback((msg) => {
    if (msg.type !== 'position') return
    qc.setQueryData(['devices'], (prev = []) =>
      prev.map(d => d.id === msg.device_id
        ? { ...d, last_lat: msg.lat, last_lon: msg.lon, last_speed: msg.speed,
            last_recorded_at: msg.recorded_at,
            status: (msg.speed ?? 0) > (d.trip_stop_speed_kmh ?? 2) ? 'moving' : 'stopped' }
        : d),
    )
    // A brand-new device we do not know about yet → pull the fresh list.
    qc.setQueryData(['devices'], (prev = []) => {
      if (!prev.some(d => d.id === msg.device_id)) qc.invalidateQueries({ queryKey: ['devices'] })
      return prev
    })
  }, [qc])

  useSocket(onMessage)
  return query.data ?? []
}
