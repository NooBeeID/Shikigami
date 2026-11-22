import { registerThreadSlot } from '../slots';
import CloseButton from './CloseButton.svelte';
import CloseBanner from './CloseBanner.svelte';
import ClosedNotice from './ClosedNotice.svelte';

export { default as ReadyBadge } from './ReadyBadge.svelte';
export { isThreadReadOnly } from './readOnly';

let stopper: (() => void) | null = null;

// Idempoten: panggilan kedua mengembalikan pembersih yang sama. Registrasi slot tidak punya
// API unregister, jadi pembersih hanya mereset flag.
export function installClose(): () => void {
  if (stopper) return stopper;
  registerThreadSlot({ slot: 'header-actions', component: CloseButton, order: 100, id: 'close-button' });
  registerThreadSlot({ slot: 'above-input', component: CloseBanner, order: -10, id: 'close-banner' });
  registerThreadSlot({ slot: 'above-input', component: ClosedNotice, order: 0, id: 'close-notice' });
  const stop = () => {
    if (stopper === stop) stopper = null;
  };
  stopper = stop;
  return stop;
}
