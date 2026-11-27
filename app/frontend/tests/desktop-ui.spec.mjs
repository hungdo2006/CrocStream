import { test, expect } from '@playwright/test'
import { readFileSync } from 'node:fs'

const labels = JSON.parse(readFileSync(new URL('../../../packaging/ui-labels.json', import.meta.url), 'utf8'))

// Exercise the actual React hooks/DOM with deferred desktop RPCs and synthetic
// JPEGs. No capture, relay traffic, clipboard writes, screenshots or recordings.
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    const idle = generation => ({ state: 'Idle', generation, route: '', error: '', retryAfterMs: 0,
      captured: 0, previewError: '', sentKiB: 0, receivedKiB: 0, baselineKiB: 0,
      meanKeyKiB: 0, meanDeltaKiB: 0, keyMeanKnown: false, deltaMeanKnown: false,
      savingsKnown: false, baselineKnown: false, measurementOverflow: false,
      dropped: 0, captureFPS: 0, emittedFPS: 0, receivedFPS: 0, decodedFPS: 0, encodeFPS: 0, uptime: 0,
      counters: { emitted: 0, received: 0, decoded: 0, keys: 0, deltas: 0, rttKnown: false, lastKind: '' },
      measurementsComplete: true, samplesComplete: true })
    const fake = window.__desktopTest = {
      session: idle(0), preview: idle(0), defaultRelay: { address: '', password: '' },
      displays: [
        { id: 'synthetic', kind: 'display', name: 'Màn hình 1', width: 1920, height: 1080, primary: true, minimized: false },
        { id: 'window:42', kind: 'window', name: 'Google Chrome', width: 1280, height: 800, primary: false, minimized: false },
        { id: 'window:99', kind: 'window', name: 'Zalo', width: 1040, height: 720, primary: false, minimized: true },
      ],
      listDisplaysCalls: 0,
      starts: 0, stops: 0, previewStarts: 0, previewStops: 0, previewActive: false,
      delay: 0, stopDelay: 0,
      pendingInvitations: [], holdInvitation: false, captureCleanupFails: false,
      sockets: [], pendingDecodes: [], holdDecode: false, decodeFails: false, closedBitmaps: 0,
      published: 0, drawCalls: 0, decodeCalls: 0,
      async start(preview, role, options, relay) {
        if (!preview && fake.previewActive) {
          throw new Error('preview still active during StartBroadcast')
        }
        if (preview) {
          fake.previewStarts++
          fake.previewOptions = options
          fake.previewActive = true
        } else {
          fake.starts++
          fake.captureOptions = options
          fake.relay = relay
        }
        await new Promise(resolve => setTimeout(resolve, fake.delay))
        const key = preview ? 'preview' : 'session'
        const generation = fake[key].generation + 1
        fake[key] = { ...idle(generation), state: preview ? 'Previewing' : role === 'share' ? 'WaitingForViewer' : 'Watching',
          role, captured: 5, captureFPS: 24, encodeFPS: 24, receivedFPS: 24,
          counters: { ...idle(generation).counters, received: 5, decoded: 5, emitted: 0, keys: 1, lastKind: 'KEY' }, uptime: 1 }
        return { generation, endpoint: { url: `ws://127.0.0.1:1/${preview ? 'preview' : 'viewer'}`, token: 'synthetic-only' } }
      },
      async stop(preview) {
        if (preview) {
          fake.previewStops++
        } else {
          fake.stops++
        }
        await new Promise(resolve => setTimeout(resolve, fake.stopDelay))
        const key = preview ? 'preview' : 'session'
        fake[key] = idle(fake[key].generation)
        if (preview) fake.previewActive = false
        if (!preview && fake.captureCleanupFails) {
          fake.session = { ...fake.session, state: 'Error', error: 'Không giải phóng được tài nguyên chụp màn hình.' }
          throw new Error(fake.session.error)
        }
      },
      async invitation() {
        if (fake.holdInvitation) return new Promise(resolve => fake.pendingInvitations.push(resolve))
        return fake.session.state === 'WaitingForViewer' ? 'synthetic-invitation' : ''
      },
      publish() {
        for (const socket of fake.sockets) if (!socket.closed) socket.onmessage?.({ data: fake.jpeg })
        fake.published++
      },
    }
    // Valid, entirely generated image for createImageBitmap and canvas tests.
    const image = document.createElement('canvas')
    image.width = 32; image.height = 18
    const ctx = image.getContext('2d')
    ctx.fillStyle = '#3060a0'; ctx.fillRect(0, 0, 32, 18)
    image.toBlob(blob => { fake.jpeg = blob }, 'image/jpeg')
    const draw = CanvasRenderingContext2D.prototype.drawImage
    CanvasRenderingContext2D.prototype.drawImage = function (...args) { fake.drawCalls++; return draw.apply(this, args) }
    const decode = window.createImageBitmap.bind(window)
    window.createImageBitmap = async blob => {
      fake.decodeCalls++
      if (fake.decodeFails) throw Error('synthetic decode failure')
      const bitmap = await decode(blob)
      const close = bitmap.close.bind(bitmap)
      bitmap.close = () => { fake.closedBitmaps++; close() }
      if (fake.holdDecode) await new Promise(resolve => fake.pendingDecodes.push(resolve))
      return bitmap
    }
    const NativeSocket = window.WebSocket
    window.WebSocket = class {
      constructor(url, protocols) {
        // Vite HMR still uses its real connection.
        if (protocols === 'vite-hmr') return new NativeSocket(url, protocols)
        this.closed = false
        fake.sockets.push(this)
      }
      close() { if (!this.closed) { this.closed = true; this.onclose?.() } }
    }
    setInterval(() => { if (fake.jpeg) fake.publish() }, 40)
  })
  await page.route('**/bindings/**/desktop/service.ts*', route => route.fulfill({
    contentType: 'text/javascript',
    body: `
      const t = window.__desktopTest;
      export const ListDisplays = async () => {
        t.listDisplaysCalls++;
        return structuredClone(t.displays);
      };
      export const GetStats = async () => structuredClone(t.preview);
      export const GetSessionStats = async () => structuredClone(t.session);
      export const GetInvitation = () => t.invitation();
      export const GetDefaultRelay = async () => structuredClone(t.defaultRelay);
      export const StartPreview = options => t.start(true, 'local', options);
      export const StopPreview = () => t.stop(true);
      export const StartBroadcast = (options, relay) => t.start(false, 'share', options, relay);
      export const JoinSession = (_code, relay) => t.start(false, 'watch', undefined, relay);
      export const StopSession = () => t.stop(false);
    `,
  }))
  await page.goto('/')
})

