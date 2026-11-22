import { registerRightTab } from '../shell';
import { registerPaneKind } from '../layout/registry';
import ReviewPane from './ReviewPane.svelte';
import ReviewTab from './ReviewTab.svelte';
import ReviewBadge from './ReviewBadge.svelte';
import { startReviewSync } from './store';

registerRightTab({ id: 'review', title: 'Review', order: 15, component: ReviewTab });

registerPaneKind({
  kind: 'review',
  component: ReviewPane,
  titleFor: (tab) => tab.title || 'Review',
});

// Idempoten; modul ini dimuat sekali oleh glob di main.ts.
startReviewSync();

export { ReviewTab, ReviewBadge };
export { reviewFiles, reviewCounts, refreshReview } from './store';
export type { ReviewFile, ReviewStatus, ReviewFilesResult } from './types';
