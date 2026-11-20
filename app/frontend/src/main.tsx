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
      <main className="home">
        <header>
          <div>
            <h1>CrocStream</h1>
            <p className="subtitle">Chia sẻ màn hình giữa hai máy Windows.</p>
          </div>
          <span className="version">Bản thử nghiệm 0.0.1</span>
        </header>

        <section className="home-actions" aria-label="Bắt đầu phiên">
          <div className="home-action">
            <h2>Chia sẻ màn hình</h2>
            <p>Chọn màn hình, tạo phiên và gửi mã mời cho người xem.</p>
            <button data-testid="home-share" className="primary" onClick={() => setMode('share')}>
              Chia sẻ màn hình
            </button>
          </div>
          <div className="home-action">
            <h2>Xem một phiên</h2>
            <p>Nhập mã mời từ người chia sẻ để xem trên máy này.</p>
            <button data-testid="home-watch" onClick={() => setMode('watch')}>
              Tham gia bằng mã
            </button>
          </div>
        </section>

        <section className="home-setup">
          <h2>Trước khi kết nối</h2>
          <p>Hai máy cần cùng địa chỉ và mật khẩu relay. Mã mời sẽ xuất hiện sau khi tạo phiên.</p>
          <p>Chưa có relay? Xem <strong>DEMO.md</strong> trong thư mục ứng dụng.</p>
        </section>

        <div className="home-preview">
          <div>
            <h2>Kiểm tra màn hình</h2>
            <p>Xem thử nguồn chụp, không cần relay.</p>
          </div>
          <button data-testid="home-local" onClick={() => setMode('local')}>
            Xem trước trên máy này
          </button>
        </div>

        <footer>
          1 người chia sẻ · 1 người xem
          <span>Không ghi hình hoặc âm thanh</span>
        </footer>
      </main>
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
