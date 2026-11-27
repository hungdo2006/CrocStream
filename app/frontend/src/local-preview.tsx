import type { State } from './api'
import { CaptureSettings, useCaptureSettings } from './capture-settings'
import { useFrameStream } from './hooks/useFrameStream'
import { useLeaveSession, usePreviewSession } from './hooks/useSession'

const names: Record<State, string> = {
  Idle: 'Chưa chụp màn hình',
  Preparing: 'Đang chuẩn bị',
  Previewing: 'Đang xem trước cục bộ',
  Stopping: 'Đang dừng và giải phóng tài nguyên',
  Error: 'Preview gặp lỗi',
}

export function LocalPreview({ onHome }: { onHome: () => void }) {
  const controller = usePreviewSession()
  const { state, error, started: session, stats } = controller
  const locked = state === 'Preparing' || state === 'Previewing' || state === 'Stopping'
  const settings = useCaptureSettings(true, controller.setError, 'Không đọc được danh sách nguồn chụp. Hãy thử làm mới.', !locked)
  const { canvas, error: previewError, hasImage, renderFPS } = useFrameStream(session, 'preview')
  const start = () => controller.start(settings.options())
  const stop = controller.stop
  const { leaving, home } = useLeaveSession(stop, onHome)
  const measured = stats && session && stats.generation === session.generation && stats.captured > 0

  return (
    <div className="app-shell">
      <div className="topbar">
        <div className="topbar-left">
          <button type="button" className="back-btn" disabled={leaving} onClick={() => void home()}>
            ← Dừng phiên và về Home
          </button>
          <span className="topbar-divider" aria-hidden="true" />
          <h1>Kiểm tra nguồn chụp</h1>
        </div>
        <span className={`status ${locked ? 'active' : ''}`} role="status">
          {names[state as State] ?? state}
        </span>
      </div>

      <main className="workspace-main">
        <div className="workspace">
          <section className="viewer" aria-labelledby="preview-title">
            <div className="stage-bar">
              <div className="stage-title">
                <h2 id="preview-title">{settings.selectedSource?.name || 'Màn hình của bạn'}</h2>
                <span className="stage-sub">Chỉ chạy cục bộ · Không gửi qua mạng</span>
              </div>
            </div>
            <div className="canvas-wrap">
              <canvas ref={canvas} width={1280} height={720} aria-label="Ảnh xem trước nguồn đã chọn" />
              {!hasImage && (
                <div className="placeholder">
                  <strong>{session ? 'Đang tải hình xem trước…' : 'Chưa có hình xem trước'}</strong>
                  <p>Chọn màn hình hoặc cửa sổ bên phải rồi bấm Bắt đầu preview.</p>
                </div>
              )}
            </div>
            {previewError && (
              <p role="status" className="error">
                {previewError}
              </p>
            )}
            <div className="metrics metrics-4" aria-label="Số đo preview">
              <Metric label="Capture FPS" value={measured ? String(stats.captureFPS) : '—'} />
              <Metric label="JPEG FPS" value={measured ? String(stats.encodeFPS) : '—'} />
              <Metric label="Render FPS" value={measured ? String(renderFPS) : '—'} />
              <Metric label="Mẫu bỏ trước JPEG" value={measured ? String(stats.dropped) : '—'} />
            </div>
            <p className="caption">FPS đo trong 1 giây gần nhất. Ảnh chỉ giữ trong RAM, không ghi đĩa.</p>
          </section>

          <aside className="inspector">
            <div className="panel-section">
              <CaptureSettings settings={settings} mode="local" locked={locked} />
            </div>
            <div className="panel-actions">
              <button
                type="button"
                className="primary"
                disabled={locked || leaving || !settings.display}
                onClick={() => { void start() }}
              >
                Bắt đầu preview
              </button>
              <button
                type="button"
                className="stop"
                disabled={!locked}
                onClick={() => { void stop() }}
              >
                Dừng preview
              </button>
              {error && (
                <p role="alert" className="error">
                  {error}
                </p>
              )}
            </div>
          </aside>
        </div>
      </main>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}
