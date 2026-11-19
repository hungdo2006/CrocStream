import { useState } from 'react'
import type { Options } from './api'
import { useDisplays } from './hooks/useDisplays'

const resolutions = {
  '1280x720': { width: 1280, height: 720 },
  '640x360': { width: 640, height: 360 },
}
type Resolution = keyof typeof resolutions

export function useCaptureSettings(enabled: boolean, setError: (message: string) => void, errorMessage: string) {
  const source = useDisplays(enabled, setError, errorMessage)
  const [resolution, setResolution] = useState<Resolution>('1280x720')
  const [fps, setFPS] = useState(24)

  function options(tau = 4, keyInterval = 30): Options {
    return { displayID: source.display, ...resolutions[resolution], fps, tau, keyInterval }
  }

  return { ...source, resolution, setResolution, fps, setFPS, options }
}

export function CaptureSettings({ settings, mode, locked }: {
  settings: ReturnType<typeof useCaptureSettings>
  mode: 'local' | 'share'
  locked: boolean
}) {
  const local = mode === 'local'
  const prefix = local ? '' : 'stream-'
  const { displays, display, setDisplay, refresh, resolution, setResolution, fps, setFPS } = settings

  return <>
    <label htmlFor={prefix + 'display'}>Màn hình</label>
    <select id={prefix + 'display'} value={display} disabled={locked} onChange={e => setDisplay(e.target.value)}>
      {!displays.length && <option value="">{local ? 'Chưa tìm thấy màn hình' : 'Không có màn hình'}</option>}
      {displays.map((item, index) => <option key={item.id} value={item.id}>
        Màn hình {index + 1}{local && item.primary ? ' · Chính' : ''}{local ? ' — ' : ' · '}{item.width}×{item.height}
      </option>)}
    </select>
    <button className="secondary" disabled={locked} onClick={() => void refresh()}>Làm mới danh sách</button>
    <label htmlFor={prefix + 'resolution'}>Độ phân giải</label>
    <select id={prefix + 'resolution'} value={resolution} disabled={locked} onChange={e => setResolution(e.target.value as Resolution)}>
      {Object.entries(resolutions).map(([value, { width, height }]) =>
        <option key={value} value={value}>{width} × {height}</option>,
      )}
    </select>
    <label htmlFor={prefix + 'fps'}>FPS mục tiêu</label>
    <select id={prefix + 'fps'} value={fps} disabled={locked} onChange={e => setFPS(Number(e.target.value))}>
      {[15, 24, 30].map(value => <option key={value} value={value}>{value} FPS</option>)}
    </select>
  </>
}
