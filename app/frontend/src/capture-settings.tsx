import { useState } from 'react'
import type { Display, Options } from './api'
import { useDisplays } from './hooks/useDisplays'

const resolutions = {
  '1280x720': { width: 1280, height: 720 },
  '640x360': { width: 640, height: 360 },
}
type Resolution = keyof typeof resolutions
type SourceTab = 'display' | 'window'

export function useCaptureSettings(enabled: boolean, setError: (message: string) => void, errorMessage: string) {
  const source = useDisplays(enabled, setError, errorMessage)
  const [resolution, setResolution] = useState<Resolution>('1280x720')
  const [fps, setFPS] = useState(24)

  function options(tau = 4, keyInterval = 30): Options {
    return { displayID: source.display, ...resolutions[resolution], fps, tau, keyInterval }
  }

  const selectedSource = source.displays.find(item => item.id === source.display)

  return { ...source, selectedSource, resolution, setResolution, fps, setFPS, options }
}

function sourceTitle(item: Display, index: number) {
  if (item.kind === 'window') {
    return item.name || 'Cửa sổ ứng dụng'
  }
  return item.name || `Màn hình ${index + 1}`
}

export function CaptureSettings({ settings, mode, locked }: {
  settings: ReturnType<typeof useCaptureSettings>
  mode: 'local' | 'share'
  locked: boolean
}) {
  const local = mode === 'local'
  const prefix = local ? '' : 'stream-'
  const { displays, display, setDisplay, refresh, resolution, setResolution, fps, setFPS } = settings

  const monitors = displays.filter(item => item.kind !== 'window')
  const windows = displays.filter(item => item.kind === 'window')
  const [tab, setTab] = useState<SourceTab>('display')
  const items = tab === 'window' ? windows : monitors

  return (
    <div className="capture-settings">
      <div className="field-header">
        <span className="field-title">Nguồn phát</span>
        <button type="button" className="text-btn" disabled={locked} onClick={() => void refresh()}>
          Làm mới
        </button>
      </div>

      <div className="segmented" role="tablist" aria-label="Loại nguồn chụp">
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'display'}
          className={tab === 'display' ? 'seg-btn active' : 'seg-btn'}
          disabled={locked}
          onClick={() => setTab('display')}
        >
          Màn hình <span className="count">{monitors.length}</span>
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'window'}
          className={tab === 'window' ? 'seg-btn active' : 'seg-btn'}
          disabled={locked}
          onClick={() => setTab('window')}
        >
          Cửa sổ <span className="count">{windows.length}</span>
        </button>
      </div>

      <div
        id={prefix + 'source-list'}
        className="source-list"
        role="listbox"
        aria-label={tab === 'window' ? 'Danh sách cửa sổ ứng dụng' : 'Danh sách màn hình'}
      >
        {items.length === 0 ? (
          <p className="source-empty">
            {tab === 'window'
              ? 'Không thấy cửa sổ đang mở. Mở ứng dụng (không thu nhỏ) rồi bấm Làm mới.'
              : 'Chưa tìm thấy màn hình.'}
          </p>
        ) : (
          items.map((item, index) => {
            const active = item.id === display
            const label = sourceTitle(item, index)
            return (
              <button
                key={item.id}
                type="button"
                role="option"
                aria-selected={active}
                data-source-id={item.id}
                className={active ? 'source-item selected' : 'source-item'}
                disabled={locked}
                onClick={() => setDisplay(item.id)}
                title={label}
              >
                <span className="source-name">
                  {label}
                  {item.kind !== 'window' && item.primary && <span className="tag-pill">Chính</span>}
                </span>
                <span className="source-dims">{item.width}×{item.height}</span>
              </button>
            )
          })
        )}
      </div>

      <div className="inline-fields">
        <div>
          <label htmlFor={prefix + 'resolution'}>Độ phân giải</label>
          <select id={prefix + 'resolution'} value={resolution} disabled={locked} onChange={e => setResolution(e.target.value as Resolution)}>
            {Object.entries(resolutions).map(([value, { width, height }]) =>
              <option key={value} value={value}>{width} × {height}</option>,
            )}
          </select>
        </div>
        <div>
          <label htmlFor={prefix + 'fps'}>FPS</label>
          <select id={prefix + 'fps'} value={fps} disabled={locked} onChange={e => setFPS(Number(e.target.value))}>
            {[15, 24, 30].map(value => <option key={value} value={value}>{value} FPS</option>)}
          </select>
        </div>
      </div>
    </div>
  )
}
