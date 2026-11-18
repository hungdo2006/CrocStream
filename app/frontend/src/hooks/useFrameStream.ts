import { useLayoutEffect, useRef, useState } from 'react'
import type { Started } from '../api'
import { LatestFrames } from '../latest'

const errors = {
  preview: {
    decode: 'Preview unavailable · Không giải mã được ảnh cục bộ.',
    socket: 'Preview unavailable · Kết nối loopback thất bại.',
    closed: 'Kết nối preview đã đóng. Dừng rồi bắt đầu lại để thử lại.',
  },
  stream: {
    decode: 'Preview unavailable · Không giải mã được JPEG cục bộ.',
    socket: 'Preview unavailable · Lỗi WebSocket cục bộ.',
    closed: 'Preview unavailable · Kênh hiển thị đã đóng; có thể thử nối lại.',
  },
}

// Pixels stay outside React. Only first-image/error transitions and a 500-ms
// numeric sample update state; at most one decode and one newest blob are kept.
export function useFrameStream(started: Started | undefined, mode: keyof typeof errors) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const [error, setError] = useState('')
  const [hasImage, setHasImage] = useState(false)
  const [renderFPS, setRenderFPS] = useState(0)
  const [attempt, setAttempt] = useState(0)
  const reconnectAt = useRef(0)
  const url = started?.endpoint.url, token = started?.endpoint.token, generation = started?.generation
  useLayoutEffect(() => {
    const target = canvas.current, ctx = target?.getContext('2d')
    setError(''); setHasImage(false); setRenderFPS(0)
    if (!target || !ctx) return
    ctx.clearRect(0, 0, target.width, target.height)
    if (!url || !token) return
    let visible = false, failure = '', lastFPS = 0
    const rendered: number[] = []
    const report = (message: string) => {
      if (failure !== message) { failure = message; setError(message) }
    }
    const expire = (now: number) => {
      while (rendered.length && rendered[0] <= now - 1000) rendered.shift()
    }
    const frames = new LatestFrames(
      // Only the selected blob needs a MIME wrapper; queued replacements don't.
      blob => createImageBitmap(blob.type === 'image/jpeg' ? blob : blob.slice(0, blob.size, 'image/jpeg')),
      bitmap => {
        ctx.clearRect(0, 0, target.width, target.height)
        const scale = Math.min(target.width / bitmap.width, target.height / bitmap.height)
        const w = bitmap.width * scale, h = bitmap.height * scale
        ctx.drawImage(bitmap, (target.width - w) / 2, (target.height - h) / 2, w, h)
        const now = performance.now()
        expire(now); rendered.push(now)
        report('')
        if (!visible) { visible = true; setHasImage(true) }
      },
      () => report(errors[mode].decode),
      run => { const id = requestAnimationFrame(run); return () => cancelAnimationFrame(id) },
    )
    const timer = setInterval(() => {
      expire(performance.now())
      if (lastFPS !== rendered.length) { lastFPS = rendered.length; setRenderFPS(lastFPS) }
    }, 500)
    let socket: WebSocket | undefined
    try {
      socket = new WebSocket(url, ['crocstream.preview', `auth.${token}`])
      socket.binaryType = 'blob'
      socket.onmessage = event => {
        if (event.data instanceof Blob && event.data.size <= 4 * 1024 * 1024) frames.push(event.data)
      }
      socket.onerror = () => report(errors[mode].socket)
      socket.onclose = () => { frames.dispose(); report(errors[mode].closed) }
    } catch { frames.dispose(); report(errors[mode].socket) }
    return () => {
      clearInterval(timer)
      frames.dispose()
      if (socket) { socket.onmessage = socket.onerror = socket.onclose = null; socket.close() }
      ctx.clearRect(0, 0, target.width, target.height)
    }
  }, [url, token, generation, attempt, mode])
  const reconnect = () => {
    if (!started || performance.now() < reconnectAt.current) return
    reconnectAt.current = performance.now() + 2000
    setAttempt(n => n + 1)
  }
  return { canvas, error, hasImage, renderFPS, reconnect }
}