async function enterViewer(page) {
  await page.getByTestId('home-watch').click()
  await page.locator('#relay').fill('127.0.0.1:9009')
  await page.locator('#relay-password').fill('synthetic-password')
  await page.locator('#join-code').fill('synthetic-code')
  await page.getByRole('button', { name: labels.join, exact: true }).click()
  await expect(page.locator('.status')).toHaveText(labels.watching)
  await expect.poll(() => page.evaluate(() => window.__desktopTest.drawCalls)).toBeGreaterThan(0)
}

async function startShare(page) {
  await page.getByTestId('home-share').click()
  await page.locator('#relay').fill('127.0.0.1:9009')
  await page.locator('#relay-password').fill('synthetic-password')
  await page.getByRole('button', { name: labels.start, exact: true }).click()
}

test('a stalled invitation does not hide disconnect or restore a stale code', async ({ page }) => {
  await page.evaluate(() => { window.__desktopTest.holdInvitation = true })
  await startShare(page)
  await expect.poll(() => page.evaluate(() => window.__desktopTest.pendingInvitations.length)).toBe(1)
  await page.evaluate(() => { window.__desktopTest.session.state = 'Disconnected' })
  await expect(page.locator('.status')).toHaveText(labels.disconnected)
  await page.evaluate(() => window.__desktopTest.pendingInvitations.shift()('stale-synthetic-invitation'))
  await expect(page.locator('#invitation')).toHaveCount(0)
  await expect(page.locator('#relay')).toBeEnabled()
})

test('Stop shows a sanitized terminal capture cleanup error', async ({ page }) => {
  await startShare(page)
  await expect(page.locator('#invitation')).toHaveValue('synthetic-invitation')
  await page.evaluate(() => { window.__desktopTest.captureCleanupFails = true })
  await page.getByRole('button', { name: labels.stop, exact: true }).click()
  await expect(page.locator('.status')).toHaveClass(/state-Error/)
  await expect(page.getByRole('alert')).toHaveText('Không giải phóng được tài nguyên chụp màn hình.')
  await expect(page.locator('#relay')).toBeEnabled()
  await expect(page.locator('#invitation')).toHaveCount(0)
})

