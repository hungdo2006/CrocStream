import type { Started } from './api'

interface Status { state: string; generation: number; error: string }
type InvitationRequest = Promise<string> & { cancel?: () => void | PromiseLike<void> }
interface InvitationLookup { request?: InvitationRequest }
interface SessionAPI<T extends Status> {
  stats(): Promise<T>
  stop(): Promise<void>
  invitation?(): InvitationRequest
}
interface Messages { start: string; stop: string; poll: string }
export interface SessionView<T> {
  state: string
  error: string
  invitation: string
  copyStatus: string
  started?: Started
  stats?: T
}
export const inactive = (state: string) => ['Idle', 'Disconnected', 'Error'].includes(state)

// One coordinator per mounted screen. Revision protects RPC completions;
// generation protects backend snapshots. Stop always follows a pending Start.
export class SessionLifecycle<T extends Status> {
  private view: SessionView<T> = { state: 'Idle', error: '', invitation: '', copyStatus: '' }
  private listeners = new Set<() => void>()
  private connected = false
  private connection = 0
  private revision = 0
  private active = false
  private generation?: number
  private starting?: Promise<Started>
  private stopping?: Promise<boolean>
  private timer?: ReturnType<typeof setTimeout>
  private invitationLookup?: InvitationLookup

  constructor(private readonly api: SessionAPI<T>, private readonly messages: Messages) {}

  getSnapshot = () => this.view
  subscribe = (listener: () => void) => {
    this.listeners.add(listener)
    return () => { this.listeners.delete(listener) }
  }
  private update(patch: Partial<SessionView<T>>) {
    this.view = { ...this.view, ...patch }
    this.listeners.forEach(listener => listener())
  }
  setError = (error: string) => { this.update({ error }) }

  connect() {
    this.connected = true
    const connection = ++this.connection
    void this.poll(connection)
  }
  disconnect() {
    this.connected = false
    this.connection++
    clearTimeout(this.timer)
    this.clearInvitationLookup()
    if (this.active || this.starting || this.stopping) void this.stop()
  }
  private current(revision: number, connection = this.connection) {
    return this.connected && revision === this.revision && connection === this.connection
  }
  private async poll(connection: number) {
    const revision = this.revision
    try {
      // Never issue a poll during a command: it can contain the previous state
      // even if its response arrives after that command has completed.
      if (this.starting || this.stopping) return
      const next = await this.api.stats()
      if (!this.current(revision, connection)) return
      if (this.generation !== undefined && next.generation !== this.generation) return
      this.active = !inactive(next.state)
      const mayInvite = next.state === 'WaitingForViewer' || next.state === 'Authenticating'
      if (!mayInvite) this.clearInvitationLookup()
      this.update({
        stats: next, state: next.state,
        error: next.error || (this.view.error === this.messages.poll ||
          (inactive(next.state) && this.view.error === this.messages.stop) ? '' : this.view.error),
        started: this.active ? this.view.started : undefined,
        invitation: mayInvite ? this.view.invitation : '',
        copyStatus: mayInvite ? this.view.copyStatus : '',
      })
      if (mayInvite && this.api.invitation && this.current(revision, connection)) {
        const lookup = this.invitationLookup ??= {}
        if (!lookup.request) void this.pollInvitation(lookup, revision, connection)
      }
    } catch {
      if (this.current(revision, connection)) this.setError(this.messages.poll)
    } finally {
      if (this.connected && connection === this.connection) {
        this.timer = setTimeout(() => void this.poll(connection), 500)
      }
    }
  }

  private clearInvitationLookup() {
    const request = this.invitationLookup?.request
    this.invitationLookup = undefined
    if (request?.cancel) void Promise.resolve(request.cancel()).catch(() => {})
  }

  private async pollInvitation(lookup: InvitationLookup, revision: number, connection: number) {
    const current = () => this.invitationLookup === lookup && this.current(revision, connection)
    try {
      lookup.request = this.api.invitation?.()
      const invitation = await lookup.request
      if (invitation !== undefined && current()) this.update({ invitation })
    } catch {
      if (current()) this.setError(this.messages.poll)
    } finally {
      lookup.request = undefined
    }
  }

  async start(request: () => Promise<Started>, state: string): Promise<boolean> {
    if (!this.connected || this.active || this.starting || this.stopping) return false
    const revision = ++this.revision
    this.clearInvitationLookup()
    this.active = true
    this.update({ state, error: '', invitation: '', copyStatus: '', stats: undefined, started: undefined })
    const starting = Promise.resolve().then(request)
    this.starting = starting
    try {
      const started = await starting
      // Even a canceled UI operation must remember the backend generation so
      // polling can observe eventual cleanup if Stop itself times out.
      this.generation = started.generation
      if (!this.current(revision)) return false
      this.update({ started })
      return true
    } catch {
      if (this.current(revision)) {
        this.active = false
        this.update({ state: 'Error', error: this.messages.start })
      }
      return false
    } finally {
      if (this.starting === starting) this.starting = undefined
      if (revision === this.revision) this.revision++
    }
  }

  stop = (): Promise<boolean> => {
    if (this.stopping) return this.stopping
    this.revision++
    this.clearInvitationLookup()
    this.update({ state: 'Stopping', started: undefined, stats: undefined, invitation: '', copyStatus: '' })
    const starting = this.starting
    const stopping = (async () => {
      try {
        await starting?.catch(() => {})
        await this.api.stop()
        this.active = false
        this.update({ state: 'Idle', error: '' })
        return true
      } catch {
        this.active = true
        this.update({ state: 'Stopping', error: this.messages.stop })
        return false
      } finally {
        this.revision++
        this.stopping = undefined
      }
    })()
    this.stopping = stopping
    return stopping
  }

  async copyInvitation(write: (value: string) => Promise<void>) {
    const revision = this.revision, invitation = this.view.invitation
    const lookup = this.invitationLookup
    if (!invitation) return
    const current = () => this.current(revision) && this.invitationLookup === lookup && this.view.invitation === invitation
    try {
      await write(invitation)
      if (current()) this.update({ copyStatus: 'Đã sao chép mã.' })
    } catch {
      if (current()) this.setError('Không sao chép được mã.')
    }
  }
}
