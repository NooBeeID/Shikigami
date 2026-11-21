type Callback = (payload: any) => void;

const listeners = new Map<string, Set<Callback>>();

export function on(name: string, fn: Callback): () => void {
  let set = listeners.get(name);
  if (!set) {
    set = new Set();
    listeners.set(name, set);
  }
  set.add(fn);
  return () => {
    set!.delete(fn);
  };
}

export function emit(name: string, payload?: unknown): void {
  const set = listeners.get(name);
  if (!set) return;
  for (const fn of [...set]) {
    try {
      fn(payload);
    } catch (e) {
      console.error(`appmenu: handler ${name} gagal`, e);
    }
  }
}