test('Start/Stop/Home are ordered even when bindings resolve late', async ({ page }) => {
  await page.getByTestId('home-share').click()
  await page.locator('#relay').fill('127.0.0.1:9009')
  await page.locator('#relay-password').fill('synthetic-password')
  await page.evaluate(() => { window.__desktopTest.delay = 300; window.__desktopTest.stopDelay = 400 })
  await page.getByRole('button', { name: labels.start, exact: true }).click()
  const stop = page.getByRole('button', { name: labels.stop, exact: true })
  await stop.click(); await stop.click()
  await page.getByRole('button', { name: '← Dừng phiên và về Home', exact: true }).click()
  await expect(page.getByTestId('home-share')).toBeVisible()
  await expect.poll(() => page.evaluate(() => window.__desktopTest.previewActive)).toBe(false)
  expect(await page.evaluate(() => ({ starts: window.__desktopTest.starts, stops: window.__desktopTest.stops,
    state: window.__desktopTest.session.state, openSockets: window.__desktopTest.sockets.filter(s => !s.closed).length })))
    .toEqual({ starts: 1, stops: 1, state: 'Idle', openSockets: 0 })
})

test('bounded display decode, failure/retry and disconnect preserve media state', async ({ page }) => {
  const pageErrors = []
  page.on('pageerror', error => pageErrors.push(error.message))
  await enterViewer(page)
  await page.evaluate(() => { window.__desktopTest.decodeFails = true })
  await expect(page.getByText('Preview unavailable · Không giải mã được JPEG cục bộ.', { exact: true })).toBeVisible()
  expect(await page.evaluate(() => window.__desktopTest.session.state)).toBe('Watching')
  await page.evaluate(() => { window.__desktopTest.decodeFails = false })
  await expect(page.locator('.error')).toHaveCount(0)
  await page.evaluate(() => window.__desktopTest.sockets.at(-1).close())
  await page.getByRole('button', { name: 'Thử lại hình hiển thị', exact: true }).click()
  await expect(page.locator('.error')).toHaveCount(0)
  expect(await page.evaluate(() => window.__desktopTest.starts)).toBe(1)

  await page.evaluate(() => { window.__desktopTest.holdDecode = true })
  await expect.poll(() => page.evaluate(() => window.__desktopTest.pendingDecodes.length)).toBe(1)
  const before = await page.evaluate(() => window.__desktopTest.decodeCalls)
  const closedBefore = await page.evaluate(() => window.__desktopTest.closedBitmaps)
  await page.evaluate(() => { for (let i = 0; i < 500; i++) window.__desktopTest.publish() })
  expect(await page.evaluate(() => window.__desktopTest.decodeCalls)).toBe(before)
  await page.evaluate(() => { window.__desktopTest.session.state = 'Disconnected' })
  await expect(page.locator('.status')).toHaveText(labels.disconnected)
  await page.evaluate(() => {
    const t = window.__desktopTest
    t.holdDecode = false; t.pendingDecodes.splice(0).forEach(resolve => resolve())
  })
  await expect.poll(() => page.evaluate(() => window.__desktopTest.closedBitmaps)).toBeGreaterThan(closedBefore)
  expect(await page.locator('canvas').evaluate(canvas =>
    canvas.getContext('2d').getImageData(0, 0, 1, 1).data.every(value => value === 0))).toBe(true)
  await expect(page.locator('#join-code')).toBeEnabled()
  expect(pageErrors).toEqual([])
})

test('local preview uses the same cleanup and preserves existing controls', async ({ page }) => {
  await page.getByTestId('home-local').click()
  await expect(page.locator('[data-source-id="synthetic"]')).toHaveAttribute('aria-selected', 'true')
  await page.locator('#resolution').selectOption('640x360')
  await page.locator('#fps').selectOption('15')
  await page.getByRole('button', { name: 'Bắt đầu preview', exact: true }).click()
  await expect(page.locator('.status')).toHaveText('Đang xem trước cục bộ')
  await expect.poll(() => page.evaluate(() => window.__desktopTest.drawCalls)).toBeGreaterThan(0)
  await expect(page.locator('#resolution')).toBeDisabled()
  expect(await page.evaluate(() => window.__desktopTest.previewOptions)).toEqual({
    displayID: 'synthetic', width: 640, height: 360, fps: 15, tau: 4, keyInterval: 30,
  })
  await page.getByRole('button', { name: 'Dừng preview', exact: true }).click()
  await expect(page.locator('.status')).toHaveText('Chưa chụp màn hình')
  await expect(page.locator('#resolution')).toBeEnabled()
  expect(await page.evaluate(() => window.__desktopTest.sockets.every(s => s.closed))).toBe(true)
  await page.getByRole('button', { name: '← Dừng phiên và về Home', exact: true }).click()
  await expect(page.getByTestId('home-local')).toBeVisible()
})

