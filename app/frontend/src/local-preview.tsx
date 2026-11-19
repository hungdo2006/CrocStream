import type { State } from './api'
import { CaptureSettings, useCaptureSettings } from './capture-settings'
import { useFrameStream } from './hooks/useFrameStream'
import { useLeaveSession, usePreviewSession } from './hooks/useSession'

const names: Record<State, string> = {
  Idle: 'Chưa chụp màn hình', Preparing: 'Đang chuẩn bị', Previewing: 'Đang xem trước cục bộ',
  Stopping: 'Đang dừng và giải phóng tài nguyên', Error: 'Preview gặp lỗi',
}

export function LocalPreview({ onHome }: { onHome: () => void }) {
  const controller = usePreviewSession()
  const { state, error, started: session, stats } = controller
  const settings = useCaptureSettings(true, controller.setError, 'Không đọc được danh sách màn hình. Hãy thử làm mới.')
  const { canvas, error: previewError, renderFPS } = useFrameStream(session, 'preview')
  const start = () => controller.start(settings.options())
  const stop = controller.stop
  const { leaving, home } = useLeaveSession(stop, onHome)
  const locked = state === 'Preparing' || state === 'Previewing' || state === 'Stopping'
  const measured = stats && session && stats.generation === session.generation && stats.captured > 0
  return <><nav><button disabled={leaving} onClick={()=>void home()}>← Dừng phiên và về Home</button></nav><main>
    <header><div><span className="eyebrow">CrocStream</span><h1>Xem trước</h1></div>
      <span className={`status ${locked ? 'active' : ''}`} role="status">{names[state as State]??state}</span></header>
    <div className="workspace">
      <section className="viewer" aria-labelledby="preview-title">
        <div className="section-heading"><h2 id="preview-title">Màn hình của bạn</h2><span>Chỉ trên máy này</span></div>
        <div className="canvas-wrap"><canvas ref={canvas} width={1280} height={720} aria-label="Ảnh xem trước màn hình đã chọn" />
          {!session && <div className="placeholder"><strong>Chưa có hình xem trước</strong><p>Chọn màn hình bên phải để bắt đầu.</p></div>}
        </div>
        <p className="caption">Xem thử nguồn chụp. Không gửi ảnh tới máy khác.</p>
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
        <CaptureSettings settings={settings} mode="local" locked={locked} />
        <p className="notice">Bắt đầu sẽ chụp màn hình đã chọn. Ảnh chỉ ở RAM; không lưu hoặc gửi ra Internet.</p>
        <button className="primary" disabled={locked || leaving || !settings.display} onClick={() => { void start() }}>Bắt đầu preview</button>
        <button className="stop" disabled={!locked} onClick={() => { void stop() }}>Dừng preview</button>
        {error && <p role="alert" className="error">{error}</p>}
      </aside>
    </div>
  </main></>
}
function Metric({ label, value }: { label: string; value: string }) {
  return <div><span>{label}</span><strong>{value}</strong></div>
}
