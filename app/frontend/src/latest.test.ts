import { describe, expect, it, vi } from 'vitest'
import { LatestFrames } from './latest'

describe('latest-only canvas lifecycle', () => {
  it('discards queued old images and closes every decoded bitmap', async () => {
    const pending: ((value: { close: () => void }) => void)[] = []
    const decode = vi.fn(() => new Promise<{ close: () => void }>(resolve => pending.push(resolve)))
    const draw = vi.fn(), failed = vi.fn()
    const frames = new LatestFrames(decode, draw, failed)
    const first = new Blob(['first']), stale = new Blob(['stale']), last = new Blob(['last'])
    frames.push(first); frames.push(stale); frames.push(last)
    expect(decode).toHaveBeenCalledTimes(1)
    const a = { close: vi.fn() }; pending.shift()!(a)
    await Promise.resolve()
    expect(a.close).toHaveBeenCalledOnce()
    expect(decode).toHaveBeenLastCalledWith(last)
    frames.dispose()
    const b = { close: vi.fn() }; pending.shift()!(b)
    await Promise.resolve()
    expect(draw).toHaveBeenCalledTimes(1)
    expect(b.close).toHaveBeenCalledOnce()
    expect(failed).not.toHaveBeenCalled()
  })
  it('handles decode/draw failures without leaking bitmaps', async () => {
    const failed = vi.fn(), bitmap = { close: vi.fn() }
    const decode = vi.fn().mockRejectedValueOnce(new Error('bad JPEG')).mockResolvedValue(bitmap)
    const frames = new LatestFrames(decode, () => { throw new Error('canvas') }, failed)
    frames.push(new Blob()); await Promise.resolve(); await Promise.resolve()
    frames.push(new Blob()); await Promise.resolve(); await Promise.resolve()
    expect(failed).toHaveBeenCalledTimes(2)
    expect(bitmap.close).toHaveBeenCalledOnce()
    frames.dispose()
    frames.push(new Blob())
    expect(decode).toHaveBeenCalledTimes(2)
  })
  it('does not let an old session decode overwrite a rejoined canvas', async () => {
    let complete!: (bitmap: { close: () => void }) => void
    const drawn: string[] = []
    const stale = { close: vi.fn() }, fresh = { close: vi.fn() }
    const failed = vi.fn()
    const old = new LatestFrames(() => new Promise<{ close: () => void }>(r => { complete = r }), () => drawn.push('old'), failed)
    old.push(new Blob())
    old.dispose()
    const next = new LatestFrames(async () => fresh, () => drawn.push('new'), failed)
    next.push(new Blob())
    await Promise.resolve()
    complete(stale)
    await Promise.resolve()
    expect(drawn).toEqual(['new'])
    expect(stale.close).toHaveBeenCalledOnce()
    expect(fresh.close).toHaveBeenCalledOnce()
    expect(failed).not.toHaveBeenCalled()
    next.dispose()
  })
  it('continues after a failed image and suppresses errors after leaving', async () => {
    let reject!: () => void
    const failed = vi.fn(), draw = vi.fn(), bitmap = { close: vi.fn() }
    const decode = vi.fn().mockRejectedValueOnce(new Error('bad'))
      .mockResolvedValueOnce(bitmap)
      .mockImplementationOnce(() => new Promise((_, no) => { reject = () => no(new Error('late')) }))
    const frames = new LatestFrames(decode, draw, failed)
    frames.push(new Blob()); await Promise.resolve(); await Promise.resolve()
    frames.push(new Blob()); await Promise.resolve(); await Promise.resolve()
    expect(draw).toHaveBeenCalledOnce()
    expect(bitmap.close).toHaveBeenCalledOnce()
    frames.push(new Blob()); frames.dispose(); reject()
    await Promise.resolve(); await Promise.resolve()
    expect(failed).toHaveBeenCalledOnce()
  })
})
