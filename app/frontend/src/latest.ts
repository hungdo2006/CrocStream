export interface Bitmap { close(): void }

// At most one decode and one newest pending blob. A disposed generation can
// never draw; every successful decode closes the bitmap even if draw throws.
export class LatestFrames<T extends Bitmap> {
  private latest: Blob | undefined
  private busy = false
  private disposed = false
  private scheduled?: () => void
  constructor(
    private readonly decode: (blob: Blob) => Promise<T>,
    private readonly draw: (bitmap: T) => void,
    private readonly failed: () => void,
    private readonly schedule?: (run: () => void) => () => void,
  ) {}

  push(blob: Blob) {
    if (this.disposed) return
    this.latest = blob
    this.start()
  }
  dispose() { this.disposed = true; this.latest = undefined; this.scheduled?.(); this.scheduled = undefined }
  private start() {
    if (this.busy || this.disposed || this.scheduled || !this.latest) return
    if (this.schedule) {
      this.scheduled = this.schedule(() => { this.scheduled = undefined; void this.run() })
    } else void this.run()
  }
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
        if (this.schedule) break // Admit the newest blob on the next paint turn.
      }
    } finally { this.busy = false; this.start() }
  }
}
