import { useEffect, useRef, useState } from 'react'
import * as Service from '../bindings/crocstream.local/app/internal/desktop/service'
import type { Stats as StreamStats, Started } from '../bindings/crocstream.local/app/internal/session/models'
import { useCanvasPreview } from './canvas-preview'
import { CaptureSettings, useCaptureSettings } from './capture-settings'

const labels: Record<string, string> = {
  Idle: 'Chưa kết nối',
  Preparing: 'Đang chuẩn bị relay',
  WaitingForViewer: 'Đang chờ người xem',
  Connecting: 'Đang kết nối relay',
  Authenticating: 'Đang xác thực peer',
  AwaitingKeyframe: 'Đang chờ KEY đầu tiên',
  Live: 'Đang chia sẻ',
  Watching: 'Đang xem',
  Recovering: 'Đang phục hồi ảnh',
  Stopping: 'Đang đóng phiên',
  Disconnected: 'Đã ngắt kết nối',
  Error: 'Không kết nối được',
}
const inactive = (state: string) => ['Idle', 'Disconnected', 'Error'].includes(state)

const previewMessages = {
  decodeFailed: 'Preview unavailable · Không giải mã được JPEG cục bộ.',
  connectionFailed: 'Preview unavailable · Lỗi WebSocket cục bộ.',
  connectionClosed: 'Preview unavailable · Kênh hiển thị đã đóng; có thể thử nối lại.',
}

