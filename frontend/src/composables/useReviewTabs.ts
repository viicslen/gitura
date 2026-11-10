import { reactive, ref } from 'vue'

export type ReviewTab = 'overview' | 'conversation' | 'files'

// Shared so the rail can switch the review tab that PRReviewPage owns.
const reviewTab = ref<ReviewTab>('overview')

/** Counts the rail shows on each tab button; 0 hides the badge. */
const tabBadges = reactive<Record<ReviewTab, number>>({ overview: 0, conversation: 0, files: 0 })

/** Shared review-tab state: the active tab and the per-tab badge counts. */
export function useReviewTabs() {
  return { reviewTab, tabBadges }
}
