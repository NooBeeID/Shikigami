import './zoom.css';
import { readable, type Readable } from 'svelte/store';
import { onMenuEvent, getAppConfig, setAppConfig } from '../appmenu';

const MIN = 50;
const MAX = 300;
const STEP = 10;
const DEFAULT = 100;
const PERSIST_DELAY_MS = 300;

let percent = DEFAULT;
let setStore: ((v: number) => void) | null = null;

export const zoomPercent: Readable<number> = readable(DEFAULT, (set) => {
  setStore = set;
  set(percent);
  return () => {
    setStore = null;
  };
});

export function getZoomPercent(): number {
  return percent;
}

function clamp(n: number): number {
  if (!Number.isFinite(n)) return DEFAULT;
  return Math.min(MAX, Math.max(MIN, Math.round(n)));
}

function applyZoom(next: number): void {
  percent = clamp(next);
  setStore?.(percent);
  document.documentElement.style.setProperty('--ui-scale', String(percent / 100));
  import('../terminal')
    .then((m) => (m as any).setTerminalFontScale?.(percent))
    .catch(() => {});
}

let persistTimer: ReturnType<typeof setTimeout> | undefined;
function schedulePersist(): void {
  if (persistTimer !== undefined) clearTimeout(persistTimer);
  persistTimer = setTimeout(() => {
    persistTimer = undefined;
    void setAppConfig({ zoom: percent }).catch(() => {});
  }, PERSIST_DELAY_MS);
}

function step(action: 'in' | 'out' | 'reset'): void {
  const next = clamp(action === 'in' ? percent + STEP : action === 'out' ? percent - STEP : DEFAULT);
  if (next === percent) return;
  applyZoom(next);
  schedulePersist();
}

onMenuEvent('menu:zoom', (p) => step(p.action));

void getAppConfig()
  .then((cfg) => {
    if (persistTimer === undefined && percent === DEFAULT) applyZoom(cfg.zoom);
  })
  .catch(() => {});
