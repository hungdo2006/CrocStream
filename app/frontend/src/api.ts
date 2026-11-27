import * as Service from '../bindings/crocstream.local/app/internal/desktop/service'
import type { Stats as SessionStats } from '../bindings/crocstream.local/app/internal/session/models'

export type StreamStats = SessionStats
export interface Relay { address: string; password: string }

export interface Display {
  id: string
  kind?: string
  name?: string
  width: number
  height: number
  primary: boolean
  minimized?: boolean
}
export interface Options { displayID: string; width: number; height: number; fps: number; tau: number; keyInterval: number }
export interface Started { endpoint: { url: string; token: string }; generation: number }
export type State = 'Idle' | 'Preparing' | 'Previewing' | 'Stopping' | 'Error'
export interface Stats {
  state: State; error: string; generation: number; captured: number; encoded: number;
  dropped: number; sent: number; bridgeDropped: number; captureFPS: number; encodeFPS: number;
  captureP50Ms: number; captureP95Ms: number; encodeP50Ms: number; encodeP95Ms: number;
  samplesComplete: boolean;
}

export const api = {
  displays: async (): Promise<Display[]> => (await Service.ListDisplays()) ?? [],
  start: (options: Options): Promise<Started> => Service.StartPreview(options),
  stop: (): Promise<void> => Service.StopPreview(),
  stats: (): Promise<Stats> => Service.GetStats() as Promise<Stats>,
}

// Bindings carry commands, small state snapshots and the trusted local endpoint.
// JPEG bytes are received directly over the session's binary WebSocket.
export const streamAPI = {
  stats: (): Promise<StreamStats> => Service.GetSessionStats(),
  stop: (): Promise<void> => Service.StopSession(),
  invitation: () => Service.GetInvitation(),
  defaultRelay: (): Promise<Relay> => Service.GetDefaultRelay(),
  host: (options: Options, relay: Relay): Promise<Started> => Service.StartBroadcast(options, relay),
  join: (code: string, relay: Relay): Promise<Started> => Service.JoinSession(code, relay),
}
