import { useState } from 'react'
import { useStreamSession, useLeaveSession } from './hooks/useSession'
import { useDisplays } from './hooks/useDisplays'
import { useFrameStream } from './hooks/useFrameStream'
import { inactive } from './session-lifecycle'

const labels: Record<string,string> = {
  Idle:'Chưa kết nối', Preparing:'Đang chuẩn bị relay', WaitingForViewer:'Đang chờ người xem',
  Connecting:'Đang kết nối relay', Authenticating:'Đang xác thực peer', AwaitingKeyframe:'Đang chờ KEY đầu tiên',
  Live:'Đang chia sẻ', Watching:'Đang xem', Recovering:'Đang phục hồi ảnh',
  Stopping:'Đang đóng phiên', Disconnected:'Đã ngắt kết nối', Error:'Không kết nối được',
}

export function Stream({ role, onHome }: { role:'share'|'watch'; onHome:()=>void }) {
  const session=useStreamSession(role)
  const {state,error,started,stats,invitation,copyStatus}=session
  const {displays,display,setDisplay,refresh}=useDisplays(role==='share',session.setError,'Không đọc được danh sách màn hình.')
  const {canvas,error:frameError,hasImage,renderFPS,reconnect}=useFrameStream(started,'stream')
  const imageError=frameError||(started?stats?.previewError:'')
  const [resolution,setResolution]=useState('1280x720')
  const [fps,setFPS]=useState(24)
  const [tau,setTau]=useState(4)
  const [keyInterval,setKeyInterval]=useState(30)
  const [address,setAddress]=useState('')
  const [password,setPassword]=useState('')
  const [code,setCode]=useState('')
  const start=async()=>{
    const [width,height]=resolution.split('x').map(Number)
    if(await session.start({displayID:display,width,height,fps,tau,keyInterval},{address,password},code)){
      setPassword('');setCode('')
    }
  }
  const stop=()=>{
    setCode('');setPassword('')
    return session.stop()
  }
  const {leaving,home}=useLeaveSession(stop,onHome)
  const retryAfter=stats?.retryAfterMs??0
  const locked=!inactive(state)
  const validStats=stats?.measurementsComplete===true&&stats.generation===started?.generation&&!inactive(state)&&
    (role==='share'?stats.counters.emitted>0:stats.counters.received>0)
  const sessionStats=stats&&started&&stats.generation===started.generation&&!inactive(state)
  return <><nav><button disabled={leaving} onClick={()=>void home()}>← Dừng phiên và về Home</button></nav><main>
    <header><div><span className="eyebrow">CrocStream</span><h1>{role==='share'?'Chia sẻ màn hình':'Xem một phiên'}</h1></div>
      <span className={`status state-${state}`} role="status">{labels[state]??state}</span></header>
    <div className="workspace">
      <section className="viewer">
        <div className="section-heading"><h2>{role==='share'?'Màn hình của bạn':'Màn hình người chia sẻ'}</h2><span>{role==='share'?'Xem trước tại máy này':'Ảnh nhận từ phiên'}</span></div>
        <div className="canvas-wrap"><canvas ref={canvas} width={1280} height={720} aria-label={role==='share'?'Preview màn hình đang phát':'Ảnh nhận từ người phát'} />
          {!hasImage&&<div className="placeholder"><strong>{started?labels[state]??'Đang chờ ảnh…':role==='share'?'Chưa chia sẻ màn hình':'Chưa tham gia phiên'}</strong><p>{role==='share'?'Chọn nguồn và relay ở bên phải.':'Nhập mã mời và relay ở bên phải.'}</p></div>}
          {hasImage&&state==='Recovering'&&<div className="recovery" role="status">Đang phục hồi · giữ ảnh hợp lệ cuối cùng</div>}</div>
        {imageError&&<><p className="error" role="status">{imageError}</p>{started&&<button onClick={reconnect}>Thử lại hình hiển thị</button>}</>}
        <div className="metrics">
          <Metric label="FPS hiển thị" value={started?String(renderFPS):'—'} />
          <Metric label="Tốc độ video (KiB/s)" help="Byte KEY/DELTA, gồm header, trong 1 giây gần nhất; không gồm control, mã hóa hay overhead mạng." value={validStats?(role==='share'?stats.sentKiB:stats.receivedKiB).toFixed(1):'—'} />
          <Metric label="RTT khứ hồi (ms)" help="Ping/Pong gần nhất; không phải độ trễ một chiều hay end-to-end." value={validStats&&stats.counters.rttKnown?stats.counters.rttMs.toFixed(1):'N/A'} />
        </div>
        <details className="stream-details"><summary>Chi tiết luồng</summary><div className="metrics metrics-detail">
          <Metric label={role==='share'?'FPS chụp':'FPS nhận'} value={sessionStats&&stats.measurementsComplete?String(role==='share'?stats.captureFPS:stats.receivedFPS):'—'} />
          <Metric label={role==='share'?'FPS gửi':'FPS giải mã'} value={validStats?String(role==='share'?stats.emittedFPS:stats.decodedFPS):'—'} />
          <Metric label="KEY / DELTA" value={validStats?`${stats.counters.keys} / ${stats.counters.deltas}`:'—'} />
          <Metric label="KEY trung bình (KiB)" value={validStats&&stats.keyMeanKnown?stats.meanKeyKiB.toFixed(2):'—'} />
          <Metric label="DELTA trung bình (KiB)" value={validStats&&stats.deltaMeanKnown?stats.meanDeltaKiB.toFixed(2):'—'} />
          <Metric label="Mẫu chụp bỏ qua" value={validStats&&role==='share'?String(stats.dropped):'—'} />
          <Metric label="Nén toàn ảnh (KiB/s)" help="Nén LZ4 toàn ảnh trên cùng mẫu đã gửi, gồm header, trong cùng cửa sổ 1 giây." value={validStats&&role==='share'&&stats.baselineKnown?stats.baselineKiB.toFixed(1):'—'} />
          <Metric label="Tiết kiệm so với toàn ảnh" help="100 × (1 − byte video / byte nén toàn ảnh), cùng cửa sổ 1 giây. Có thể âm." value={validStats&&role==='share'&&stats.savingsKnown?`${stats.savings.toFixed(1)}%`:'—'} />
          <Metric label="Thời lượng phiên (giây)" help="Tính từ lúc chuẩn bị, gồm cả thời gian chờ người xem." value={sessionStats?stats.uptime.toFixed(0):'—'} />
          <Metric label="Khung gần nhất" help="KEY/DELTA cuối được gửi hoặc giải mã; không xác nhận canvas đã hiển thị khung đó." value={validStats?stats.counters.lastKind||'—':'—'} />
          <Metric label="Giải mã đến hiển thị (ms)" help="Chưa đo: ảnh JPEG cục bộ chưa có dấu thời gian liên kết với bước giải mã LZ4." value="N/A" />
        </div>
        <p className="caption">Tốc độ tính trong 1 giây gần nhất; cập nhật mỗi 500 ms. KEY/DELTA và mẫu bỏ là số cộng dồn. Mức tiết kiệm so sánh với nén toàn ảnh cùng mẫu, có thể âm. Byte gồm header, không gồm control hoặc overhead mạng. RTT là khứ hồi; JPEG chỉ dùng để hiển thị.</p></details>
        {stats?.measurementOverflow&&<p role="status" className="error">Cửa sổ đo chưa đầy đủ; tạm ẩn tốc độ và savings.</p>}
      </section>
      <aside><h2>{role==='share'?'Thiết lập chia sẻ':'Kết nối'}</h2>
        {invitation&&<div className="invitation"><label htmlFor="invitation">Mã mời</label>
          <input id="invitation" type="password" readOnly value={invitation} autoComplete="off" />
          <button onClick={()=>void session.copyInvitation()}>Sao chép mã</button>
          <p role="status" className="caption">{copyStatus}</p>
          <p className="caption">Chỉ gửi cho người xem. Dừng phiên để thu hồi mã.</p></div>}
        <label htmlFor="relay">Địa chỉ relay</label>
        <input id="relay" value={address} disabled={locked} onChange={e=>setAddress(e.target.value)} placeholder="127.0.0.1:9009" autoComplete="off" />
        <label htmlFor="relay-password">Mật khẩu relay</label>
        <input id="relay-password" type="password" value={password} disabled={locked} onChange={e=>setPassword(e.target.value)} autoComplete="off" />
        <p className="field-help">Dùng cùng relay trên cả hai máy. Mật khẩu chỉ giữ trong phiên.</p>
        {role==='share'?<>
          <label htmlFor="stream-display">Màn hình</label><select id="stream-display" value={display} disabled={locked} onChange={e=>setDisplay(e.target.value)}>
            {!displays.length&&<option value="">Không có màn hình</option>}
            {displays.map((d,i)=><option key={d.id} value={d.id}>Màn hình {i+1} · {d.width}×{d.height}</option>)}</select>
          <button className="secondary" disabled={locked} onClick={()=>void refresh()}>Làm mới danh sách</button>
          <label htmlFor="stream-resolution">Độ phân giải</label><select id="stream-resolution" value={resolution} disabled={locked} onChange={e=>setResolution(e.target.value)}>
            <option value="1280x720">1280 × 720</option><option value="640x360">640 × 360</option></select>
          <label htmlFor="stream-fps">FPS mục tiêu</label><select id="stream-fps" value={fps} disabled={locked} onChange={e=>setFPS(Number(e.target.value))}>
            {[15,24,30].map(n=><option key={n} value={n}>{n} FPS</option>)}</select>
          <details><summary>Nén ảnh nâng cao</summary>
            <label htmlFor="stream-tau">Ngưỡng RGB MAE (tau)</label><input id="stream-tau" type="number" min="0" max="255" value={tau} disabled={locked} onChange={e=>setTau(Math.max(0,Math.min(255,Number(e.target.value))))} />
            <label htmlFor="stream-keyinterval">Khoảng KEY (1–120 frame)</label><input id="stream-keyinterval" type="number" min="1" max="120" value={keyInterval} disabled={locked} onChange={e=>setKeyInterval(Math.max(1,Math.min(120,Number(e.target.value))))} />
          </details>
           <p className="notice">Bắt đầu sẽ chụp màn hình đã chọn. Người có mã mời có thể tham gia xem.</p>
        </>:<><label htmlFor="join-code">Mã phiên</label><input id="join-code" type="password" value={code} disabled={locked} onChange={e=>setCode(e.target.value)} autoComplete="off" /></>}
        <button className="primary" disabled={locked||leaving||!address||!password||(role==='share'?!display:!code||retryAfter>0)} onClick={()=>void start()}>{role==='share'?'Bắt đầu chia sẻ':'Tham gia phiên'}</button>
        {role==='watch'&&retryAfter>0&&!locked&&<p role="status" className="caption">Thử lại sau {Math.ceil(retryAfter/1000)} giây. Không tự động kết nối lại.</p>}
        <button className="stop" disabled={!locked} onClick={()=>void stop()}>{role==='share'?'Dừng chia sẻ':'Rời phiên'}</button>
        {error&&<p className="error" role="alert">{error}</p>}
      </aside>
    </div>
  </main></>
}
function Metric({label,value,help='Số đo thật trong cửa sổ 1 giây gần nhất; — khi chưa có mẫu.'}:{label:string;value:string;help?:string}) {return <div tabIndex={0} title={help} aria-label={`${label}: ${value}. ${help}`}><span>{label}</span><strong>{value}</strong></div>}
