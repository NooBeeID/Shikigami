import type { Participant, Thread, ThreadMessage } from '../types';
import { RECOVERY_DEAD_NOTICE, RECOVERY_RESPAWN_PREFIX, RECOVERY_FAILED_PREFIX } from '../types';

export type RecoveryKind = 'dead' | 'respawn' | 'failed';

export function recoveryKind(m: ThreadMessage | undefined): RecoveryKind | null {
  if (!m || m.kind !== 'system' || typeof m.text !== 'string') return null;
  if (m.text === RECOVERY_DEAD_NOTICE) return 'dead';
  if (m.text.startsWith(RECOVERY_RESPAWN_PREFIX)) return 'respawn';
  if (m.text.startsWith(RECOVERY_FAILED_PREFIX)) return 'failed';
  return null;
}

export function isRecoveryMessage(m: ThreadMessage | undefined): boolean {
  return recoveryKind(m) !== null;
}

export function deadParticipants(thread: Thread): Participant[] {
  return (thread?.participants ?? []).filter((p) => p.dead === true);
}
