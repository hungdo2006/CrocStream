import React, { useState } from 'react'
import { createRoot } from 'react-dom/client'
import type { State } from './api'
import { usePreviewSession, useLeaveSession } from './hooks/useSession'
import { useDisplays } from './hooks/useDisplays'
import { useFrameStream } from './hooks/useFrameStream'
import { Stream } from './stream'
import './style.css'

const names: Record<State, string> = {
  Idle: 'Chưa chụp màn hình', Preparing: 'Đang chuẩn bị', Previewing: 'Đang xem trước cục bộ',
  Stopping: 'Đang dừng và giải phóng tài nguyên', Error: 'Preview gặp lỗi',
}

function App({onHome}:{onHome:()=>void}) {
  const controller = usePreviewSession()
  const { state, error, started: session, stats } = controller
  const { displays, display, setDisplay, refresh } = useDisplays(true, controller.setError, 'Không đọc được danh sách màn hình. Hãy thử làm mới.')
  const { canvas, error: previewError, renderFPS } = useFrameStream(session, 'preview')
  const [resolution, setResolution] = useState('1280x720')
  const [fps, setFPS] = useState(24)
  const start = async () => {
    const [width, height] = resolution.split('x').map(Number)
    await controller.start({ displayID: display, width, height, fps, tau:4, keyInterval:30 })
  }
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
        <button className="primary" disabled={locked || leaving || !display} onClick={() => { void start() }}>Bắt đầu preview</button>
        <button className="stop" disabled={!locked} onClick={() => { void stop() }}>Dừng preview</button>
        {error && <p role="alert" className="error">{error}</p>}
      </aside>
    </div>
  </main></>
}
function Metric({ label, value }: { label: string; value: string }) {
  return <div><span>{label}</span><strong>{value}</strong></div>
}
function Root() {
  const [mode,setMode]=useState<'home'|'local'|'share'|'watch'>('home')
  const home=()=>setMode('home')
  if(mode==='home')return <main className="home"><header><div><h1>CrocStream</h1><p className="subtitle">Chia sẻ màn hình giữa hai máy Windows.</p></div><span className="version">Bản thử nghiệm 0.0.1</span></header>
    <section className="home-actions" aria-label="Bắt đầu phiên">
      <div className="home-action"><h2>Chia sẻ màn hình</h2><p>Chọn màn hình, tạo phiên và gửi mã mời cho người xem.</p><button data-testid="home-share" className="primary" onClick={()=>setMode('share')}>Chia sẻ màn hình</button></div>
      <div className="home-action"><h2>Xem một phiên</h2><p>Nhập mã mời từ người chia sẻ để xem trên máy này.</p><button data-testid="home-watch" onClick={()=>setMode('watch')}>Tham gia bằng mã</button></div>
    </section>
    <section className="home-setup"><h2>Trước khi kết nối</h2><p>Hai máy cần cùng địa chỉ và mật khẩu relay. Mã mời sẽ xuất hiện sau khi tạo phiên.</p><p>Chưa có relay? Xem <strong>DEMO.md</strong> trong thư mục ứng dụng.</p></section>
    <div className="home-preview"><div><h2>Kiểm tra màn hình</h2><p>Xem thử nguồn chụp, không cần relay.</p></div><button data-testid="home-local" onClick={()=>setMode('local')}>Xem trước trên máy này</button></div>
    <footer>1 người chia sẻ · 1 người xem<span>Không ghi hình hoặc âm thanh</span></footer></main>
  return mode==='local'?<App onHome={home}/>:<Stream key={mode} role={mode} onHome={home}/>
}
createRoot(document.getElementById('root')!).render(<React.StrictMode><Root /></React.StrictMode>)
