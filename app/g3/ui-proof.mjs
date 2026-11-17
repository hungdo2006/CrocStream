// Explicitly owner-authorized UI automation. No screenshots, traces, frame
// payloads, tokens or WebSocket headers are written to disk or stdout.
import { chromium } from '../frontend/node_modules/playwright-core/index.mjs'
import { spawn, execFileSync } from 'node:child_process'
import { once } from 'node:events'
import net from 'node:net'

const [executable, secondsText = '10', mode = 'packaged', finish = 'stop'] = process.argv.slice(2)
if (!executable) throw new Error('Executable required')
const seconds = Number(secondsText)
if (!Number.isFinite(seconds) || seconds < 1 || seconds > 610) throw new Error('Duration must be 1..610s')
if (!['stop', 'close-live'].includes(finish)) throw new Error('Finish must be stop or close-live')
const server = net.createServer()
server.listen(0, '127.0.0.1')
await once(server, 'listening')
const port = server.address().port
await new Promise(resolve => server.close(resolve))
const cli = mode === 'dev-cli'
const child = spawn(executable, cli ? ['dev', '-host', '127.0.0.1', '-port', '9245', '-tags', 'g3test', '-quiet'] : [], {
  env: { ...process.env, CROCSTREAM_TEST_CDP_PORT: String(port), WAILS_EXP_USE_WAKE: '1' },
  stdio: 'ignore',
})
let browser, page, started = false
const delay = ms => new Promise(resolve => setTimeout(resolve, ms))
try {
  const deadline = Date.now() + (cli ? 60000 : 20000)
  while (Date.now() < deadline) {
    try { browser = await chromium.connectOverCDP(`http://127.0.0.1:${port}`); break } catch { await delay(200) }
  }
  if (!browser) throw new Error('WebView CDP did not become available')
  page = browser.contexts()[0].pages()[0]
  if (!page) throw new Error('No WebView page')
  await page.locator('main').waitFor({ timeout: 15000 })
  const localChoice = page.getByRole('button', { name: 'Local preview · Xem trước', exact: true })
  if (await localChoice.count()) await localChoice.click()
  const errors = []
  page.on('pageerror', () => errors.push('page exception'))
  const previewAddresses = new Set()
  page.on('websocket', socket => {
    const address = new URL(socket.url())
    if (address.hostname === '127.0.0.1' && address.pathname === '/preview') previewAddresses.add(address.port)
  })
  await page.getByRole('button', { name: 'Bắt đầu preview', exact: true }).waitFor({ timeout: 15000 })
  await page.waitForFunction(() => document.querySelector('#display')?.value, null, { timeout: 15000 })
  const getStats = () => page.evaluate(async () => {
    const { Call } = await import('/wails/runtime.js')
    return Call.ByName('crocstream.local/app/internal/desktop.Service.GetStats')
  })
  const diagnostics = () => page.evaluate(async () => {
    const { Call } = await import('/wails/runtime.js')
    return Call.ByName('crocstream.local/app/internal/desktop.Service.GetDiagnostics')
  })
  const beforeStats = await getStats()
  const beforeResources = await diagnostics()
  if (beforeStats.captured !== 0) throw new Error('Capture occurred before Start')
  console.log(JSON.stringify({ phase: 'ready', at: new Date().toISOString(), mode, origin: new URL(page.url()).origin,
    displays: await page.locator('#display option').count(),
    startEnabled: await page.getByRole('button', { name: 'Bắt đầu preview', exact: true }).isEnabled(),
    resources: beforeResources }))
  await page.locator('#display').focus()
  await page.keyboard.press('Tab')
  if (!(await page.getByRole('button', { name: 'Làm mới danh sách' }).evaluate(el => el === document.activeElement))) {
    throw new Error('Keyboard focus navigation failed')
  }
  await page.getByRole('button', { name: 'Bắt đầu preview', exact: true }).click()
  started = true
  await page.getByRole('status').filter({ hasText: 'Đang xem trước cục bộ' }).waitFor({ timeout: 15000 })
  await page.waitForFunction(() => {
    const labels = [...document.querySelectorAll('.metrics div')]
    return labels.some(d => d.querySelector('span')?.textContent === 'Render FPS' && Number(d.querySelector('strong')?.textContent) > 0)
  }, null, { timeout: 15000 })
  const begin = Date.now()
  const samples = []
  const resources = []
  while (Date.now() - begin < seconds * 1000) {
    await delay(1000)
    samples.push(await page.locator('.metrics div strong').allTextContents())
    if (samples.length % 30 === 0 || samples.length === 1) resources.push({ seconds: (Date.now() - begin) / 1000, ...await diagnostics() })
    if (await page.getByRole('alert').count()) throw new Error('Application reported capture error')
    if (await page.getByText('Preview unavailable', { exact: false }).count()) throw new Error('Preview failed')
    if (samples.length === 120) console.log(JSON.stringify({ phase: 'two-minute', stats: await getStats(),
      renderFPSMean: samples.reduce((sum, sample) => sum + Number(sample[2]), 0) / samples.length }))
  }
  const stopAt = Date.now()
  const finalStats = await getStats()
  const canvasHasImage = await page.locator('canvas').evaluate(canvas => {
    const pixels = canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data
    for (let i = 0; i < pixels.length; i += 4) {
      if (pixels[i] !== 0 || pixels[i + 1] !== 0 || pixels[i + 2] !== 0) return true
    }
    return false
  })
  if (!canvasHasImage) throw new Error('Rendered preview is entirely black')
  console.log(JSON.stringify({ phase: 'pre-stop', mode, measuredSeconds: (Date.now() - begin) / 1000,
    canvasHasImage, finalStats, resources, renderFPSMean: samples.reduce((sum, sample) => sum + Number(sample[2]), 0) / samples.length }))
  if (finish === 'stop') {
  await page.getByRole('button', { name: 'Dừng preview', exact: true }).focus()
  await page.keyboard.press('Enter')
  await page.getByRole('status').filter({ hasText: 'Chưa chụp màn hình' }).waitFor({ timeout: 5000 })
  started = false
  const clear = await page.locator('canvas').evaluate(canvas => {
    const pixels = canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data
    return pixels.every(byte => byte === 0)
  })
  console.log(JSON.stringify({ phase: 'completed', mode, seconds: (Date.now() - begin) / 1000,
    samples, resources, finalStats, afterResources: await diagnostics(),
    stopMs: Date.now() - stopAt, canvasCleared: clear, pageErrors: errors.length,
    settingsUnlocked: await page.locator('#display').isEnabled() }))
  if (!clear || errors.length) throw new Error('Cleanup/UI error')
  // Viewport/layout checks do not record the canvas or image pixels.
  for (const [width, height] of [[960, 640], [1150, 760]]) {
    await page.evaluate(async ({ width, height }) => {
      const { Window } = await import('/wails/runtime.js')
      await Window.SetSize(width, height)
    }, { width, height })
    await page.waitForFunction(() => document.documentElement.scrollWidth <= window.innerWidth)
    const accessible = await page.locator('main').ariaSnapshot()
    if (!accessible.includes('Bắt đầu preview') || !accessible.includes('Màn hình')) throw new Error('Accessible controls missing')
  }
  // Rapid prepare/stop must not leave a late Start capturing in the background.
  for (let i = 0; i < 5; i++) {
    await page.getByRole('button', { name: 'Bắt đầu preview', exact: true }).focus()
    await page.keyboard.press('Enter')
    await page.getByRole('button', { name: 'Dừng preview', exact: true }).focus()
    await page.keyboard.press('Enter')
    await page.getByRole('status').filter({ hasText: 'Chưa chụp màn hình' }).waitFor({ timeout: 5000 })
  }
  await delay(600)
  if ((await getStats()).state !== 'Idle') throw new Error('Rapid Stop left capture active')
  console.log(JSON.stringify({ phase: 'layout-lifecycle', sizes: ['960x640', '1150x760'], rapidCycles: 5, state: 'Idle' }))
  }
  // Close the native window through its runtime rather than killing the process.
  await page.evaluate(async () => {
    const runtime = await import('/wails/runtime.js')
    await runtime.Application.Quit()
  }).catch(() => {})
  if (cli) {
    await delay(1000)
    if (!page.isClosed()) throw new Error('Native dev window did not close')
    console.log(JSON.stringify({ phase: 'exit', graceful: true, watcherCleanup: 'owned process tree' }))
  } else {
    const exited = await Promise.race([once(child, 'exit').then(() => true), delay(5000).then(() => false)])
    console.log(JSON.stringify({ phase: 'exit', graceful: exited }))
    if (!exited) throw new Error('Native window did not exit gracefully')
  }
  started = false
  for (const previewPort of previewAddresses) {
    const closed = await new Promise(resolve => {
      const socket = net.createConnection({ host: '127.0.0.1', port: Number(previewPort) })
      const done = result => { socket.destroy(); resolve(result) }
      socket.once('connect', () => done(false))
      socket.once('error', () => done(true))
      socket.setTimeout(1000, () => done(false))
    })
    if (!closed) throw new Error('Preview listener remained after exit')
  }
  console.log(JSON.stringify({ phase: 'listener-cleanup', at: new Date().toISOString(), finish, closed: previewAddresses.size, pageErrors: errors.length }))
} catch (error) {
  if (page) console.log(JSON.stringify({ phase: 'failure', status: await page.locator('[role="status"]').allTextContents(),
    alerts: await page.locator('[role="alert"]').allTextContents(), metrics: await page.locator('.metrics').allTextContents() }))
  throw error
} finally {
  if (started && page) {
    await page.getByRole('button', { name: 'Dừng preview', exact: true }).click({ timeout: 2000 }).catch(() => {})
  }
  await browser?.close().catch(() => {})
  if (child.exitCode === null) {
    if (cli) {
      try { execFileSync('taskkill', ['/PID', String(child.pid), '/T', '/F'], { stdio: 'ignore' }) } catch {}
    } else child.kill()
  }
}
