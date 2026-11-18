import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import { api, streamAPI, type Options, type Relay } from '../api'
import { SessionLifecycle } from '../session-lifecycle'

const previewMessages = {
  start: 'Không bắt đầu được preview. Kiểm tra màn hình và trạng thái rồi thử lại.',
  stop: 'Dừng quá thời gian; đang chờ Windows giải phóng capture.',
  poll: 'Không đọc được trạng thái ứng dụng.',
}
const streamMessages = {
  start: 'Không bắt đầu được phiên. Kiểm tra relay, mã và nguồn màn hình; nếu vừa thử kết nối, chờ rồi thử lại.',
  stop: 'Đang chờ đóng tài nguyên; phiên vẫn ở trạng thái Stopping.',
  poll: 'Không đọc được trạng thái phiên.',
}

function useLifecycle<T extends { state: string; generation: number; error: string }>(lifecycle: SessionLifecycle<T>) {
  const view = useSyncExternalStore(lifecycle.subscribe, lifecycle.getSnapshot)
  useEffect(() => {
    lifecycle.connect()
    return () => lifecycle.disconnect()
  }, [lifecycle])
  return view
}

export function usePreviewSession() {
  const lifecycle = useMemo(() => new SessionLifecycle(api, previewMessages), [])
  const view = useLifecycle(lifecycle)
  return { ...view, setError: lifecycle.setError, stop: lifecycle.stop,
    start: (options: Options) => lifecycle.start(() => api.start(options), 'Preparing') }
}

export function useStreamSession(role: 'share' | 'watch') {
  const lifecycle = useMemo(() => new SessionLifecycle({
    stats: streamAPI.stats, stop: streamAPI.stop,
    invitation: role === 'share' ? streamAPI.invitation : undefined,
  }, streamMessages), [role])
  const view = useLifecycle(lifecycle)
  return { ...view, setError: lifecycle.setError, stop: lifecycle.stop,
    start: (options: Options, relay: Relay, code: string) => lifecycle.start(
      () => role === 'share' ? streamAPI.host(options, relay) : streamAPI.join(code, relay),
      role === 'share' ? 'Preparing' : 'Connecting'),
    copyInvitation: () => lifecycle.copyInvitation(value => navigator.clipboard.writeText(value)),
  }
}

export function useLeaveSession(stop: () => Promise<boolean>, onHome: () => void) {
  const [leaving, setLeaving] = useState(false)
  const pending = useRef(false), mounted = useRef(false)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])
  const home = async () => {
    if (pending.current) return
    pending.current = true
    setLeaving(true)
    try { if (await stop() && mounted.current) onHome() }
    finally {
      pending.current = false
      if (mounted.current) setLeaving(false)
    }
  }
  return { leaving, home }
}
