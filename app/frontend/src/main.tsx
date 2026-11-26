import React, { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { LocalPreview } from './local-preview'
import { Stream } from './stream'
import './style.css'

type ScreenMode = 'home' | 'local' | 'share' | 'watch'

function Root() {
  const [mode, setMode] = useState<ScreenMode>('home')
  const goHome = () => setMode('home')

  if (mode === 'home') {
    return (
      <div className="home-shell">
        <main className="home">
          <header className="home-header">
            <div className="brand">
              <span className="brand-mark" aria-hidden="true">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="2" y="3" width="20" height="14" rx="2" />
                  <path d="M8 21h8" />
                  <path d="M12 17v4" />
                </svg>
              </span>
              <div>
                <h1>CrocStream</h1>
                <p className="subtitle">Truyền màn hình 1-1 mã hóa đầu cuối trên Windows.</p>
              </div>
            </div>
            <span className="version">v0.0.1</span>
          </header>

          <section className="home-actions" aria-label="Bắt đầu phiên">
            <div className="home-action">
              <div className="action-icon" aria-hidden="true">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M12 17V7" />
                  <path d="m7 12 5-5 5 5" />
                  <rect x="3" y="3" width="18" height="18" rx="3" />
                </svg>
              </div>
              <h2>Chia sẻ màn hình</h2>
              <p>Chọn toàn màn hình hoặc cửa sổ ứng dụng (Chrome, Zalo…), nhận mã mời 1 lần để gửi cho người xem.</p>
              <button type="button" data-testid="home-share" className="primary" onClick={() => setMode('share')}>
                Chia sẻ màn hình
              </button>
            </div>
            <div className="home-action">
              <div className="action-icon" aria-hidden="true">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                  <polygon points="6 4 20 12 6 20 6 4" />
                </svg>
              </div>
              <h2>Xem một phiên</h2>
              <p>Nhập mã mời từ người chia sẻ để xem trực tiếp. Tự ưu tiên LAN khi cùng mạng, tự chuyển Relay khi khác mạng.</p>
              <button type="button" data-testid="home-watch" className="btn-secondary" onClick={() => setMode('watch')}>
                Tham gia bằng mã
              </button>
            </div>
          </section>

          <section className="home-utility">
            <div>
              <h2>Kiểm tra nguồn chụp</h2>
              <p>Xem thử màn hình hoặc cửa sổ trên máy này mà không cần mở kết nối mạng.</p>
            </div>
            <button type="button" data-testid="home-local" className="btn-ghost" onClick={() => setMode('local')}>
              Xem trước trên máy này
            </button>
          </section>

          <footer className="home-footer">
            <span>1 người chia sẻ · 1 người xem</span>
            <span>PAKE + AES-GCM đầu cuối</span>
            <span>Không lưu hình ảnh hay âm thanh</span>
          </footer>
        </main>
      </div>
    )
  }

  return mode === 'local' ? (
    <LocalPreview onHome={goHome} />
  ) : (
    <Stream key={mode} role={mode} onHome={goHome} />
  )
}

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>,
)