test('Share sends the selected capture and compression settings', async ({ page }) => {
  await page.getByTestId('home-share').click()
  await expect(page.locator('[data-source-id="synthetic"]')).toHaveAttribute('aria-selected', 'true')
  await page.locator('#relay').fill('127.0.0.1:9009')
  await page.locator('#relay-password').fill('synthetic-password')
  await page.locator('#stream-resolution').selectOption('640x360')
  await page.locator('#stream-fps').selectOption('30')
  await page.getByText('Nén ảnh nâng cao', { exact: true }).click()
  await page.locator('#stream-tau').fill('6')
  await page.locator('#stream-keyinterval').fill('60')
  await page.getByRole('button', { name: labels.start, exact: true }).click()
  await expect(page.locator('#invitation')).toHaveValue('synthetic-invitation')
  await expect(page.locator('#stream-resolution')).toBeDisabled()
  expect(await page.evaluate(() => window.__desktopTest.captureOptions)).toEqual({
    displayID: 'synthetic', width: 640, height: 360, fps: 30, tau: 6, keyInterval: 60,
  })
})

test('waiting and incomplete telemetry keep unmeasured video rates hidden', async ({ page }) => {
  await startShare(page)
  await expect(page.locator('#invitation')).toHaveValue('synthetic-invitation')
  await page.getByText('Chi tiết luồng', { exact: true }).click()

  const rate = page.getByLabel(/^Tốc độ video/)
  const capture = page.getByLabel(/^FPS chụp/)
  const uptime = page.getByLabel(/^Thời lượng phiên/)
  await expect(rate.getByText('—', { exact: true })).toBeVisible()
  await expect(capture.getByText('24', { exact: true })).toBeVisible()

  await page.evaluate(() => {
    const stats = window.__desktopTest.session
    stats.state = 'Live'
    stats.route = 'lan'
    stats.counters.emitted = 1
    stats.sentKiB = 12.5
  })
  await expect(rate.getByText('12.5', { exact: true })).toBeVisible()
  await expect(page.getByLabel(/^Đường truyền/).getByText('LAN trực tiếp', { exact: true })).toBeVisible()

  await page.evaluate(() => {
    const stats = window.__desktopTest.session
    stats.measurementsComplete = false
    stats.measurementOverflow = true
  })
  await expect(rate.getByText('—', { exact: true })).toBeVisible()
  await expect(capture.getByText('—', { exact: true })).toBeVisible()
  await expect(uptime.getByText('1', { exact: true })).toBeVisible()
  await expect(page.getByText(/Cửa sổ đo chưa đầy đủ/)).toBeVisible()
})

test('default relay from relay.json pre-fills fields and survives stop', async ({ page }) => {
  await page.evaluate(() => {
    window.__desktopTest.defaultRelay = { address: '198.51.100.10:9009', password: 'synthetic-default-pass' }
  })
  await page.getByTestId('home-share').click()
  await expect(page.locator('#relay')).toHaveValue('198.51.100.10:9009')
  await expect(page.locator('#relay-password')).toHaveValue('synthetic-default-pass')
  await expect(page.getByText(/Đã tự điền từ relay\.json/)).toBeVisible()

  const startBtn = page.getByRole('button', { name: labels.start, exact: true })
  await expect(startBtn).toBeEnabled()
  await startBtn.click()
  await expect(page.locator('#invitation')).toHaveValue('synthetic-invitation')
  expect(await page.evaluate(() => window.__desktopTest.relay)).toEqual({
    address: '198.51.100.10:9009',
    password: 'synthetic-default-pass',
  })

  await page.getByRole('button', { name: labels.stop, exact: true }).click()
  await expect(page.locator('#relay')).toHaveValue('198.51.100.10:9009')
  await expect(page.locator('#relay-password')).toHaveValue('synthetic-default-pass')
  await expect(startBtn).toBeEnabled()
})

