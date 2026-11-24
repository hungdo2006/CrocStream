// Actual two-process desktop proof. Never saves screenshots, pixels, invitation,
// relay password, WebSocket headers or traces. Secrets only in private IPC.
import { chromium } from '../../app/frontend/node_modules/playwright-core/index.mjs'
import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { createInterface } from 'node:readline'
import { randomBytes, randomUUID } from 'node:crypto'
import { join } from 'node:path'
import net from 'node:net'

const [executable, relayExecutable, secondsText='15'] = process.argv.slice(2)
const duration=Number(secondsText)
if(!executable||!relayExecutable||!Number.isFinite(duration)||duration<5||duration>610)throw new Error('Invalid proof arguments')
const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms))
const children=[],browsers=[]
const password=randomBytes(32).toString('hex')
const relay=spawn(relayExecutable,['-mode','relay'],{stdio:['pipe','pipe','ignore']})
children.push(relay)
relay.on('error',()=>{}) // Readiness timeout is reported without private stderr.
relay.stdin.end(JSON.stringify({Address:'127.0.0.1:0',Password:password,Duration:(duration+90)*1e9})+'\n')
relay.stdin.on('error',()=>{})
const lines=createInterface({input:relay.stdout})
let ready
const ports=new Set()
const errors=[]
const observedStates={host:new Set(),viewer:new Set()}
async function freePort(){
  const server=net.createServer();server.listen(0,'127.0.0.1');await once(server,'listening')
  const port=server.address().port
  await new Promise(resolve=>server.close(resolve));return port
}
async function startApp(role){
  const port=await freePort()
  const child=spawn(executable,[],{env:{...process.env,CROCSTREAM_TEST_CDP_PORT:String(port),
    CROCSTREAM_TEST_WEBVIEW_PATH:join(process.env.LOCALAPPDATA,'Temp','opencode',`g4-webview-${randomUUID()}`)},stdio:'ignore'})
  children.push(child)
  child.on('error',()=>errors.push(role))
  let browser
  const deadline=Date.now()+20000
  while(Date.now()<deadline){try{browser=await chromium.connectOverCDP(`http://127.0.0.1:${port}`);break}catch{await delay(200)}}
  if(!browser)throw new Error('Desktop debug endpoint unavailable')
  browsers.push(browser)
  const page=browser.contexts()[0].pages()[0]
  page.on('pageerror',()=>errors.push(role))
  page.on('websocket',ws=>{
    const u=new URL(ws.url())
    if(u.hostname==='127.0.0.1'&&['/preview','/viewer'].includes(u.pathname))ports.add(u.port)
  })
  await page.getByTestId(role==='host'?'home-share':'home-watch').click()
  await page.locator('#relay').fill(ready.address)
  await page.locator('#relay-password').fill(password)
  return {page,child}
}
const call=(page,name)=>page.evaluate(async name=>{
  const {Call}=await import('/wails/runtime.js')
  return Call.ByName(`crocstream.local/app/internal/desktop.Service.${name}`)
},name)
const until=async(check,timeout=15000)=>{
  const deadline=Date.now()+timeout
  while(Date.now()<deadline){if(await check())return;await delay(100)}
  throw new Error('Expected state not reached')
}
const canvasImage=page=>page.locator('canvas').evaluate(canvas=>{
  const p=canvas.getContext('2d').getImageData(0,0,canvas.width,canvas.height).data
  for(let i=0;i<p.length;i+=4)if(p[i]||p[i+1]||p[i+2])return true
  return false
})
try{
  ready=await Promise.race([once(lines,'line').then(([line])=>JSON.parse(line)),delay(15000).then(()=>{throw new Error('Relay startup timeout')})])
  if(ready.event!=='relay_ready')throw new Error('Relay startup failed')
  const host=await startApp('host'),viewer=await startApp('viewer')
  const initialHost=await call(host.page,'GetDiagnostics'),initialViewer=await call(viewer.page,'GetDiagnostics')
  if((await call(host.page,'GetSessionStats')).counters.emitted!==0)throw new Error('Media before Start')
  await host.page.getByRole('button',{name:'Bắt đầu chia sẻ',exact:true}).click()
  await until(async()=>(await call(host.page,'GetSessionStats')).state==='WaitingForViewer')
  observedStates.host.add('WaitingForViewer')
  await until(()=>canvasImage(host.page))
  let code=await call(host.page,'GetInvitation')
  if(!code)throw new Error('Invitation missing after preparation')
  await viewer.page.locator('#join-code').fill(code);code=''
  await viewer.page.getByRole('button',{name:'Tham gia phiên',exact:true}).click()
  await until(async()=>(await call(viewer.page,'GetSessionStats')).state==='Watching',30000)
  observedStates.viewer.add('Watching')
  if(!(await call(host.page,'GetSessionStats')).counters.lastID)throw new Error('Authenticated session did not emit media')
  await until(()=>canvasImage(viewer.page))
  await until(async()=>(await call(host.page,'GetSessionStats')).state==='Live')
  console.log(JSON.stringify({phase:'paired',at:new Date().toISOString(),initialHost,initialViewer,
    hostOrigin:new URL(host.page.url()).origin,viewerOrigin:new URL(viewer.page.url()).origin}))
  const begin=Date.now()
  const resources=[]
  let invalidated=false
  while(Date.now()-begin<duration*1000){
    await delay(1000)
    const stats=await call(viewer.page,'GetSessionStats')
    observedStates.viewer.add(stats.state)
    observedStates.host.add((await call(host.page,'GetSessionStats')).state)
    if(stats.state==='Error'||stats.state==='Disconnected')throw new Error('Unexpected disconnect')
    if(!invalidated&&Date.now()-begin>=2000){
      await call(viewer.page,'InvalidateViewerReference');invalidated=true
    }
    if(resources.length===0 || Date.now()-begin-resources.at(-1).ms>=30000)
      resources.push({ms:Date.now()-begin,host:await call(host.page,'GetDiagnostics'),viewer:await call(viewer.page,'GetDiagnostics')})
  }
  const hostStats=await call(host.page,'GetSessionStats'),viewerStats=await call(viewer.page,'GetSessionStats')
  if(!hostStats.counters.pliAccepted||!viewerStats.counters.pliRequested||!viewerStats.counters.recoveries||
     !hostStats.counters.pongs||!viewerStats.counters.pongs||!viewerStats.counters.deltas)throw new Error('Missing media/control proof')
  const stopStart=Date.now()
  await host.page.getByRole('button',{name:'Dừng chia sẻ',exact:true}).focus()
  await host.page.keyboard.press('Enter')
  await until(async()=>(await call(host.page,'GetSessionStats')).state==='Idle',5000)
  await until(async()=>(await call(viewer.page,'GetSessionStats')).state==='Disconnected',5000)
  await until(async()=>!await canvasImage(host.page)&&!await canvasImage(viewer.page),5000)
  if(await call(host.page,'GetInvitation'))throw new Error('Invitation retained after Stop')
  const stoppedHost=await call(host.page,'GetSessionStats'),stoppedViewer=await call(viewer.page,'GetSessionStats')
  if(!stoppedViewer.counters.ended)throw new Error('Viewer did not receive validated END')
  console.log(JSON.stringify({status:'LOCAL_DESKTOP_MEDIA_PASS',seconds:(Date.now()-begin)/1000,hostStates:[...observedStates.host],viewerStates:[...observedStates.viewer],
    hostStats,viewerStats,stoppedHost,stoppedViewer,stopMs:Date.now()-stopStart,resources,
    afterHost:await call(host.page,'GetDiagnostics'),afterViewer:await call(viewer.page,'GetDiagnostics'),pageErrors:errors.length}))
  if(errors.length)throw new Error('Page errors')
  for(const app of [host,viewer]){
    const exited=once(app.child,'exit')
    await app.page.evaluate(async()=>{const {Application}=await import('/wails/runtime.js');await Application.Quit()}).catch(()=>{})
    await Promise.race([exited,delay(5000).then(()=>{throw new Error('Quit timeout')})])
  }
  for(const port of ports){
    const refused=await new Promise(resolve=>{
      const c=net.createConnection({host:'127.0.0.1',port:Number(port)})
      c.once('connect',()=>{c.destroy();resolve(false)});c.once('error',()=>resolve(true));c.setTimeout(1000,()=>{c.destroy();resolve(false)})
    })
    if(!refused)throw new Error('Preview socket leaked')
  }
  console.log(JSON.stringify({phase:'cleanup',gracefulPeers:true,closedBridgeListeners:ports.size}))
}catch{
  console.error('G4 desktop proof failed (secret-bearing diagnostics suppressed)')
  process.exitCode=1
}finally{
  lines.close()
  for(const browser of browsers)await browser.close().catch(()=>{})
  for(const child of children)if(child.exitCode===null)child.kill()
}
