import React, { useEffect, useRef, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { api, type Display, type Started, type State, type Stats } from './api'
import { LatestFrames } from './latest'
import './style.css'

const names: Record<State, string> = {
  Idle: 'Chưa chụp màn hình', Preparing: 'Đang chuẩn bị', Previewing: 'Đang xem trước cục bộ',
  Stopping: 'Đang dừng và giải phóng tài nguyên', Error: 'Preview gặp lỗi',
}

function App() {
  const [displays, setDisplays] = useState<Display[]>([])
  const [display, setDisplay] = useState('')
  const [resolution, setResolution] = useState('1280x720')
  const [fps, setFPS] = useState(24)
  const [state, setState] = useState<State>('Idle')
  const [error, setError] = useState('')
  const [previewError, setPreviewError] = useState('')
  const [session, setSession] = useState<Started>()
  const [stats, setStats] = useState<Stats>()
  const [renderFPS, setRenderFPS] = useState(0)
  const canvas = useRef<HTMLCanvasElement>(null)
  const generation = useRef(0)
  const active = useRef(false)
  const pendingOperation = useRef(false)
  const starting = useRef<Promise<Started> | undefined>(undefined)
  const rendered = useRef<number[]>([])
  const refresh = async () => {
    try {
      const list = await api.displays()
      setDisplays(list)
      setDisplay(previous => list.some(d => d.id === previous) ? previous : (list.find(d => d.primary)?.id ?? list[0]?.id ?? ''))
    } catch { setError('Không đọc được danh sách màn hình. Hãy thử làm mới.') }
  }
  useEffect(() => {
    void refresh()
    let mounted = true
    let timer: ReturnType<typeof setTimeout>
    const poll = async () => {
      const epoch = generation.current
      try {
        const next = await api.stats()
        if (!mounted) return
        if (epoch === generation.current && !pendingOperation.current) {
          setStats(next)
          setState(next.state)
          if (next.error) setError(next.error)
          if (next.state === 'Idle' || next.state === 'Error') {
            active.current = false
            setSession(undefined)
          }
        }
        const cutoff = performance.now() - 1000
        rendered.current = rendered.current.filter(t => t > cutoff)
        setRenderFPS(rendered.current.length)
      } catch { if (mounted) setError('Không đọc được trạng thái ứng dụng.') }
      if (mounted) timer = setTimeout(() => { void poll() }, 500)
    }
    void poll()
    return () => {
      mounted = false
      clearTimeout(timer)
      generation.current++
      if (active.current) {
        void (starting.current ?? Promise.resolve()).catch(() => {}).then(() => api.stop()).catch(() => {})
      }
    }
  }, [])

  useEffect(() => {
    const target = canvas.current
    if (!target) return
    const ctx = target.getContext('2d')
    ctx?.clearRect(0, 0, target.width, target.height)
    rendered.current = []
    if (!session || !ctx) return
    const frames = new LatestFrames(blob => createImageBitmap(blob), bitmap => {
      ctx.clearRect(0, 0, target.width, target.height)
      const scale = Math.min(target.width / bitmap.width, target.height / bitmap.height)
      const w = bitmap.width * scale, h = bitmap.height * scale
      ctx.drawImage(bitmap, (target.width - w) / 2, (target.height - h) / 2, w, h)
      rendered.current.push(performance.now())
      rendered.current = rendered.current.filter(t => t > performance.now() - 1000)
      setPreviewError('')
    }, () => setPreviewError('Preview unavailable · Không giải mã được ảnh cục bộ.'))
    const socket = new WebSocket(session.endpoint.url, ['crocstream.preview', `auth.${session.endpoint.token}`])
    socket.binaryType = 'blob'
    socket.onmessage = event => {
      if (event.data instanceof Blob && event.data.size <= 4 * 1024 * 1024) {
        frames.push(new Blob([event.data], { type: 'image/jpeg' }))
      }
    }
    socket.onerror = () => setPreviewError('Preview unavailable · Kết nối loopback thất bại.')
    socket.onclose = () => setPreviewError('Kết nối preview đã đóng. Dừng rồi bắt đầu lại để thử lại.')
    return () => {
      frames.dispose()
      socket.onmessage = socket.onerror = socket.onclose = null
      socket.close()
      ctx.clearRect(0, 0, target.width, target.height)
      rendered.current = []
    }
  }, [session])

  const start = async () => {
    if (active.current) return
    active.current = true
    pendingOperation.current = true
    const current = ++generation.current
    setError(''); setPreviewError(''); setState('Preparing')
    const [width, height] = resolution.split('x').map(Number)
    try {
      const request = api.start({ displayID: display, width, height, fps })
      starting.current = request
      const next = await request
      if (current !== generation.current) return // Stop already owns cleanup.
      setSession(next)
    } catch {
      if (current === generation.current) {
        active.current = false; setState('Error')
        setError('Không bắt đầu được preview. Kiểm tra màn hình và trạng thái rồi thử lại.')
      }
    } finally {
      starting.current = undefined
      if (current === generation.current) pendingOperation.current = false
    }
  }
  const stop = async () => {
    generation.current++
    pendingOperation.current = true
    setState('Stopping')
    setSession(undefined)
    try {
      // A Stop click during preparation must follow the outstanding Start RPC,
      // otherwise it could stop nothing and allow the late Start to capture.
      await starting.current?.catch(() => {})
      await api.stop(); active.current = false; setState('Idle')
    }
    catch { setError('Dừng quá thời gian; đang chờ Windows giải phóng capture.'); setState('Stopping') }
    finally { pendingOperation.current = false }
  }
  const locked = state === 'Preparing' || state === 'Previewing' || state === 'Stopping'
  const measured = stats && stats.captured > 0
  return <main>
    <header><div><span className="eyebrow">CROCSTREAM / G3</span><h1>Màn hình của bạn.<br /><span>Xem trước, ngay tại đây.</span></h1></div>
      <span className={`status ${locked ? 'active' : ''}`} role="status">{names[state]}</span></header>
    <div className="workspace">
      <section className="viewer" aria-labelledby="preview-title">
        <div className="section-heading"><h2 id="preview-title">Local preview</h2><span>Chỉ trên máy này</span></div>
        <div className="canvas-wrap"><canvas ref={canvas} width={1280} height={720} aria-label="Ảnh xem trước màn hình đã chọn" />
          {!session && <div className="placeholder"><strong>Sẵn sàng khi bạn muốn.</strong><p>Chọn màn hình, rồi bấm Bắt đầu preview.</p></div>}
        </div>
        <p className="caption">Ảnh JPEG cục bộ qua WebSocket loopback. Chưa phát tới người xem từ xa.</p>
        {previewError && <p role="status" className="error">{previewError}</p>}
        <div className="metrics" aria-label="Số đo preview">
          <Metric label="Capture FPS" value={measured ? String(stats.captureFPS) : '—'} />
          <Metric label="JPEG FPS" value={measured ? String(stats.encodeFPS) : '—'} />
          <Metric label="Render FPS" value={measured ? String(renderFPS) : '—'} />
          <Metric label="Mẫu bỏ trước JPEG" value={measured ? String(stats.dropped) : '—'} />
        </div>
        <p className="caption">FPS đo trong 1 giây gần nhất. RTT / savings mạng: N/A.</p>
      </section>
      <aside><h2>Thiết lập nguồn</h2>
        <label htmlFor="display">Màn hình</label>
        <select id="display" value={display} disabled={locked} onChange={e => setDisplay(e.target.value)}>
          {!displays.length && <option value="">Chưa tìm thấy màn hình</option>}
          {displays.map((d, i) => <option key={d.id} value={d.id}>Màn hình {i + 1}{d.primary ? ' · Chính' : ''} — {d.width}×{d.height}</option>)}
        </select>
        <button className="secondary" disabled={locked} onClick={() => { void refresh() }}>Làm mới danh sách</button>
        <label htmlFor="resolution">Độ phân giải</label>
        <select id="resolution" disabled={locked} value={resolution} onChange={e => setResolution(e.target.value)}>
          <option value="1280x720">1280 × 720</option><option value="640x360">640 × 360</option>
        </select>
        <label htmlFor="fps">FPS mục tiêu</label>
        <select id="fps" disabled={locked} value={fps} onChange={e => setFPS(Number(e.target.value))}>
          {[15, 24, 30].map(value => <option key={value} value={value}>{value} FPS</option>)}
        </select>
        <p className="notice">Bắt đầu sẽ chụp màn hình đã chọn. Ảnh chỉ ở RAM; không lưu hoặc gửi ra Internet.</p>
        <button className="primary" disabled={locked || !display} onClick={() => { void start() }}>Bắt đầu preview</button>
        <button className="stop" disabled={!locked} onClick={() => { void stop() }}>Dừng preview</button>
        {error && <p role="alert" className="error">{error}</p>}
      </aside>
    </div>
    <footer>01 / Chọn nguồn <span>02 / Bắt đầu</span><span>03 / Dừng để giải phóng capture</span></footer>
  </main>
}
function Metric({ label, value }: { label: string; value: string }) {
  return <div><span>{label}</span><strong>{value}</strong></div>
}
createRoot(document.getElementById('root')!).render(<React.StrictMode><App /></React.StrictMode>)