test('Share previews selected monitor and app window live before broadcasting', async ({ page }) => {
  await page.evaluate(() => {
    window.__desktopTest.defaultRelay = { address: '198.51.100.10:9009', password: 'synthetic-default-pass' }
  })
  await page.getByTestId('home-share').click()
  await expect(page.getByText('Xem trước · Chưa phát', { exact: true })).toBeVisible()
  await expect.poll(() => page.evaluate(() => window.__desktopTest.previewOptions?.displayID)).toBe('synthetic')

  await page.getByRole('tab', { name: /Cửa sổ/ }).click()
  await page.locator('[data-source-id="window:42"]').click()
  await expect(page.locator('[data-source-id="window:42"]')).toHaveAttribute('aria-selected', 'true')
  await expect.poll(() => page.evaluate(() => window.__desktopTest.previewOptions?.displayID)).toBe('window:42')

  await page.getByRole('button', { name: labels.start, exact: true }).click()
  await expect(page.locator('#invitation')).toHaveValue('synthetic-invitation')
  expect(await page.evaluate(() => ({
    previewActive: window.__desktopTest.previewActive,
    displayID: window.__desktopTest.captureOptions?.displayID,
  }))).toEqual({ previewActive: false, displayID: 'window:42' })
})

test('minimized windows are listed with badge and source list auto-refreshes on tab and focus', async ({ page }) => {
  await page.evaluate(() => {
    window.__desktopTest.defaultRelay = { address: '198.51.100.10:9009', password: 'synthetic-default-pass' }
  })
  await page.getByTestId('home-share').click()
  await expect(page.locator('[data-source-id="synthetic"]')).toHaveAttribute('aria-selected', 'true')
  const callsAfterMount = await page.evaluate(() => window.__desktopTest.listDisplaysCalls)

  // Add a new minimized window before switching to the Windows tab.
  await page.evaluate(() => {
    window.__desktopTest.displays.push({
      id: 'window:105', kind: 'window', name: 'PowerPoint', width: 1440, height: 900, primary: false, minimized: true,
    })
  })
  await page.getByRole('tab', { name: /Cửa sổ/ }).click()
  await expect.poll(() => page.evaluate(() => window.__desktopTest.listDisplaysCalls)).toBeGreaterThan(callsAfterMount)
  const zaloOption = page.locator('[data-source-id="window:99"]')
  await expect(zaloOption).toContainText('Zalo')
  await expect(zaloOption).toContainText('Đang thu nhỏ')
  await expect(page.locator('[data-source-id="window:105"]')).toContainText('PowerPoint')

  await zaloOption.click()
  await expect(zaloOption).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByText(/Cửa sổ đang thu nhỏ dưới thanh tác vụ sẽ tự mở lên/)).toBeVisible()
  await expect.poll(() => page.evaluate(() => window.__desktopTest.previewOptions?.displayID)).toBe('window:99')

  // Focus event refreshes the window list automatically when idle.
  const callsBeforeFocus = await page.evaluate(() => window.__desktopTest.listDisplaysCalls)
  await page.evaluate(() => {
    window.__desktopTest.displays.push({
      id: 'window:108', kind: 'window', name: 'Notepad', width: 800, height: 600, primary: false, minimized: false,
    })
    window.dispatchEvent(new window.Event('focus'))
  })
  await expect.poll(() => page.evaluate(() => window.__desktopTest.listDisplaysCalls)).toBeGreaterThan(callsBeforeFocus)
  await expect(page.locator('[data-source-id="window:108"]')).toContainText('Notepad')

  // Broadcasting a minimized window succeeds and disables focus auto-refresh while locked.
  await page.getByRole('button', { name: labels.start, exact: true }).click()
  await expect(page.locator('#invitation')).toHaveValue('synthetic-invitation')
  expect(await page.evaluate(() => window.__desktopTest.captureOptions?.displayID)).toBe('window:99')
  const callsWhileLocked = await page.evaluate(() => window.__desktopTest.listDisplaysCalls)
  await page.evaluate(() => window.dispatchEvent(new window.Event('focus')))
  await page.waitForTimeout(200)
  expect(await page.evaluate(() => window.__desktopTest.listDisplaysCalls)).toBe(callsWhileLocked)
})
