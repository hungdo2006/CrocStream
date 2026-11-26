import type { StreamStats } from './api'

interface StreamMetricsProps {
  role: 'share' | 'watch'
  generation: number | undefined
  sessionActive: boolean
  stats: StreamStats | undefined
  renderFPS: number
}

export function StreamMetrics({ role, generation, sessionActive, stats, renderFPS }: StreamMetricsProps) {
  const sharing = role === 'share'
  const currentStats = sessionActive && generation !== undefined && stats?.generation === generation
    ? stats
    : undefined
  const completeStats = currentStats?.measurementsComplete ? currentStats : undefined
  // Capture and uptime can be measured while the broadcaster waits for a viewer.
  const hasVideoSample = completeStats && (sharing ? completeStats.counters.emitted : completeStats.counters.received) > 0
  const videoStats = hasVideoSample ? completeStats : undefined
  const routeLabel = currentStats?.route === 'lan'
    ? 'LAN trực tiếp'
    : currentStats?.route === 'relay'
      ? 'Relay'
      : '—'

  return <>
    <div className="metrics metrics-4">
      <Metric label="FPS hiển thị" value={generation !== undefined ? String(renderFPS) : '—'} />
      <Metric
        label="Tốc độ video (KiB/s)"
        help="Byte KEY/DELTA, gồm header, trong 1 giây gần nhất; không gồm control, mã hóa hay overhead mạng."
        value={videoStats ? (sharing ? videoStats.sentKiB : videoStats.receivedKiB).toFixed(1) : '—'}
      />
      <Metric
        label="RTT khứ hồi (ms)"
        help="Ping/Pong gần nhất; không phải độ trễ một chiều hay end-to-end."
        value={videoStats?.counters.rttKnown ? videoStats.counters.rttMs.toFixed(1) : 'N/A'}
      />
      <Metric
        label="Đường truyền"
        help="Tuyến thực tế đã bắt tay PAKE thành công (LAN trực tiếp hoặc Relay); không đoán trước khi kết nối."
        value={routeLabel}
      />
    </div>
    <details className="stream-details">
      <summary>Chi tiết luồng</summary>
      <div className="metrics metrics-detail">
        <Metric
          label={sharing ? 'FPS chụp' : 'FPS nhận'}
          value={completeStats ? String(sharing ? completeStats.captureFPS : completeStats.receivedFPS) : '—'}
        />
        <Metric
          label={sharing ? 'FPS gửi' : 'FPS giải mã'}
          value={videoStats ? String(sharing ? videoStats.emittedFPS : videoStats.decodedFPS) : '—'}
        />
        <Metric label="KEY / DELTA" value={videoStats ? `${videoStats.counters.keys} / ${videoStats.counters.deltas}` : '—'} />
        <Metric label="KEY trung bình (KiB)" value={videoStats?.keyMeanKnown ? videoStats.meanKeyKiB.toFixed(2) : '—'} />
        <Metric label="DELTA trung bình (KiB)" value={videoStats?.deltaMeanKnown ? videoStats.meanDeltaKiB.toFixed(2) : '—'} />
        <Metric label="Mẫu chụp bỏ qua" value={sharing && videoStats ? String(videoStats.dropped) : '—'} />
        <Metric
          label="Nén toàn ảnh (KiB/s)"
          help="Nén LZ4 toàn ảnh trên cùng mẫu đã gửi, gồm header, trong cùng cửa sổ 1 giây."
          value={sharing && videoStats?.baselineKnown ? videoStats.baselineKiB.toFixed(1) : '—'}
        />
        <Metric
          label="Tiết kiệm so với toàn ảnh"
          help="100 × (1 − byte video / byte nén toàn ảnh), cùng cửa sổ 1 giây. Có thể âm."
          value={sharing && videoStats?.savingsKnown ? `${videoStats.savings.toFixed(1)}%` : '—'}
        />
        <Metric
          label="Thời lượng phiên (giây)"
          help="Tính từ lúc chuẩn bị, gồm cả thời gian chờ người xem."
          value={currentStats ? currentStats.uptime.toFixed(0) : '—'}
        />
        <Metric
          label="Khung gần nhất"
          help="KEY/DELTA cuối được gửi hoặc giải mã; không xác nhận canvas đã hiển thị khung đó."
          value={videoStats?.counters.lastKind || '—'}
        />
        <Metric
          label="Giải mã đến hiển thị (ms)"
          help="Chưa đo: ảnh JPEG cục bộ chưa có dấu thời gian liên kết với bước giải mã LZ4."
          value="N/A"
        />
      </div>
      <p className="caption">Tốc độ tính trong 1 giây gần nhất; cập nhật mỗi 500 ms. KEY/DELTA và mẫu bỏ là số cộng dồn. Mức tiết kiệm so sánh với nén toàn ảnh cùng mẫu, có thể âm.</p>
    </details>
    {stats?.measurementOverflow && <p role="status" className="error">Cửa sổ đo chưa đầy đủ; tạm ẩn tốc độ và savings.</p>}
  </>
}

function Metric({ label, value, help = 'Số đo thật trong cửa sổ 1 giây gần nhất; — khi chưa có mẫu.' }: {
  label: string
  value: string
  help?: string
}) {
  return <div tabIndex={0} title={help} aria-label={`${label}: ${value}. ${help}`}>
    <span>{label}</span><strong>{value}</strong>
  </div>
}
