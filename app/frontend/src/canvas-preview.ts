import { useCallback, useEffect, useRef, useState } from 'react'
import type { Endpoint } from '../bindings/crocstream.local/app/internal/bridge/models'
import { LatestFrames } from './latest'

const maxJPEG = 4 * 1024 * 1024

interface PreviewMessages {
  decodeFailed: string
  connectionFailed: string
  connectionClosed: string
}

// Each effect owns one socket and its pending decodes for a particular endpoint.
export function useCanvasPreview(
  endpoint: Endpoint | undefined,
  messages: PreviewMessages,
  attempt = 0,
) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const renders = useRef<number[]>([])
  const [hasImage, setHasImage] = useState(false)
  const [imageError, setImageError] = useState('')
  const [renderFPS, setRenderFPS] = useState(0)

  const trimRenders = useCallback((now: number) => {
    renders.current = renders.current.filter(time => time > now - 1000)
  }, [])
  const sampleRenderFPS = useCallback(() => {
    trimRenders(performance.now())
    setRenderFPS(renders.current.length)
  }, [trimRenders])

  useEffect(() => {
    const target = canvas.current
    const ctx = target?.getContext('2d')
    renders.current = []
    setHasImage(false)
    if (!target || !ctx) return

    const clear = () => ctx.clearRect(0, 0, target.width, target.height)
    clear()
    if (!endpoint) return

    const frames = new LatestFrames(
      blob => createImageBitmap(blob),
      bitmap => {
        clear()
        const scale = Math.min(target.width / bitmap.width, target.height / bitmap.height)
        const width = bitmap.width * scale
        const height = bitmap.height * scale
        ctx.drawImage(bitmap, (target.width - width) / 2, (target.height - height) / 2, width, height)
        const now = performance.now()
        renders.current.push(now)
        trimRenders(now)
        setImageError('')
        setHasImage(true)
      },
      () => setImageError(messages.decodeFailed),
    )

    const socket = new WebSocket(endpoint.url, ['crocstream.preview', `auth.${endpoint.token}`])
    socket.binaryType = 'blob'
    socket.onmessage = event => {
      if (event.data instanceof Blob && event.data.size <= maxJPEG) {
        frames.push(new Blob([event.data], { type: 'image/jpeg' }))
      }
    }
    socket.onerror = () => setImageError(messages.connectionFailed)
    socket.onclose = () => setImageError(messages.connectionClosed)

    return () => {
      frames.dispose()
      socket.onmessage = socket.onerror = socket.onclose = null
      socket.close()
      clear()
      renders.current = []
    }
  }, [endpoint, messages, attempt, trimRenders])

  return { canvas, hasImage, imageError, setImageError, renderFPS, sampleRenderFPS }
}
