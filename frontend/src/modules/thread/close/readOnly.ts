import type { Thread } from '../types';

export function isThreadReadOnly(thread: Thread): boolean {
  return thread.state === 'closed';
}