export function Stream({ role, onHome }: { role: 'share' | 'watch'; onHome: () => void }) {
  const sharing = role === 'share'
  const settings = useCaptureSettings('share')
  const { refresh } = settings
  const [tau, setTau] = useState(4)
  const [keyInterval, setKeyInterval] = useState(30)
  const [address, setAddress] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [invitation, setInvitation] = useState('')
  const [state, setState] = useState('Idle')
  const [error, setError] = useState('')
  const [started, setStarted] = useState<Started>()
  const [stats, setStats] = useState<StreamStats>()
  const [imageAttempt, setImageAttempt] = useState(0)
  const [copyStatus, setCopyStatus] = useState('')
  const [leaving, setLeaving] = useState(false)
  const [retryAfter, setRetryAfter] = useState(0)
  const reconnectAt = useRef(0)
  const { canvas, hasImage, imageError, setImageError, renderFPS, sampleRenderFPS } =
    useCanvasPreview(started?.endpoint, previewMessages, imageAttempt)

  const epoch = useRef(0)
  const active = useRef(false)
  const busy = useRef(false)
  const pending = useRef<Promise<Started> | undefined>(undefined)

  useEffect(() => {
    let mounted = true
    let timer: ReturnType<typeof setTimeout>
    if (sharing) void refresh(setError, { shouldApply: () => mounted, preferPrimary: true })

    const poll = async () => {
      const current = epoch.current
      const canApply = () => mounted && current === epoch.current && !busy.current
      try {
        const next = await Service.GetSessionStats()
        if (!mounted) return
        if (canApply()) {
          setStats(next)
          setState(next.state)
          setRetryAfter(next.retryAfterMs)
          if (next.error) setError(next.error)
          if (next.previewError) setImageError(next.previewError)

          if (inactive(next.state)) {
            active.current = false
            setStarted(undefined)
            setInvitation('')
          } else if (sharing) {
            const invite = await Service.GetInvitation()
            if (canApply()) setInvitation(invite)
          }
        }
        if (mounted) sampleRenderFPS()
      } catch {
        if (mounted) setError('Không đọc được trạng thái phiên.')
      }
      if (mounted) timer = setTimeout(() => void poll(), 500)
    }

    void poll()
    return () => {
      mounted = false
      clearTimeout(timer)
      epoch.current++
      if (active.current) {
        void (pending.current ?? Promise.resolve())
          .catch(() => {})
          .then(() => Service.StopSession())
          .catch(() => {})
      }
    }
  }, [sharing, refresh, sampleRenderFPS, setImageError])

  const start = async () => {
    if (active.current) return
    active.current = true
    busy.current = true
    const current = ++epoch.current
    setState(sharing ? 'Preparing' : 'Connecting')
    setError('')
    setImageError('')
    setInvitation('')
    setCopyStatus('')
    setStats(undefined)

    try {
      const relay = { address, password }
      const request = sharing
        ? Service.StartBroadcast(settings.options(tau, keyInterval), relay)
        : Service.JoinSession(code, relay)
      pending.current = request
      const result = await request
      setPassword('')
      setCode('')
      if (current === epoch.current) setStarted(result)
    } catch {
      if (current === epoch.current) {
        active.current = false
        setState('Error')
        setError('Không bắt đầu được phiên. Kiểm tra relay, mã và nguồn màn hình; nếu vừa thử kết nối, chờ rồi thử lại.')
      }
    } finally {
      pending.current = undefined
      if (current === epoch.current) busy.current = false
    }
  }

  const stop = async () => {
    epoch.current++
    busy.current = true
    setStarted(undefined)
    setInvitation('')
    setCode('')
    setPassword('')
    setCopyStatus('')
    setImageError('')
    setState('Stopping')

    try {
      await pending.current?.catch(() => {})
      await Service.StopSession()
      active.current = false
      setState('Idle')
      setError('')
      return true
    } catch {
      setError('Đang chờ đóng tài nguyên; phiên vẫn ở trạng thái Stopping.')
      return false
    } finally {
      busy.current = false
    }
  }

  const home = async () => {
    setLeaving(true)
    if (await stop()) onHome()
    setLeaving(false)
  }

  const reconnect = () => {
    if (performance.now() < reconnectAt.current) return
    reconnectAt.current = performance.now() + 2000
    setImageAttempt(attempt => attempt + 1)
  }

  const locked = !inactive(state)
  const sessionStats = stats && started && stats.generation === started.generation && locked
  const validStats = sessionStats && stats.measurementsComplete &&
    (sharing ? stats.counters.emitted > 0 : stats.counters.received > 0)

  return <><nav><button disabled={leaving} onClick={()=>void home()}>← Dừng phiên và về Home</button></nav><main>
    <header><div><span className="eyebrow">CrocStream</span><h1>{sharing?'Chia sẻ màn hình':'Xem một phiên'}</h1></div>
      <span className={`status state-${state}`} role="status">{labels[state]??state}</span></header>
    <div className="workspace">
      <section className="viewer">
        <div className="section-heading"><h2>{sharing?'Màn hình của bạn':'Màn hình người chia sẻ'}</h2><span>{sharing?'Xem trước tại máy này':'Ảnh nhận từ phiên'}</span></div>
        <div className="canvas-wrap"><canvas ref={canvas} width={1280} height={720} aria-label={sharing?'Preview màn hình đang phát':'Ảnh nhận từ người phát'} />
          {!hasImage&&<div className="placeholder"><strong>{started?labels[state]??'Đang chờ ảnh…':sharing?'Chưa chia sẻ màn hình':'Chưa tham gia phiên'}</strong><p>{sharing?'Chọn nguồn và relay ở bên phải.':'Nhập mã mời và relay ở bên phải.'}</p></div>}
          {hasImage&&state==='Recovering'&&<div className="recovery" role="status">Đang phục hồi · giữ ảnh hợp lệ cuối cùng</div>}</div>
        {imageError&&<><p className="error" role="status">{imageError}</p>{started&&<button onClick={reconnect}>Thử lại hình hiển thị</button>}</>}
        <div className="metrics">
          <Metric label="FPS hiển thị" value={started?String(renderFPS):'—'} />
          <Metric label="Tốc độ video (KiB/s)" help="Byte KEY/DELTA, gồm header, trong 1 giây gần nhất; không gồm control, mã hóa hay overhead mạng." value={validStats?(sharing?stats.sentKiB:stats.receivedKiB).toFixed(1):'—'} />
          <Metric label="RTT khứ hồi (ms)" help="Ping/Pong gần nhất; không phải độ trễ một chiều hay end-to-end." value={validStats&&stats.counters.rttKnown?stats.counters.rttMs.toFixed(1):'N/A'} />
        </div>
        <details className="stream-details"><summary>Chi tiết luồng</summary><div className="metrics metrics-detail">
          <Metric label={sharing?'FPS chụp':'FPS nhận'} value={sessionStats&&stats.measurementsComplete?String(sharing?stats.captureFPS:stats.receivedFPS):'—'} />
          <Metric label={sharing?'FPS gửi':'FPS giải mã'} value={validStats?String(sharing?stats.emittedFPS:stats.decodedFPS):'—'} />
          <Metric label="KEY / DELTA" value={validStats?`${stats.counters.keys} / ${stats.counters.deltas}`:'—'} />
          <Metric label="KEY trung bình (KiB)" value={validStats&&stats.keyMeanKnown?stats.meanKeyKiB.toFixed(2):'—'} />
          <Metric label="DELTA trung bình (KiB)" value={validStats&&stats.deltaMeanKnown?stats.meanDeltaKiB.toFixed(2):'—'} />
          <Metric label="Mẫu chụp bỏ qua" value={validStats&&sharing?String(stats.dropped):'—'} />
          <Metric label="Nén toàn ảnh (KiB/s)" help="Nén LZ4 toàn ảnh trên cùng mẫu đã gửi, gồm header, trong cùng cửa sổ 1 giây." value={validStats&&sharing&&stats.baselineKnown?stats.baselineKiB.toFixed(1):'—'} />
          <Metric label="Tiết kiệm so với toàn ảnh" help="100 × (1 − byte video / byte nén toàn ảnh), cùng cửa sổ 1 giây. Có thể âm." value={validStats&&sharing&&stats.savingsKnown?`${stats.savings.toFixed(1)}%`:'—'} />
          <Metric label="Thời lượng phiên (giây)" help="Tính từ lúc chuẩn bị, gồm cả thời gian chờ người xem." value={sessionStats?stats.uptime.toFixed(0):'—'} />
          <Metric label="Khung gần nhất" help="KEY/DELTA cuối được gửi hoặc giải mã; không xác nhận canvas đã hiển thị khung đó." value={validStats?stats.counters.lastKind||'—':'—'} />
          <Metric label="Giải mã đến hiển thị (ms)" help="Chưa đo: ảnh JPEG cục bộ chưa có dấu thời gian liên kết với bước giải mã LZ4." value="N/A" />
        </div>
        <p className="caption">Tốc độ tính trong 1 giây gần nhất; cập nhật mỗi 500 ms. KEY/DELTA và mẫu bỏ là số cộng dồn. Mức tiết kiệm so sánh với nén toàn ảnh cùng mẫu, có thể âm. Byte gồm header, không gồm control hoặc overhead mạng. RTT là khứ hồi; JPEG chỉ dùng để hiển thị.</p></details>
        {stats?.measurementOverflow&&<p role="status" className="error">Cửa sổ đo chưa đầy đủ; tạm ẩn tốc độ và savings.</p>}
      </section>
      <aside><h2>{sharing?'Thiết lập chia sẻ':'Kết nối'}</h2>
        {invitation&&<div className="invitation"><label htmlFor="invitation">Mã mời</label>
          <input id="invitation" type="password" readOnly value={invitation} autoComplete="off" />
          <button onClick={()=>void navigator.clipboard.writeText(invitation).then(()=>setCopyStatus('Đã sao chép mã.')).catch(()=>setError('Không sao chép được mã.'))}>Sao chép mã</button>
          <p role="status" className="caption">{copyStatus}</p>
          <p className="caption">Chỉ gửi cho người xem. Dừng phiên để thu hồi mã.</p></div>}
        <label htmlFor="relay">Địa chỉ relay</label>
        <input id="relay" value={address} disabled={locked} onChange={e=>setAddress(e.target.value)} placeholder="127.0.0.1:9009" autoComplete="off" />
        <label htmlFor="relay-password">Mật khẩu relay</label>
        <input id="relay-password" type="password" value={password} disabled={locked} onChange={e=>setPassword(e.target.value)} autoComplete="off" />
        <p className="field-help">Dùng cùng relay trên cả hai máy. Mật khẩu chỉ giữ trong phiên.</p>
        {sharing?<>
          <CaptureSettings settings={settings} locked={locked}
            onRefresh={() => void refresh(setError)} />
          <details><summary>Nén ảnh nâng cao</summary>
            <label htmlFor="stream-tau">Ngưỡng RGB MAE (tau)</label><input id="stream-tau" type="number" min="0" max="255" value={tau} disabled={locked} onChange={e=>setTau(Math.max(0,Math.min(255,Number(e.target.value))))} />
            <label htmlFor="stream-keyinterval">Khoảng KEY (1–120 frame)</label><input id="stream-keyinterval" type="number" min="1" max="120" value={keyInterval} disabled={locked} onChange={e=>setKeyInterval(Math.max(1,Math.min(120,Number(e.target.value))))} />
          </details>
           <p className="notice">Bắt đầu sẽ chụp màn hình đã chọn. Người có mã mời có thể tham gia xem.</p>
        </>:<><label htmlFor="join-code">Mã phiên</label><input id="join-code" type="password" value={code} disabled={locked} onChange={e=>setCode(e.target.value)} autoComplete="off" /></>}
        <button className="primary" disabled={locked||leaving||!address||!password||(sharing?!settings.display:!code||retryAfter>0)} onClick={()=>void start()}>{sharing?'Bắt đầu chia sẻ':'Tham gia phiên'}</button>
        {!sharing&&retryAfter>0&&!locked&&<p role="status" className="caption">Thử lại sau {Math.ceil(retryAfter/1000)} giây. Không tự động kết nối lại.</p>}
        <button className="stop" disabled={!locked} onClick={()=>void stop()}>{sharing?'Dừng chia sẻ':'Rời phiên'}</button>
        {error&&<p className="error" role="alert">{error}</p>}
      </aside>
    </div>
  </main></>
}
function Metric({label,value,help='Số đo thật trong cửa sổ 1 giây gần nhất; — khi chưa có mẫu.'}:{label:string;value:string;help?:string}) {return <div tabIndex={0} title={help} aria-label={`${label}: ${value}. ${help}`}><span>{label}</span><strong>{value}</strong></div>}
