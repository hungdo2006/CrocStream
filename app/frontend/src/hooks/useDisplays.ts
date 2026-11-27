import { useCallback, useEffect, useRef, useState } from 'react'
import { api, type Display } from '../api'

const AUTO_REFRESH_COOLDOWN_MS = 150

export function useDisplays(
  enabled: boolean,
  setError: (message: string) => void,
  message: string,
  autoRefresh = enabled,
) {
  const [displays, setDisplays] = useState<Display[]>([])
  const [display, setDisplay] = useState('')
  const revision = useRef(0)
  const inFlight = useRef(false)
  const lastAutoAt = useRef(0)

  const refresh = useCallback(async () => {
    const current = ++revision.current
    inFlight.current = true
    try {
      const list = await api.displays()
      if (current !== revision.current) return
      setDisplays(list)
      setDisplay(previous => list.some(d => d.id === previous) ? previous : (list.find(d => d.primary)?.id ?? list[0]?.id ?? ''))
    } catch {
      if (current === revision.current) setError(message)
    } finally {
      if (current === revision.current) inFlight.current = false
    }
  }, [setError, message])

  const refreshAuto = useCallback(() => {
    const now = Date.now()
    if (inFlight.current || now - lastAutoAt.current < AUTO_REFRESH_COOLDOWN_MS) return
    lastAutoAt.current = now
    void refresh()
  }, [refresh])

  useEffect(() => {
    if (enabled) void refresh()
    return () => {
      revision.current++
      inFlight.current = false
    }
  }, [enabled, refresh])

  useEffect(() => {
    if (!enabled || !autoRefresh) return
    const onFocus = () => refreshAuto()
    const onVisibility = () => {
      if (document.visibilityState === 'visible') refreshAuto()
    }
    window.addEventListener('focus', onFocus)
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      window.removeEventListener('focus', onFocus)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [enabled, autoRefresh, refreshAuto])

  return { displays, display, setDisplay, refresh }
}
