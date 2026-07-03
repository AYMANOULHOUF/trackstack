import { useCallback } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import api from './api'
import { useSocket } from './useSocket'

export function useLiveDevices() {
  const qc = useQueryClient()
  const query = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.get('/devices').then(r => r.data),
    refetchInterval: 30_000,
  })

  const onMessage = useCallback((msg) => {
    if (msg.type === 'position') {
      qc.setQueryData(['devices'], (prev = []) => {
        if (!prev.some(d => d.id === msg.device_id)) {
          qc.invalidateQueries({ queryKey: ['devices'] })
          return prev
        }
        return prev.map(d => d.id === msg.device_id
          ? { ...d,
              last_lat: msg.lat, last_lon: msg.lon, last_speed: msg.speed,
              last_recorded_at: msg.recorded_at,
              tracking_active: true,
              status: (msg.speed ?? 0) > (d.trip_stop_speed_kmh ?? 2) ? 'moving' : 'stopped',
            }
          : d)
      })
    } else if (msg.type === 'tracking_state') {
      qc.setQueryData(['devices'], (prev = []) =>
        prev.map(d => d.id === msg.device_id
          ? { ...d, tracking_active: msg.tracking_active,
              status: msg.tracking_active
                ? ((d.last_speed ?? 0) > (d.trip_stop_speed_kmh ?? 2) ? 'moving' : 'stopped')
                : 'paused' }
          : d))
    }
  }, [qc])

  useSocket(onMessage)
  return query.data ?? []
}
