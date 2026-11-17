import { useEffect, useRef, useState } from 'react'
import * as Service from '../bindings/crocstream.local/app/internal/desktop/service'
import type { Stats as StreamStats, Started } from '../bindings/crocstream.local/app/internal/session/models'
import type { Display } from './api'
import { LatestFrames } from './latest'

const labels: Record<string,string> = {
  Idle:'Chưa kết nối', Preparing:'Đang chuẩn bị relay', WaitingForViewer:'Đang chờ người xem',
  Connecting:'Đang kết nối relay', Authenticating:'Đang xác thực peer', AwaitingKeyframe:'Đang chờ KEY đầu tiên',
  Live:'Đang chia sẻ', Watching:'Đang xem', Recovering:'Đang phục hồi ảnh',
  Stopping:'Đang đóng phiên', Disconnected:'Đã ngắt kết nối', Error:'Không kết nối được',
}
const inactive = (state: string) => ['Idle','Disconnected','Error'].includes(state)

export function Stream({ role, onHome }: { role:'share'|'watch'; onHome:()=>void }) {
  const [displays,setDisplays]=useState<Display[]>([])
  const [display,setDisplay]=useState('')
  const [resolution,setResolution]=useState('1280x720')
  const [fps,setFPS]=useState(24)
  const [tau,setTau]=useState(4)
  const [keyInterval,setKeyInterval]=useState(30)
  const [address,setAddress]=useState('')
  const [password,setPassword]=useState('')
  const [code,setCode]=useState('')
  const [invitation,setInvitation]=useState('')
  const [state,setState]=useState('Idle')
  const [error,setError]=useState('')
  const [imageError,setImageError]=useState('')
  const [started,setStarted]=useState<Started>()
  const [stats,setStats]=useState<StreamStats>()
  const [renderFPS,setRenderFPS]=useState(0)
  const [hasImage,setHasImage]=useState(false)
  const [imageAttempt,setImageAttempt]=useState(0)
  const [copyStatus,setCopyStatus]=useState('')
  const [leaving,setLeaving]=useState(false)
  const [retryAfter,setRetryAfter]=useState(0)
  const reconnectAt=useRef(0)
  const canvas=useRef<HTMLCanvasElement>(null)
  const epoch=useRef(0), active=useRef(false), busy=useRef(false)
  const pending=useRef<Promise<Started>|undefined>(undefined)
  const renders=useRef<number[]>([])
  useEffect(()=>{
    let mounted=true
    let timer:ReturnType<typeof setTimeout>
    if(role==='share') void Service.ListDisplays().then(list=>{
      if(!mounted)return
      setDisplays(list??[]);setDisplay(list?.find(d=>d.primary)?.id??list?.[0]?.id??'')
    }).catch(()=>{if(mounted)setError('Không đọc được danh sách màn hình.')})
    const poll=async()=>{
      const current=epoch.current
      try {
        const next=await Service.GetSessionStats()
        if(!mounted)return
        if(current===epoch.current&&!busy.current){
          setStats(next);setState(next.state)
          setRetryAfter(next.retryAfterMs)
          if(next.error)setError(next.error)
          if(next.previewError)setImageError(next.previewError)
          if(inactive(next.state)){
            active.current=false;setStarted(undefined);setInvitation('')
          } else if(role==='share') {
            const code=await Service.GetInvitation()
            if(mounted&&current===epoch.current&&!busy.current)setInvitation(code)
          }
        }
        renders.current=renders.current.filter(t=>t>performance.now()-1000)
        if(mounted)setRenderFPS(renders.current.length)
      } catch {if(mounted)setError('Không đọc được trạng thái phiên.')}
      if(mounted)timer=setTimeout(()=>void poll(),500)
    }
    void poll()
    return ()=>{
      mounted=false;clearTimeout(timer);epoch.current++
      if(active.current)void (pending.current??Promise.resolve()).catch(()=>{}).then(()=>Service.StopSession()).catch(()=>{})
    }
  },[role])
  useEffect(()=>{
    const target=canvas.current,ctx=target?.getContext('2d')
    if(!target||!ctx)return
    ctx.clearRect(0,0,target.width,target.height);renders.current=[];setHasImage(false)
    if(!started)return
    const frames=new LatestFrames(blob=>createImageBitmap(blob),bitmap=>{
      ctx.clearRect(0,0,target.width,target.height)
      const scale=Math.min(target.width/bitmap.width,target.height/bitmap.height)
      const w=bitmap.width*scale,h=bitmap.height*scale
      ctx.drawImage(bitmap,(target.width-w)/2,(target.height-h)/2,w,h)
      renders.current.push(performance.now())
      renders.current=renders.current.filter(t=>t>performance.now()-1000)
      setImageError('')
      setHasImage(true)
    },()=>setImageError('Preview unavailable · Không giải mã được JPEG cục bộ.'))
    const ws=new WebSocket(started.endpoint.url,['crocstream.preview',`auth.${started.endpoint.token}`])
    ws.binaryType='blob'
    ws.onmessage=event=>{if(event.data instanceof Blob&&event.data.size<=4*1024*1024)frames.push(new Blob([event.data],{type:'image/jpeg'}))}
    ws.onerror=()=>setImageError('Preview unavailable · Lỗi WebSocket cục bộ.')
    ws.onclose=()=>setImageError('Preview unavailable · Kênh hiển thị đã đóng; có thể thử nối lại.')
    return ()=>{frames.dispose();ws.onmessage=ws.onerror=ws.onclose=null;ws.close();ctx.clearRect(0,0,target.width,target.height);renders.current=[]}
  },[started,imageAttempt])
  const start=async()=>{
    if(active.current)return
    active.current=true;busy.current=true
    const current=++epoch.current
    setState(role==='share'?'Preparing':'Connecting');setError('');setImageError('');setInvitation('');setCopyStatus('');setStats(undefined)
    const [width,height]=resolution.split('x').map(Number)
    try{
      const request=role==='share'
        ? Service.StartBroadcast({displayID:display,width,height,fps,tau,keyInterval},{address,password})
        : Service.JoinSession(code,{address,password})
      pending.current=request
      const result=await request
      setPassword('');setCode('')
      if(current===epoch.current)setStarted(result)
    }catch{
      if(current===epoch.current){active.current=false;setState('Error');setError('Không bắt đầu được phiên. Kiểm tra relay, mã và nguồn màn hình; nếu vừa thử kết nối, chờ rồi thử lại.')}
    }finally{pending.current=undefined;if(current===epoch.current)busy.current=false}
  }
  const stop=async()=>{
    epoch.current++;busy.current=true
    setStarted(undefined);setInvitation('');setCode('');setPassword('');setCopyStatus('');setImageError('');setState('Stopping')
    try{await pending.current?.catch(()=>{});await Service.StopSession();active.current=false;setState('Idle');setError('');return true}
    catch{setError('Đang chờ đóng tài nguyên; phiên vẫn ở trạng thái Stopping.');return false}
    finally{busy.current=false}
  }
  const home=async()=>{
    setLeaving(true)
    if(await stop())onHome()
    setLeaving(false)
  }
  const reconnect=()=>{
    if(performance.now()<reconnectAt.current)return
    reconnectAt.current=performance.now()+2000
    setImageAttempt(n=>n+1)
  }
  const locked=!inactive(state)
  const validStats=stats?.measurementsComplete===true&&stats.generation===started?.generation&&!inactive(state)&&
    (role==='share'?stats.counters.emitted>0:stats.counters.received>0)
  const sessionStats=stats&&started&&stats.generation===started.generation&&!inactive(state)
  return <><nav><button disabled={leaving} onClick={()=>void home()}>← Dừng phiên và về Home</button></nav><main>
    <header><div><span className="eyebrow">CROCSTREAM / G5</span><h1>{role==='share'?'Chia sẻ màn hình.':'Xem màn hình từ xa.'}</h1></div>
      <span className={`status state-${state}`} role="status">{labels[state]??state}</span></header>
    <div className="workspace">
      <section className="viewer">
        <div className="section-heading"><h2>{role==='share'?'Local preview · máy phát':'Remote view · máy xem'}</h2><span>Ảnh hiển thị JPEG cục bộ</span></div>
        <div className="canvas-wrap"><canvas ref={canvas} width={1280} height={720} aria-label={role==='share'?'Preview màn hình đang phát':'Ảnh nhận từ người phát'} />
          {!hasImage&&<div className="placeholder"><strong>{started?labels[state]??'Đang chờ ảnh…':role==='share'?'Chọn nguồn và bắt đầu.':'Nhập mã để tham gia.'}</strong><p>Chỉ hiển thị ảnh hợp lệ của phiên hiện tại.</p></div>}
          {hasImage&&state==='Recovering'&&<div className="recovery" role="status">Đang phục hồi · giữ ảnh hợp lệ cuối cùng</div>}</div>
        {imageError&&<><p className="error" role="status">{imageError}</p>{started&&<button onClick={reconnect}>Nối lại kênh hiển thị cục bộ</button>}</>}
        <div className="metrics">
          <Metric label={role==='share'?'Capture FPS':'Received FPS'} value={sessionStats&&stats.measurementsComplete?String(role==='share'?stats.captureFPS:stats.receivedFPS):'—'} />
          <Metric label={role==='share'?'Emitted FPS':'Decoded FPS'} value={validStats?String(role==='share'?stats.emittedFPS:stats.decodedFPS):'—'} />
          <Metric label="Render FPS" value={started?String(renderFPS):'—'} />
          <Metric label="Video protocol rate (KiB/s)" help="Bytes KEY/DELTA gồm header, trong 1 giây gần nhất; không gồm CONFIG/control, mã hóa, relay hoặc NIC." value={validStats?(role==='share'?stats.sentKiB:stats.receivedKiB).toFixed(1):'—'} />
          <Metric label="RTT (ms)" help="Ping/Pong gần nhất bằng đồng hồ monotonic; không phải độ trễ một chiều hay end-to-end." value={validStats&&stats.counters.rttKnown?stats.counters.rttMs.toFixed(1):'N/A'} />
        </div>
        <div className="metrics">
          <Metric label="Keyframe / Delta" value={validStats?`${stats.counters.keys} / ${stats.counters.deltas}`:'—'} />
          <Metric label="Avg Key (KiB)" value={validStats&&stats.keyMeanKnown?stats.meanKeyKiB.toFixed(2):'—'} />
          <Metric label="Avg Delta (KiB)" value={validStats&&stats.deltaMeanKnown?stats.meanDeltaKiB.toFixed(2):'—'} />
          <Metric label="Dropped capture" value={validStats&&role==='share'?String(stats.dropped):'—'} />
          <Metric label="Full-frame LZ4 baseline (KiB/s)" help="Nén toàn ảnh của chính các mẫu đã phát, cùng LZ4 và header, cùng cửa sổ 1 giây. Không ước lượng." value={validStats&&role==='share'&&stats.baselineKnown?stats.baselineKiB.toFixed(1):'—'} />
          <Metric label="Delta savings" help="100 × (1 − video bytes / baseline bytes), cùng cửa sổ 1 giây. Có thể âm; — nếu không có baseline." value={validStats&&role==='share'&&stats.savingsKnown?`${stats.savings.toFixed(1)}%`:'—'} />
          <Metric label="Session uptime (s)" help="Thời gian từ khi chuẩn bị phiên, bao gồm chờ pairing." value={sessionStats?stats.uptime.toFixed(0):'—'} />
          <Metric label="Last frame kind" help="Loại packet video cuối đã phát hoặc decode thành công; không phải xác nhận canvas đã vẽ packet đó." value={validStats?stats.counters.lastKind||'—':'—'} />
          <Metric label="Viewer decode→render (ms)" help="N/A: bridge JPEG chưa mang timestamp liên kết với bước decode LZ4. Không suy ra từ đồng hồ máy phát." value="N/A" />
        </div>
        <details className="metric-help"><summary>Cách đọc số đo</summary><p className="caption">FPS và tốc độ dùng cửa sổ 1 giây; dashboard cập nhật khoảng 500 ms. Avg Key/Delta gồm header trong cùng cửa sổ; số lượng Key/Delta và dropped capture tính cả phiên. Savings có thể bằng 0 hoặc âm. Baseline đo trên chính mẫu phát, không gồm control, mã hóa hoặc overhead mạng. RTT là khứ hồi gần nhất. Luồng mạng RGBA/LZ4; JPEG chỉ dùng để hiển thị cục bộ.</p></details>
        {stats?.measurementOverflow&&<p role="status" className="error">Cửa sổ đo chưa đầy đủ; tạm ẩn tốc độ và savings.</p>}
        {invitation&&<div className="invitation"><label htmlFor="invitation">Mã mời bí mật</label>
          <input id="invitation" type="password" readOnly value={invitation} autoComplete="off" />
          <button onClick={()=>void navigator.clipboard.writeText(invitation).then(()=>setCopyStatus('Đã sao chép mã.')).catch(()=>setError('Không sao chép được mã.'))}>Sao chép mã</button>
          <p role="status" className="caption">{copyStatus}</p>
          <p className="caption">Chỉ gửi cho người xem. Dừng phiên để thu hồi mã.</p></div>}
      </section>
      <aside><h2>{role==='share'?'Nguồn và relay':'Kết nối tới người phát'}</h2>
        <label htmlFor="relay">Relay đã được phép (host:port)</label>
        <input id="relay" value={address} disabled={locked} onChange={e=>setAddress(e.target.value)} placeholder="127.0.0.1:9009" autoComplete="off" />
        <label htmlFor="relay-password">Mật khẩu relay</label>
        <input id="relay-password" type="password" value={password} disabled={locked} onChange={e=>setPassword(e.target.value)} autoComplete="off" />
        {role==='share'?<>
          <label htmlFor="stream-display">Màn hình</label><select id="stream-display" value={display} disabled={locked} onChange={e=>setDisplay(e.target.value)}>
            {!displays.length&&<option value="">Không có màn hình</option>}
            {displays.map((d,i)=><option key={d.id} value={d.id}>Màn hình {i+1} · {d.width}×{d.height}</option>)}</select>
          <button className="secondary" disabled={locked} onClick={()=>void Service.ListDisplays().then(list=>{setDisplays(list??[]);setDisplay(list?.find(d=>d.id===display)?.id??list?.[0]?.id??'')}).catch(()=>setError('Không đọc được danh sách màn hình.'))}>Làm mới danh sách</button>
          <label htmlFor="stream-resolution">Độ phân giải</label><select id="stream-resolution" value={resolution} disabled={locked} onChange={e=>setResolution(e.target.value)}>
            <option value="1280x720">1280 × 720</option><option value="640x360">640 × 360</option></select>
          <label htmlFor="stream-fps">FPS mục tiêu</label><select id="stream-fps" value={fps} disabled={locked} onChange={e=>setFPS(Number(e.target.value))}>
            {[15,24,30].map(n=><option key={n} value={n}>{n} FPS</option>)}</select>
          <details><summary>Nén ảnh nâng cao</summary>
            <label htmlFor="stream-tau">Ngưỡng RGB MAE (tau)</label><input id="stream-tau" type="number" min="0" max="255" value={tau} disabled={locked} onChange={e=>setTau(Math.max(0,Math.min(255,Number(e.target.value))))} />
            <label htmlFor="stream-keyinterval">Khoảng KEY (1–120 frame)</label><input id="stream-keyinterval" type="number" min="1" max="120" value={keyInterval} disabled={locked} onChange={e=>setKeyInterval(Math.max(1,Math.min(120,Number(e.target.value))))} />
          </details>
          <p className="notice">Start chụp màn hình đã chọn và gửi cho một viewer có mã đúng qua relay bạn nhập.</p>
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
