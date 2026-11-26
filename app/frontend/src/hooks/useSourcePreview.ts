import { useCallback, useEffect, useRef, useState } from 'react'
import { api, type Started } from '../api'

// Serializes pre-share local preview lifecycles so switching monitors/windows
// or transitioning into StartBroadcast never races against backend mutual exclusion.
export function useSourcePreview(
  enabled: boolean,
  displayID: string,
  width: number,
  height: number,
  fps: number,
) {
  const [started, setStarted] = useState<Started | undefined>(undefined)
  const [busy, setBusy] = useState(false)
  const activeRef = useRef(false)
  const suspendedRef = useRef(false)
  const mountedRef = useRef(true)
  const seqRef = useRef(0)
  const queueRef = useRef<Promise<void>>(Promise.resolve())

  const enqueue = useCallback((task: () => Promise<void>) => {
    const next = queueRef.current.then(task, task)
    queueRef.current = next
    return next
  }, [])

  const stopNow = useCallback(() => {
    seqRef.current++
    setStarted(undefined)
    return enqueue(async () => {
      if (!activeRef.current) return
      activeRef.current = false
      try {
        await api.stop()
      } catch {
        // Ignore stop errors if preview already stopped.
      }
    })
  }, [enqueue])

  const suspendAndStop = useCallback(async () => {
    suspendedRef.current = true
    await stopNow()
  }, [stopNow])

  const resume = useCallback(() => {
    suspendedRef.current = false
  }, [])

  useEffect(() => {
    mountedRef.current = true
    suspendedRef.current = false
    return () => {
      mountedRef.current = false
      suspendedRef.current = true
      seqRef.current++
      void enqueue(async () => {
        if (!activeRef.current) return
        activeRef.current = false
        try {
          await api.stop()
        } catch {
          // Best-effort cleanup on unmount.
        }
      })
    }
  }, [enqueue])

  useEffect(() => {
    if (!enabled || !displayID || suspendedRef.current) {
      void stopNow()
      return
    }
    const seq = ++seqRef.current
    const previewFPS = Math.min(fps, 15)
    const timer = setTimeout(() => {
      if (!mountedRef.current || seq !== seqRef.current || suspendedRef.current) return
      void enqueue(async () => {
        if (!mountedRef.current || seq !== seqRef.current || suspendedRef.current) return
        if (activeRef.current) {
          activeRef.current = false
          if (mountedRef.current) setStarted(undefined)
          try {
            await api.stop()
          } catch {
            // Continue attempting new source preview.
          }
        }
        if (!mountedRef.current || seq !== seqRef.current || suspendedRef.current) return
        if (mountedRef.current) setBusy(true)
        try {
          const next = await api.start({ displayID, width, height, fps: previewFPS, tau: 4, keyInterval: 30 })
          activeRef.current = true
          if (!mountedRef.current || seq !== seqRef.current || suspendedRef.current) {
            activeRef.current = false
            try {
              await api.stop()
            } catch {
              // Ignore late cancellation cleanup error.
            }
            return
          }
          setStarted(next)
        } catch {
          if (mountedRef.current && seq === seqRef.current) setStarted(undefined)
        } finally {
          if (mountedRef.current && seq === seqRef.current) setBusy(false)
        }
      })
    }, 60)

    return () => {
      clearTimeout(timer)
    }
  }, [enabled, displayID, width, height, fps, enqueue, stopNow])

  return { started, busy, suspendAndStop, resume }
}
