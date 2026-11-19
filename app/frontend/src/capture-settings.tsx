import { useCallback, useState } from 'react'
import { ListDisplays } from '../bindings/crocstream.local/app/internal/desktop/service'
import type { Display, Options } from '../bindings/crocstream.local/app/internal/capture/models'

const resolutions = {
  '1280x720': { width: 1280, height: 720 },
  '640x360': { width: 640, height: 360 },
}
type Resolution = keyof typeof resolutions

export function useCaptureSettings(mode: 'local' | 'share') {
  const [displays, setDisplays] = useState<Display[]>([])
  const [display, setDisplay] = useState('')
  const [resolution, setResolution] = useState<Resolution>('1280x720')
  const [fps, setFPS] = useState(24)

  const refresh = useCallback(async (
    onError: (message: string) => void,
    { shouldApply = () => true, preferPrimary = mode === 'local' } = {},
  ) => {
    try {
      const list = (await ListDisplays()) ?? []
      if (!shouldApply()) return
      setDisplays(list)
      setDisplay(previous => {
        if (list.some(item => item.id === previous)) return previous
        const primary = preferPrimary ? list.find(item => item.primary) : undefined
        return primary?.id ?? list[0]?.id ?? ''
      })
    } catch {
      if (!shouldApply()) return
      onError(mode === 'local'
        ? 'Không đọc được danh sách màn hình. Hãy thử làm mới.'
        : 'Không đọc được danh sách màn hình.')
    }
  }, [mode])

  function options(tau = 4, keyInterval = 30): Options {
    return { displayID: display, ...resolutions[resolution], fps, tau, keyInterval }
  }

  return { mode, displays, display, setDisplay, resolution, setResolution, fps, setFPS, refresh, options }
}

export function CaptureSettings({ settings, locked, onRefresh }: {
  settings: ReturnType<typeof useCaptureSettings>
  locked: boolean
  onRefresh: () => void
}) {
  const local = settings.mode === 'local'
  const prefix = local ? '' : 'stream-'
  const { displays, display, resolution, fps, setDisplay, setResolution, setFPS } = settings

  return <>
    <label htmlFor={prefix + 'display'}>Màn hình</label>
    <select id={prefix + 'display'} value={display} disabled={locked} onChange={e => setDisplay(e.target.value)}>
      {!displays.length && <option value="">{local ? 'Chưa tìm thấy màn hình' : 'Không có màn hình'}</option>}
      {displays.map((item, index) => <option key={item.id} value={item.id}>
        Màn hình {index + 1}{local && item.primary ? ' · Chính' : ''}{local ? ' — ' : ' · '}{item.width}×{item.height}
      </option>)}
    </select>
    <button className="secondary" disabled={locked} onClick={onRefresh}>Làm mới danh sách</button>
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
