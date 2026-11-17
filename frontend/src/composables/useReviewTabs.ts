import { reactive, ref } from 'vue'

export type ReviewTab = 'overview' | 'conversation' | 'files'

// Shared so the rail can switch the review tab that PRReviewPage owns.
const reviewTab = ref<ReviewTab>('overview')

/** Counts the rail shows on each tab button; 0 hides the badge. */
const tabBadges = reactive<Record<ReviewTab, number>>({ overview: 0, conversation: 0, files: 0 })

/** How Overview lists activity; kept for the session, across PRs. */
const overviewMode = ref<'grouped' | 'timeline'>('grouped')

/** Shared review-tab state: the active tab, the per-tab badge counts and the Overview mode. */
export function useReviewTabs() {
  return { reviewTab, tabBadges, overviewMode }
}
