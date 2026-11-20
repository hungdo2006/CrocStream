import { useState } from 'react'
import { useStreamSession, useLeaveSession } from './hooks/useSession'
import { CaptureSettings, useCaptureSettings } from './capture-settings'
import { useFrameStream } from './hooks/useFrameStream'
import { inactive } from './session-lifecycle'
import { StreamMetrics } from './stream-metrics'

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

export function Stream({ role, onHome }: { role: 'share' | 'watch'; onHome: () => void }) {
  const sharing = role === 'share'
  const session = useStreamSession(role)
  const { state, error, started, stats, invitation, copyStatus } = session
  const settings = useCaptureSettings(sharing, session.setError, 'Không đọc được danh sách màn hình.')
  const { canvas, error: frameError, hasImage, renderFPS, reconnect } = useFrameStream(started, 'stream')
  const imageError = frameError || (started ? stats?.previewError : '')

  const [tau, setTau] = useState(4)
  const [keyInterval, setKeyInterval] = useState(30)
  const [address, setAddress] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')

  const start = async () => {
    if (await session.start(settings.options(tau, keyInterval), { address, password }, code)) {
      setPassword('')
      setCode('')
    }
  }

  const stop = () => {
    setCode('')
    setPassword('')
    return session.stop()
  }

  const { leaving, home } = useLeaveSession(stop, onHome)
  const retryAfter = stats?.retryAfterMs ?? 0
  const locked = !inactive(state)
  const canStart =
    !locked &&
    !leaving &&
    Boolean(address) &&
    Boolean(password) &&
    (sharing ? Boolean(settings.display) : Boolean(code) && retryAfter <= 0)

  return (
    <>
      <nav>
        <button disabled={leaving} onClick={() => void home()}>
          ← Dừng phiên và về Home
        </button>
      </nav>
      <main>
        <header>
          <div>
            <span className="eyebrow">CrocStream</span>
            <h1>{sharing ? 'Chia sẻ màn hình' : 'Xem một phiên'}</h1>
          </div>
          <span className={`status state-${state}`} role="status">
            {labels[state] ?? state}
          </span>
        </header>

        <div className="workspace">
          <section className="viewer">
            <div className="section-heading">
              <h2>{sharing ? 'Màn hình của bạn' : 'Màn hình người chia sẻ'}</h2>
              <span>{sharing ? 'Xem trước tại máy này' : 'Ảnh nhận từ phiên'}</span>
            </div>
            <div className="canvas-wrap">
              <canvas
                ref={canvas}
                width={1280}
                height={720}
                aria-label={sharing ? 'Preview màn hình đang phát' : 'Ảnh nhận từ người phát'}
              />
              {!hasImage && (
                <div className="placeholder">
                  <strong>
                    {started
                      ? (labels[state] ?? 'Đang chờ ảnh…')
                      : sharing
                        ? 'Chưa chia sẻ màn hình'
                        : 'Chưa tham gia phiên'}
                  </strong>
                  <p>{sharing ? 'Chọn nguồn và relay ở bên phải.' : 'Nhập mã mời và relay ở bên phải.'}</p>
                </div>
              )}
              {hasImage && state === 'Recovering' && (
                <div className="recovery" role="status">
                  Đang phục hồi · giữ ảnh hợp lệ cuối cùng
                </div>
              )}
            </div>
            {imageError && (
              <>
                <p className="error" role="status">{imageError}</p>
                {started && <button onClick={reconnect}>Thử lại hình hiển thị</button>}
              </>
            )}
            <StreamMetrics
              role={role}
              generation={started?.generation}
              sessionActive={locked}
              stats={stats}
              renderFPS={renderFPS}
            />
          </section>

          <aside>
            <h2>{sharing ? 'Thiết lập chia sẻ' : 'Kết nối'}</h2>
            {invitation && (
              <div className="invitation">
                <label htmlFor="invitation">Mã mời</label>
                <input id="invitation" type="password" readOnly value={invitation} autoComplete="off" />
                <button onClick={() => void session.copyInvitation()}>Sao chép mã</button>
                <p role="status" className="caption">{copyStatus}</p>
                <p className="caption">Chỉ gửi cho người xem. Dừng phiên để thu hồi mã.</p>
              </div>
            )}

            <label htmlFor="relay">Địa chỉ relay</label>
            <input
              id="relay"
              value={address}
              disabled={locked}
              onChange={e => setAddress(e.target.value)}
              placeholder="127.0.0.1:9009"
              autoComplete="off"
            />

            <label htmlFor="relay-password">Mật khẩu relay</label>
            <input
              id="relay-password"
              type="password"
              value={password}
              disabled={locked}
              onChange={e => setPassword(e.target.value)}
              autoComplete="off"
            />
            <p className="field-help">Dùng cùng relay trên cả hai máy. Mật khẩu chỉ giữ trong phiên.</p>

            {sharing ? (
              <>
                <CaptureSettings settings={settings} mode="share" locked={locked} />
                <details>
                  <summary>Nén ảnh nâng cao</summary>
                  <label htmlFor="stream-tau">Ngưỡng RGB MAE (tau)</label>
                  <input
                    id="stream-tau"
                    type="number"
                    min="0"
                    max="255"
                    value={tau}
                    disabled={locked}
                    onChange={e => setTau(Math.max(0, Math.min(255, Number(e.target.value))))}
                  />
                  <label htmlFor="stream-keyinterval">Khoảng KEY (1–120 frame)</label>
                  <input
                    id="stream-keyinterval"
                    type="number"
                    min="1"
                    max="120"
                    value={keyInterval}
                    disabled={locked}
                    onChange={e => setKeyInterval(Math.max(1, Math.min(120, Number(e.target.value))))}
                  />
                </details>
                <p className="notice">Bắt đầu sẽ chụp màn hình đã chọn. Người có mã mời có thể tham gia xem.</p>
              </>
            ) : (
              <>
                <label htmlFor="join-code">Mã phiên</label>
                <input
                  id="join-code"
                  type="password"
                  value={code}
                  disabled={locked}
                  onChange={e => setCode(e.target.value)}
                  autoComplete="off"
                />
              </>
            )}

            <button className="primary" disabled={!canStart} onClick={() => void start()}>
              {sharing ? 'Bắt đầu chia sẻ' : 'Tham gia phiên'}
            </button>
            {!sharing && retryAfter > 0 && !locked && (
              <p role="status" className="caption">
                Thử lại sau {Math.ceil(retryAfter / 1000)} giây. Không tự động kết nối lại.
              </p>
            )}
            <button className="stop" disabled={!locked} onClick={() => void stop()}>
              {sharing ? 'Dừng chia sẻ' : 'Rời phiên'}
            </button>
            {error && (
              <p className="error" role="alert">
                {error}
              </p>
            )}
          </aside>
        </div>
      </main>
    </>
  )
}
