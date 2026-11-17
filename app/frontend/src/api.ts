import * as Service from '../bindings/crocstream.local/app/internal/desktop/service'

export interface Display { id: string; width: number; height: number; primary: boolean }
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
  stopSession: (): Promise<void> => Service.StopSession(),
  stats: (): Promise<Stats> => Service.GetStats() as Promise<Stats>,
}
