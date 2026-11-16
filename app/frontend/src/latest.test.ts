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
})
