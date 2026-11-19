import { useEffect, useRef, useState } from 'react'
import * as Service from '../bindings/crocstream.local/app/internal/desktop/service'
import { State, type Started, type Stats } from '../bindings/crocstream.local/app/internal/preview/models'
import { useCanvasPreview } from './canvas-preview'
import { CaptureSettings, useCaptureSettings } from './capture-settings'

const names: Record<State, string> = {
  [State.$zero]: 'Chưa chụp màn hình',
  Idle: 'Chưa chụp màn hình', Preparing: 'Đang chuẩn bị', Previewing: 'Đang xem trước cục bộ',
  Stopping: 'Đang dừng và giải phóng tài nguyên', Error: 'Preview gặp lỗi',
}

const previewMessages = {
  decodeFailed: 'Preview unavailable · Không giải mã được ảnh cục bộ.',
  connectionFailed: 'Preview unavailable · Kết nối loopback thất bại.',
  connectionClosed: 'Kết nối preview đã đóng. Dừng rồi bắt đầu lại để thử lại.',
}

export function LocalPreview({ onHome }: { onHome: () => void }) {
  const settings = useCaptureSettings('local')
  const { refresh } = settings
  const [state, setState] = useState(State.Idle)
  const [error, setError] = useState('')
  const [session, setSession] = useState<Started>()
  const [stats, setStats] = useState<Stats>()
  const { canvas, imageError, setImageError, renderFPS, sampleRenderFPS } =
    useCanvasPreview(session?.endpoint, previewMessages)
  const generation = useRef(0)
  const active = useRef(false)
  const pendingOperation = useRef(false)
  const starting = useRef<Promise<Started> | undefined>(undefined)
  useEffect(() => {
    let mounted = true
    void refresh(setError, { shouldApply: () => mounted })
    let timer: ReturnType<typeof setTimeout>
    const poll = async () => {
      const epoch = generation.current
      try {
        const next = await Service.GetStats()
        if (!mounted) return
        if (epoch === generation.current && !pendingOperation.current) {
          setStats(next)
          setState(next.state)
          if (next.error) setError(next.error)
          if (next.state === State.Idle || next.state === State.Failed) {
            active.current = false
            setSession(undefined)
          }
        }
        sampleRenderFPS()
      } catch { if (mounted) setError('Không đọc được trạng thái ứng dụng.') }
      if (mounted) timer = setTimeout(() => { void poll() }, 500)
    }
    void poll()
    return () => {
      mounted = false
      clearTimeout(timer)
      generation.current++
      if (active.current) {
        void (starting.current ?? Promise.resolve()).catch(() => {}).then(() => Service.StopPreview()).catch(() => {})
      }
    }
  }, [refresh, sampleRenderFPS])

  const start = async () => {
    if (active.current) return
    active.current = true
    pendingOperation.current = true
    const current = ++generation.current
    setError('')
    setImageError('')
    setState(State.Preparing)
    try {
      const request = Service.StartPreview(settings.options())
      starting.current = request
      const next = await request
      if (current !== generation.current) return // Stop already owns cleanup.
      setSession(next)
    } catch {
      if (current === generation.current) {
        active.current = false
        setState(State.Failed)
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
    setState(State.Stopping)
    setSession(undefined)
    try {
      // A Stop click during preparation must follow the outstanding Start RPC,
      // otherwise it could stop nothing and allow the late Start to capture.
      await starting.current?.catch(() => {})
      await Service.StopPreview()
      active.current = false
      setState(State.Idle)
      setImageError('')
      setError('')
      return true
    } catch {
      setError('Dừng quá thời gian; đang chờ Windows giải phóng capture.')
      setState(State.Stopping)
      return false
    } finally {
      pendingOperation.current = false
    }
  }
  const locked = state === State.Preparing || state === State.Previewing || state === State.Stopping
  const measured = stats && session && stats.generation === session.generation && stats.captured > 0
  return <><nav><button onClick={()=>void stop().then(done=>{if(done)onHome()})}>← Dừng phiên và về Home</button></nav><main>
    <header><div><span className="eyebrow">CrocStream</span><h1>Xem trước</h1></div>
      <span className={`status ${locked ? 'active' : ''}`} role="status">{names[state]}</span></header>
    <div className="workspace">
      <section className="viewer" aria-labelledby="preview-title">
        <div className="section-heading"><h2 id="preview-title">Màn hình của bạn</h2><span>Chỉ trên máy này</span></div>
        <div className="canvas-wrap"><canvas ref={canvas} width={1280} height={720} aria-label="Ảnh xem trước màn hình đã chọn" />
          {!session && <div className="placeholder"><strong>Chưa có hình xem trước</strong><p>Chọn màn hình bên phải để bắt đầu.</p></div>}
        </div>
        <p className="caption">Xem thử nguồn chụp. Không gửi ảnh tới máy khác.</p>
        {imageError && <p role="status" className="error">{imageError}</p>}
        <div className="metrics" aria-label="Số đo preview">
          <Metric label="Capture FPS" value={measured ? String(stats.captureFPS) : '—'} />
          <Metric label="JPEG FPS" value={measured ? String(stats.encodeFPS) : '—'} />
          <Metric label="Render FPS" value={measured ? String(renderFPS) : '—'} />
          <Metric label="Mẫu bỏ trước JPEG" value={measured ? String(stats.dropped) : '—'} />
        </div>
        <p className="caption">FPS đo trong 1 giây gần nhất. RTT / savings mạng: N/A.</p>
      </section>
      <aside><h2>Thiết lập nguồn</h2>
        <CaptureSettings settings={settings} locked={locked}
          onRefresh={() => void refresh(setError)} />
        <p className="notice">Bắt đầu sẽ chụp màn hình đã chọn. Ảnh chỉ ở RAM; không lưu hoặc gửi ra Internet.</p>
        <button className="primary" disabled={locked || !settings.display} onClick={() => { void start() }}>Bắt đầu preview</button>
        <button className="stop" disabled={!locked} onClick={() => { void stop() }}>Dừng preview</button>
        {error && <p role="alert" className="error">{error}</p>}
      </aside>
    </div>
  </main></>
}
function Metric({ label, value }: { label: string; value: string }) {
  return <div><span>{label}</span><strong>{value}</strong></div>
}
