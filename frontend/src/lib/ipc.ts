interface WailsBridge { Call(method: string, params: unknown): Promise<unknown>; }
interface WailsWindow {
  go?: { main?: { Bridge?: WailsBridge } };
  runtime?: { EventsOn(name: string, cb: (...data: any[]) => void): () => void };
}
const w = (): WailsWindow => window as unknown as WailsWindow;
function hasBridge(): boolean { return typeof window !== 'undefined' && !!w().go?.main?.Bridge; }

export const inWails: boolean = hasBridge();

export function call<T = unknown>(method: string, params?: unknown): Promise<T> {
  if (!hasBridge()) return Promise.reject(new Error('bukan di dalam Wails'));
  return (w().go!.main!.Bridge!.Call(method, params ?? null) as Promise<T>).catch((e) => {
    throw e instanceof Error ? e : new Error(String(e));
  });
}

export function onEvent(topicPrefix: string, fn: (topic: string, payload: unknown) => void): () => void {
  if (typeof window === 'undefined' || !w().runtime?.EventsOn) return () => {};
  return w().runtime!.EventsOn('daemon:event', (ev: { topic: string; payload: unknown }) => {
    if (ev && typeof ev.topic === 'string' && ev.topic.startsWith(topicPrefix)) fn(ev.topic, ev.payload);
  });
}

export function appPid(): Promise<number> {
  const b = w().go?.main?.Bridge as (WailsBridge & { AppPID?: () => Promise<number> }) | undefined;
  if (!b?.AppPID) return Promise.reject(new Error('bukan di dalam Wails'));
  return b.AppPID();
}
