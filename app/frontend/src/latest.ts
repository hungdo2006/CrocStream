export interface Bitmap { close(): void }

// At most one decode and one newest pending blob. A disposed generation can
// never draw; every successful decode closes the bitmap even if draw throws.
export class LatestFrames<T extends Bitmap> {
  private latest: Blob | undefined
  private busy = false
  private disposed = false
  constructor(
    private readonly decode: (blob: Blob) => Promise<T>,
    private readonly draw: (bitmap: T) => void,
    private readonly failed: () => void,
  ) {}

  push(blob: Blob) {
    if (this.disposed) return
    this.latest = blob
    if (!this.busy) void this.run()
  }
  dispose() { this.disposed = true; this.latest = undefined }
  private async run() {
    this.busy = true
    try {
      while (this.latest && !this.disposed) {
        const blob = this.latest
        this.latest = undefined
        let bitmap: T | undefined
        try {
          bitmap = await this.decode(blob)
          if (!this.disposed) this.draw(bitmap)
        } catch {
          if (!this.disposed) this.failed()
        } finally { bitmap?.close() }
      }
    } finally { this.busy = false }
  }
}
