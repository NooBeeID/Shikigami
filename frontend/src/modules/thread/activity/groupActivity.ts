import type { ThreadMessage } from '../types';

export const ACTIVITY_RUNNING_MS = 30_000;

export interface ActivityGroup {
  type: 'activity';
  key: string; // id pesan activity pertama di grup
  from: string;
  count: number;
  items: string[];
  lastTs: string;
  last: boolean; // tidak ada item aliran lain sesudah grup
  finished: boolean; // setelah grup ada pesan kind='message' dari agent yang sama
  messages: ThreadMessage[];
}
export interface PlainFlowItem {
  type: 'message';
  key: string;
  message: ThreadMessage;
}
export type FlowItem = ActivityGroup | PlainFlowItem;

function activityItemsOf(m: ThreadMessage, names?: (id: string) => string): string[] {
  if (m.items && m.items.length > 0) return [...m.items];
  let t = (m.text ?? '').replace(/[\r\n]+/g, ' ').trim();
  if (names) {
    const prefix = names(m.from) + ': ';
    if (t.startsWith(prefix)) t = t.slice(prefix.length);
  }
  return t ? [t] : [];
}

function activityCountOf(m: ThreadMessage, items: string[]): number {
  if (typeof m.count === 'number' && Number.isFinite(m.count) && m.count > 0) return Math.max(1, m.count);
  return Math.max(1, items.length);
}

export function groupActivity(messages: ThreadMessage[], names?: (id: string) => string): FlowItem[] {
  // Dedup: versi terakhir menang, posisi mengikuti kemunculan pertama.
  const order: ThreadMessage[] = [];
  const index = new Map<string, number>();
  for (const m of messages) {
    const i = index.get(m.id);
    if (i === undefined) {
      index.set(m.id, order.length);
      order.push(m);
    } else {
      order[i] = m;
    }
  }

  const flow: FlowItem[] = [];
  for (const m of order) {
    if (m.kind !== 'activity') {
      flow.push({ type: 'message', key: m.id, message: m });
      continue;
    }
    const items = activityItemsOf(m, names);
    const count = activityCountOf(m, items);
    const tail = flow[flow.length - 1];
    if (tail && tail.type === 'activity' && tail.from === m.from) {
      tail.messages.push(m);
      tail.items.push(...items);
      tail.count += count;
      tail.lastTs = m.ts;
    } else {
      flow.push({
        type: 'activity',
        key: m.id,
        from: m.from,
        count,
        items,
        lastTs: m.ts,
        last: false,
        finished: false,
        messages: [m],
      });
    }
  }

  // Lintasan mundur: kumpulkan pengirim pesan diskusi yang muncul sesudah tiap grup.
  const spoke = new Set<string>();
  for (let i = flow.length - 1; i >= 0; i--) {
    const it = flow[i];
    if (it.type === 'message') {
      if (it.message.kind === 'message') spoke.add(it.message.from);
    } else {
      it.last = i === flow.length - 1;
      it.finished = spoke.has(it.from);
    }
  }
  return flow;
}

export function isRunning(
  group: ActivityGroup,
  now: number,
  followedByOwnMessage: boolean,
  threadClosed: boolean,
): boolean {
  if (threadClosed || followedByOwnMessage || group.finished) return false;
  const t = Date.parse(group.lastTs);
  if (Number.isNaN(t)) return false;
  return now - t < ACTIVITY_RUNNING_MS;
}

export function activityRemainingMs(group: ActivityGroup, now: number): number {
  const t = Date.parse(group.lastTs);
  if (Number.isNaN(t)) return 0;
  return Math.max(0, ACTIVITY_RUNNING_MS - (now - t));
}
