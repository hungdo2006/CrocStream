import { useCallback, useEffect, useRef, useState } from 'react'
import { api, type Display } from '../api'

export function useDisplays(enabled: boolean, setError: (message: string) => void, message: string) {
  const [displays, setDisplays] = useState<Display[]>([])
  const [display, setDisplay] = useState('')
  const revision = useRef(0)
  const refresh = useCallback(async () => {
    const current = ++revision.current
    try {
      const list = await api.displays()
      if (current !== revision.current) return
      setDisplays(list)
      setDisplay(previous => list.some(d => d.id === previous) ? previous : (list.find(d => d.primary)?.id ?? list[0]?.id ?? ''))
    } catch { if (current === revision.current) setError(message) }
  }, [setError, message])
  useEffect(() => {
    if (enabled) void refresh()
    return () => { revision.current++ }
  }, [enabled, refresh])
  return { displays, display, setDisplay, refresh }
}
