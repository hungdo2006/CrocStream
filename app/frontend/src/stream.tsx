import { useEffect, useRef, useState } from 'react'
import { streamAPI, type Relay } from './api'
import { useStreamSession, useLeaveSession } from './hooks/useSession'
import { CaptureSettings, useCaptureSettings } from './capture-settings'
import { useFrameStream } from './hooks/useFrameStream'
import { useSourcePreview } from './hooks/useSourcePreview'
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
  const locked = !inactive(state)
  const settings = useCaptureSettings(sharing, session.setError, 'Không đọc được danh sách nguồn chụp.', sharing && !locked)

  const [tau, setTau] = useState(4)
  const [keyInterval, setKeyInterval] = useState(120)
  const [defaultRelay, setDefaultRelay] = useState<Relay>({ address: '', password: '' })
  const [address, setAddress] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [previewEnabled, setPreviewEnabled] = useState(true)
  const [showInvite, setShowInvite] = useState(false)
  const relayEdited = useRef(false)
  const leavingRef = useRef(false)

  useEffect(() => {
    let cancelled = false
    void streamAPI.defaultRelay().then(
      loaded => {
        if (cancelled || !loaded?.address || !loaded?.password) return
        setDefaultRelay(loaded)
        if (!relayEdited.current) {
          setAddress(loaded.address)
          setPassword(loaded.password)
        }
      },
      () => undefined,
    )
    return () => { cancelled = true }
  }, [])

  const { width: capW, height: capH } = settings.options(tau, keyInterval)
  const sourcePreview = useSourcePreview(
    sharing && previewEnabled && !locked,
    settings.display,
    capW,
    capH,
    settings.fps,
  )

  const activeEndpoint = started ?? sourcePreview.started
  const isPreSharePreview = sharing && !started && Boolean(sourcePreview.started)
  const { canvas, error: frameError, hasImage, renderFPS, reconnect } = useFrameStream(
    activeEndpoint,
    started ? 'stream' : 'preview',
  )
  const imageError = started ? (frameError || stats?.previewError) : ''

  const hasDefaultRelay = Boolean(defaultRelay.address && defaultRelay.password)
  const usingDefaultRelay =
    hasDefaultRelay && address === defaultRelay.address && password === defaultRelay.password

  const start = async () => {
    if (sharing) await sourcePreview.suspendAndStop()
    if (leavingRef.current) return
    const ok = await session.start(settings.options(tau, keyInterval), { address, password }, code)
    if (ok) {
      if (!usingDefaultRelay) setPassword('')
      setCode('')
    } else if (sharing && !leavingRef.current) {
      sourcePreview.resume()
    }
  }

  const stop = async () => {
    setCode('')
    setPassword(usingDefaultRelay ? defaultRelay.password : '')
    if (sharing) await sourcePreview.suspendAndStop()
    const ok = await session.stop()
    if (sharing && !leavingRef.current) sourcePreview.resume()
    return ok
  }

  const { leaving, home } = useLeaveSession(async () => {
    leavingRef.current = true
    if (sharing) await sourcePreview.suspendAndStop()
    return session.stop()
  }, onHome)

  const retryAfter = stats?.retryAfterMs ?? 0
  const canStart =
    !locked &&
    !leaving &&
    Boolean(address) &&
    Boolean(password) &&
    (sharing ? Boolean(settings.display) : Boolean(code) && retryAfter <= 0)

  const selectedLabel = settings.selectedSource
    ? (settings.selectedSource.name || 'Màn hình đã chọn')
    : ''

  return (
    <div className="app-shell">
      <div className="topbar">
        <div className="topbar-left">
          <button type="button" className="back-btn" disabled={leaving} onClick={() => void home()}>
            ← Dừng phiên và về Home
          </button>
          <span className="topbar-divider" aria-hidden="true" />
          <h1>{sharing ? 'Chia sẻ màn hình' : 'Xem một phiên'}</h1>
        </div>
        <span className={`status state-${state}`} role="status">
          {labels[state] ?? state}
        </span>
      </div>

      <main className="workspace-main">
        <div className="workspace">
          <section className="viewer">
            <div className="stage-bar">
              <div className="stage-title">
                <h2>{sharing ? (selectedLabel || 'Màn hình của bạn') : 'Màn hình người chia sẻ'}</h2>
                <span className="stage-sub">
                  {sharing
                    ? (started ? 'Đang phát trực tiếp' : isPreSharePreview ? 'Xem trước nguồn đã chọn' : 'Xem trước tại máy này')
                    : 'Ảnh nhận từ phiên'}
                </span>
              </div>
              {sharing && !locked && (
                <button
                  type="button"
                  className="text-btn"
                  onClick={() => setPreviewEnabled(v => !v)}
                >
                  {previewEnabled ? 'Tạm dừng xem trước' : 'Bật xem trước'}
                </button>
              )}
            </div>

            <div className="canvas-wrap">
              <canvas
                ref={canvas}
                width={1280}
                height={720}
                aria-label={sharing ? 'Preview màn hình đang phát' : 'Ảnh nhận từ người phát'}
              />
              {isPreSharePreview && hasImage && (
                <div className="stage-badge" role="status">
                  Xem trước · Chưa phát
                </div>
              )}
              {!hasImage && (
                <div className="placeholder">
                  <strong>
                    {started
                      ? (labels[state] ?? 'Đang chờ ảnh…')
                      : sourcePreview.busy || isPreSharePreview
                        ? 'Đang tải xem trước nguồn…'
                        : sharing
                          ? 'Chưa chia sẻ màn hình'
                          : 'Chưa tham gia phiên'}
                  </strong>
                  <p>
                    {sharing
                      ? (previewEnabled ? 'Chọn màn hình hoặc cửa sổ ở cột bên phải.' : 'Bật xem trước hoặc bấm Bắt đầu chia sẻ.')
                      : 'Nhập mã phiên ở cột bên phải để kết nối.'}
                  </p>
                </div>
              )}
              {hasImage && state === 'Recovering' && (
                <div className="recovery" role="status">
                  Đang phục hồi · giữ ảnh hợp lệ cuối cùng
                </div>
              )}
            </div>

            {imageError && (
              <div className="inline-alert">
                <p className="error" role="status">{imageError}</p>
                {started && (
                  <button type="button" className="compact-btn" onClick={reconnect}>
                    Thử lại hình hiển thị
                  </button>
                )}
              </div>
            )}

            <StreamMetrics
              role={role}
              generation={started?.generation}
              sessionActive={locked}
              stats={stats}
              renderFPS={started ? renderFPS : 0}
            />
          </section>

          <aside className="inspector">
            {invitation && (
              <div className="panel-section invitation">
                <div className="field-header">
                  <label htmlFor="invitation" className="field-title">Mã mời phiên</label>
                  <button type="button" className="text-btn" onClick={() => setShowInvite(v => !v)}>
                    {showInvite ? 'Ẩn mã' : 'Hiện mã'}
                  </button>
                </div>
                <input
                  id="invitation"
                  type={showInvite ? 'text' : 'password'}
                  readOnly
                  value={invitation}
                  autoComplete="off"
                />
                <button type="button" className="btn-secondary" onClick={() => void session.copyInvitation()}>
                  Sao chép mã
                </button>
                {copyStatus && <p role="status" className="caption success-text">{copyStatus}</p>}
                <p className="caption">Gửi mã này cho người xem. Dừng phiên để thu hồi.</p>
              </div>
            )}

            {sharing ? (
              <div className="panel-section">
                <CaptureSettings settings={settings} mode="share" locked={locked} />
              </div>
            ) : (
              <div className="panel-section">
                <label htmlFor="join-code" className="field-title">Mã phiên</label>
                <input
                  id="join-code"
                  type="password"
                  value={code}
                  disabled={locked}
                  onChange={e => setCode(e.target.value)}
                  placeholder="cs1-..."
                  autoComplete="off"
                />
              </div>
            )}

            <div className="panel-section">
              <div className="field-header">
                <span className="field-title">Kết nối Relay</span>
                {usingDefaultRelay && <span className="tag-pill ok">relay.json</span>}
              </div>
              <label htmlFor="relay">Địa chỉ relay</label>
              <input
                id="relay"
                value={address}
                disabled={locked}
                onChange={e => {
                  relayEdited.current = true
                  setAddress(e.target.value)
                }}
                placeholder="127.0.0.1:9009"
                autoComplete="off"
              />

              <label htmlFor="relay-password">Mật khẩu relay</label>
              <input
                id="relay-password"
                type="password"
                value={password}
                disabled={locked}
                onChange={e => {
                  relayEdited.current = true
                  setPassword(e.target.value)
                }}
                autoComplete="off"
              />
              <p className="field-help">
                {usingDefaultRelay
                  ? 'Đã tự điền từ relay.json cạnh ứng dụng. Cùng Wi-Fi sẽ tự ưu tiên LAN trực tiếp.'
                  : 'Dùng cùng relay trên cả hai máy. Mật khẩu chỉ giữ trong phiên.'}
              </p>
            </div>

            {sharing && (
              <details className="panel-details">
                <summary>Nén ảnh nâng cao</summary>
                <div className="inline-fields">
                  <div>
                    <label htmlFor="stream-tau">Ngưỡng MAE (tau)</label>
                    <input
                      id="stream-tau"
                      type="number"
                      min="0"
                      max="255"
                      value={tau}
                      disabled={locked}
                      onChange={e => setTau(Math.max(0, Math.min(255, Number(e.target.value))))}
                    />
                  </div>
                  <div>
                    <label htmlFor="stream-keyinterval">Khoảng KEY</label>
                    <input
                      id="stream-keyinterval"
                      type="number"
                      min="1"
                      max="120"
                      value={keyInterval}
                      disabled={locked}
                      onChange={e => setKeyInterval(Math.max(1, Math.min(120, Number(e.target.value))))}
                    />
                  </div>
                </div>
              </details>
            )}

            <div className="panel-actions">
              <button type="button" className="primary" disabled={!canStart} onClick={() => void start()}>
                {sharing ? 'Bắt đầu chia sẻ' : 'Tham gia phiên'}
              </button>
              {!sharing && retryAfter > 0 && !locked && (
                <p role="status" className="caption">
                  Thử lại sau {Math.ceil(retryAfter / 1000)} giây. Không tự động kết nối lại.
                </p>
              )}
              <button type="button" className="stop" disabled={!locked} onClick={() => void stop()}>
                {sharing ? 'Dừng chia sẻ' : 'Rời phiên'}
              </button>
              {error && (
                <p className="error" role="alert">
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
