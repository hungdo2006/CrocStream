// Owner-authorized local capture only. Never record pixels, traces, clipboard,
// socket headers, invitations or credentials. Output is numeric/fixed labels.
import {chromium} from '../frontend/node_modules/playwright-core/index.mjs'
import {spawn} from 'node:child_process'
import {once} from 'node:events'
import {createInterface} from 'node:readline'
import {randomBytes,randomUUID} from 'node:crypto'
import {join} from 'node:path'
import net from 'node:net'
const [exe,relayExe,text='600']=process.argv.slice(2), duration=Number(text)
if(!exe||!relayExe||!Number.isFinite(duration)||duration<10||duration>610)throw Error('Invalid arguments')
const delay=ms=>new Promise(r=>setTimeout(r,ms)), children=[], browsers=[], ports=new Set(), checks=[]
let phase='relay', errors=0
const password=randomBytes(32).toString('hex')
const relay=spawn(relayExe,['-mode','relay'],{stdio:['pipe','pipe','ignore']})
children.push(relay);relay.on('error',()=>errors++);relay.stdin.on('error',()=>{})
relay.stdin.end(JSON.stringify({Address:'127.0.0.1:0',Password:password,Duration:(duration+240)*1e9})+'\n')
const lines=createInterface({input:relay.stdout})
const call=(p,name)=>p.evaluate(async name=>{
  const {Call}=await import('/wails/runtime.js')
  return Call.ByName(`crocstream.local/app/internal/desktop.Service.${name}`)
},name)
async function until(fn,timeout=15000){
  const end=Date.now()+timeout
  while(Date.now()<end){if(await fn())return;await delay(100)}
  throw Error('Timeout')
}
async function freePort(){
  const s=net.createServer();s.listen(0,'127.0.0.1');await once(s,'listening')
  const port=s.address().port;await new Promise(r=>s.close(r));return port
}
async function app(role){
  const port=await freePort()
  const child=spawn(exe,[],{env:{...process.env,CROCSTREAM_TEST_CDP_PORT:String(port),
    CROCSTREAM_TEST_WEBVIEW_PATH:join(process.env.LOCALAPPDATA,'Temp','opencode',`g5-${randomUUID()}`)},stdio:'ignore'})
  children.push(child);child.on('error',()=>errors++)
  let b
  await until(async()=>{try{b=await chromium.connectOverCDP(`http://127.0.0.1:${port}`);return true}catch{return false}},20000)
  browsers.push(b)
  const page=b.contexts()[0].pages()[0]
  page.on('pageerror',()=>errors++)
  page.on('websocket',s=>{const u=new URL(s.url());if(u.hostname==='127.0.0.1'&&['/preview','/viewer'].includes(u.pathname))ports.add(u.port)})
  await page.getByTestId(role==='host'?'home-share':'home-watch').click()
  await page.evaluate(()=>{
    const Native=window.WebSocket
    window.WebSocket=class extends Native {constructor(...args){super(...args);window.__g5Socket=this}}
    window.__g5Decode=window.createImageBitmap
  })
  return {page,child}
}
const image=page=>page.locator('canvas').evaluate(c=>{
  const p=c.getContext('2d').getImageData(0,0,c.width,c.height).data
  for(let i=0;i<p.length;i+=4)if(p[i]||p[i+1]||p[i+2])return true
  return false
})
async function metrics(page){
  return page.locator('.metrics').evaluateAll(groups=>Object.fromEntries(groups.flatMap(g=>[...g.children].map(d=>[
    d.querySelector('span')?.textContent,d.querySelector('strong')?.textContent]))))
}
function quantiles(values){
  const v=values.filter(Number.isFinite).sort((a,b)=>a-b)
  return v.length?{n:v.length,p50:v[Math.floor((v.length-1)*.5)],p95:v[Math.floor((v.length-1)*.95)],max:v.at(-1)}:null
}
async function setRelay(page,address){
  await page.locator('#relay').fill(address);await page.locator('#relay-password').fill(password)
}
async function sizeChecks(page){
  for(const [width,height] of [[960,640],[1150,760]]){
    await page.evaluate(async size=>{const {Window}=await import('/wails/runtime.js');await Window.SetSize(...size)},[width,height])
    await until(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
    const a=await page.locator('main').ariaSnapshot()
    if(!a.includes('relay')||!a.includes('FPS'))throw Error('Accessibility tree missing controls')
    const ratio=await page.locator('canvas').evaluate(c=>c.clientWidth/c.clientHeight)
    if(Math.abs(ratio-16/9)>.03)throw Error('Canvas distorted')
  }
}
try{
  const ready=await Promise.race([once(lines,'line').then(([s])=>JSON.parse(s)),delay(15000).then(()=>{throw Error('Relay timeout')})])
  if(ready.event!=='relay_ready')throw Error('Relay failed')
  phase='startup'
  const host=await app('host'),viewer=await app('viewer')
  const h=host.page,v=viewer.page
  const initialHost=await call(h,'GetDiagnostics'),initialViewer=await call(v,'GetDiagnostics')
  if((await call(h,'GetSessionStats')).captured!==0)throw Error('Capture before Start')
  await sizeChecks(h);await sizeChecks(v)
  await h.locator('#stream-display').focus();await h.keyboard.press('Tab')
  if(!await h.getByRole('button',{name:'Làm mới danh sách',exact:true}).evaluate(e=>e===document.activeElement))throw Error('Keyboard navigation')
  if(await h.locator(':focus-visible').count()!==1)throw Error('Focus indicator')
  checks.push('no-capture-before-start','sizes-960x640-1150x760','accessible-labels-status','keyboard-focus','16:9')
  phase='unreachable'
  const unused=await freePort()
  await setRelay(h,`127.0.0.1:${unused}`)
  await h.getByRole('button',{name:'Bắt đầu chia sẻ',exact:true}).click()
  await until(async()=>(await call(h,'GetSessionStats')).state==='Error')
  await h.getByRole('alert').waitFor()
  if((await call(h,'GetSessionStats')).captured!==0)throw Error('Capture with unavailable relay')
  checks.push('unreachable-relay-actionable')
  phase='inline-validation'
  await setRelay(v,'invalid-address');await v.locator('#join-code').fill('retained-private-input')
  await v.getByRole('button',{name:'Tham gia phiên',exact:true}).click()
  await v.getByRole('alert').waitFor()
  if(await v.locator('#join-code').inputValue()!=='retained-private-input')throw Error('Inline input lost')
  await until(async()=>await v.getByRole('button',{name:'Tham gia phiên',exact:true}).isDisabled())
  checks.push('inline-input-preserved','retry-cooldown-disabled')
  phase='invalid-code'
  await setRelay(v,ready.address);await v.locator('#join-code').fill('invalid')
  await until(()=>v.getByRole('button',{name:'Tham gia phiên',exact:true}).isEnabled())
  await v.getByRole('button',{name:'Tham gia phiên',exact:true}).click()
  await until(async()=>(await call(v,'GetSessionStats')).state==='Error')
  await v.getByRole('alert').waitFor()
  checks.push('invalid-code-actionable')
  phase='pair'
  await setRelay(h,ready.address)
  await h.getByRole('button',{name:'Bắt đầu chia sẻ',exact:true}).click()
  await until(async()=>(await call(h,'GetSessionStats')).state==='WaitingForViewer')
  await until(()=>image(h))
  const waiting=await call(h,'GetSessionStats')
  if(waiting.counters.emitted!==0||waiting.captured===0)throw Error('Waiting telemetry')
  if(!await h.locator('#stream-resolution').isDisabled())throw Error('Unlocked live settings')
  await h.getByRole('button',{name:'Sao chép mã',exact:true}).click()
  await h.getByText('Đã sao chép mã.',{exact:true}).waitFor()
  let invite=await call(h,'GetInvitation')
  await setRelay(v,ready.address);await v.locator('#join-code').fill(invite);invite=''
  await until(()=>v.getByRole('button',{name:'Tham gia phiên',exact:true}).isEnabled())
  await v.getByRole('button',{name:'Tham gia phiên',exact:true}).click()
  await until(async()=>(await call(v,'GetSessionStats')).state==='Watching',30000)
  await until(()=>image(v))
  checks.push('waiting-preview-without-live','copy-feedback','settings-lock','first-key-render')
  phase='image-error'
  await v.evaluate(()=>{window.createImageBitmap=()=>Promise.reject(Error('injected JPEG failure'))})
  await v.getByText('Preview unavailable · Không giải mã được JPEG cục bộ.',{exact:true}).waitFor()
  const before=(await call(v,'GetSessionStats')).counters.decoded
  await delay(600)
  if((await call(v,'GetSessionStats')).counters.decoded<=before)throw Error('Image failure blocked decode')
  await v.evaluate(()=>{window.createImageBitmap=window.__g5Decode})
  await until(async()=>await v.locator('.error').count()===0)
  phase='bridge-reconnect'
  await v.evaluate(()=>window.__g5Socket.close())
  await v.getByRole('button',{name:'Thử lại hình hiển thị',exact:true}).click()
  await until(()=>image(v));await until(async()=>await v.locator('.error').count()===0)
  checks.push('jpeg-failure-does-not-stop-media','manual-local-bridge-reconnect')
  phase='slow-ui'
  const slowBefore=await call(v,'GetSessionStats')
  await v.evaluate(()=>{window.createImageBitmap=async blob=>{
    await new Promise(r=>setTimeout(r,150));return window.__g5Decode(blob)
  }})
  await delay(2200)
  const slowAfter=await call(v,'GetSessionStats')
  if(slowAfter.counters.decoded-slowBefore.counters.decoded<10||slowAfter.state!=='Watching')throw Error('Slow display stalled decoder')
  await v.evaluate(()=>{window.createImageBitmap=window.__g5Decode})
  await until(async()=>Number((await metrics(v))['FPS hiển thị'])>10)
  checks.push('slow-ui-independent-decode')
  phase='stress'
  // Warm up separately. No screenshot is taken by the harness.
  await delay(3000)
  const start=Date.now(),samples=[],resources=[]
  console.log(JSON.stringify({phase:'stress-start',at:new Date().toISOString(),duration,checks,initialHost,initialViewer,waiting}))
  let invalidated=false,checkpoint=0
  while(Date.now()-start<duration*1000){
    await delay(500)
    const [hs,vs,hm,vm]=await Promise.all([call(h,'GetSessionStats'),call(v,'GetSessionStats'),metrics(h),metrics(v)])
    if(hs.state!=='Live'||!['Watching','Recovering'].includes(vs.state))throw Error('Unexpected disconnect')
    if(hs.captureQueue>1||hs.imageQueue>1||vs.imageQueue>1||!hs.measurementsComplete||!vs.measurementsComplete)throw Error('Unbounded or incomplete')
    if(hs.savingsKnown&&Math.abs(hs.savings-100*(1-hs.sentKiB/hs.baselineKiB))>1e-8)throw Error('Savings mismatch')
    if(!['KEY','DELTA'].includes(vm['Khung gần nhất']))throw Error('Frame kind missing')
    samples.push({ms:Date.now()-start,capture:hs.captureFPS,emitted:hs.emittedFPS,decoded:vs.decodedFPS,rendered:Number(vm['FPS hiển thị']),
      hostRendered:Number(hm['FPS hiển thị']),sentKiB:hs.sentKiB,receivedKiB:vs.receivedKiB,baselineKiB:hs.baselineKiB,savings:hs.savingsKnown?hs.savings:null,
      hostRTT:hs.counters.rttKnown?hs.counters.rttMs:null,viewerRTT:vs.counters.rttKnown?vs.counters.rttMs:null,
      encodeMs:hs.meanEncodeMs,decodeMs:vs.meanDecodeMs,baselineMs:hs.meanBaselineMs})
    if(!invalidated&&Date.now()-start>2000){await call(v,'InvalidateViewerReference');invalidated=true}
    if(resources.length===0||Date.now()-start-resources.at(-1).ms>=10000)
      resources.push({ms:Date.now()-start,host:await call(h,'GetDiagnostics'),viewer:await call(v,'GetDiagnostics')})
    if(Date.now()-start-checkpoint>=60000){
      checkpoint=Date.now()-start
      console.log(JSON.stringify({phase:'progress',seconds:checkpoint/1000,emitted:hs.counters.emitted,decoded:vs.counters.decoded}))
    }
  }
  const hostStats=await call(h,'GetSessionStats'),viewerStats=await call(v,'GetSessionStats')
  if(!viewerStats.counters.recoveries||!hostStats.counters.pliAccepted||!viewerStats.counters.pongs)throw Error('Control missing')
  const summary={}
  for(const key of ['capture','emitted','decoded','rendered','hostRendered','hostRTT','viewerRTT','encodeMs','decodeMs','baselineMs'])
    summary[key]=quantiles(samples.map(s=>s[key]))
  phase='stop'
  const stopAt=Date.now()
  await h.getByRole('button',{name:'Dừng chia sẻ',exact:true}).focus();await h.keyboard.press('Enter')
  await until(async()=>(await call(h,'GetSessionStats')).state==='Idle',3000)
  await until(async()=>(await call(v,'GetSessionStats')).state==='Disconnected',3000)
  const stopMs=Date.now()-stopAt
  if(stopMs>3000)throw Error('Slow cleanup')
  await until(async()=>!await image(h)&&!await image(v))
  if(await call(h,'GetInvitation'))throw Error('Code retained')
  if(!(await call(v,'GetSessionStats')).counters.ended)throw Error('END missing')
  const afterHost=await call(h,'GetDiagnostics'),afterViewer=await call(v,'GetDiagnostics')
  if(afterHost.gdiObjects>initialHost.gdiObjects||afterViewer.gdiObjects>initialViewer.gdiObjects)throw Error('GDI leak')
  checks.push('pli-recovery','dashboard-savings-formula','bounded-queues','stop-under-3s','end-disconnected','cleared-canvases-code','gdi-baseline')
  console.log(JSON.stringify({phase:'stress-result',seconds:(Date.now()-start)/1000,stopMs,summary,hostStats,viewerStats,afterHost,afterViewer,resources,samples}))
  phase='rapid-home'
  for(let i=0;i<5;i++){
    await setRelay(h,ready.address)
    await h.getByRole('button',{name:'Bắt đầu chia sẻ',exact:true}).click()
    await h.getByRole('button',{name:'← Dừng phiên và về Home',exact:true}).click()
    await h.getByTestId('home-share').waitFor()
    if((await call(h,'GetSessionStats')).state!=='Idle')throw Error('Back raced Start')
    await h.getByTestId('home-share').click()
  }
  checks.push('five-start-back-cleanup-cycles')
  // Rejoin with fresh generation and terminate the actual native window live.
  phase='rejoin-close-live'
  await setRelay(h,ready.address);await h.getByRole('button',{name:'Bắt đầu chia sẻ',exact:true}).click()
  await until(async()=>(await call(h,'GetSessionStats')).state==='WaitingForViewer')
  await until(()=>v.locator('#relay').isEnabled())
  await setRelay(v,ready.address)
  invite=await call(h,'GetInvitation')
  await v.locator('#join-code').fill(invite);invite=''
  await until(()=>v.getByRole('button',{name:'Tham gia phiên',exact:true}).isEnabled())
  await v.getByRole('button',{name:'Tham gia phiên',exact:true}).click()
  await until(async()=>(await call(v,'GetSessionStats')).state==='Watching',30000)
  if((await call(v,'GetSessionStats')).generation===viewerStats.generation)throw Error('Stale generation')
  await until(()=>image(v))
  const exit=once(host.child,'exit')
  await h.evaluate(async()=>{const {Application}=await import('/wails/runtime.js');await Application.Quit()}).catch(()=>{})
  await Promise.race([exit,delay(5000).then(()=>{throw Error('Close timeout')})])
  await until(async()=>(await call(v,'GetSessionStats')).state==='Disconnected',5000)
  await v.getByRole('button',{name:'← Dừng phiên và về Home',exact:true}).click()
  const exited=once(viewer.child,'exit')
  await v.evaluate(async()=>{const {Application}=await import('/wails/runtime.js');await Application.Quit()}).catch(()=>{})
  await Promise.race([exited,delay(5000).then(()=>{throw Error('Close timeout')})])
  checks.push('fresh-rejoin','window-close-live')
  phase='listener-cleanup'
  for(const port of ports){
    const closed=await new Promise(r=>{
      const s=net.createConnection({host:'127.0.0.1',port:Number(port)})
      s.once('connect',()=>{s.destroy();r(false)});s.once('error',()=>r(true));s.setTimeout(1000,()=>{s.destroy();r(false)})
    })
    if(!closed)throw Error('Listener leak')
  }
  if(errors)throw Error('Page errors')
  console.log(JSON.stringify({status:'G5_LOCAL_UI_PASS',checks,closedListeners:ports.size,pageErrors:errors}))
}catch{
  console.error(JSON.stringify({status:'G5_LOCAL_UI_FAIL',phase,pageErrors:errors}))
  process.exitCode=1
}finally{
  lines.close()
  for(const b of browsers)await b.close().catch(()=>{})
  for(const c of children)if(c.exitCode===null)c.kill()
}
