import { reactive, ref } from 'vue'

export type PrView = 'overview' | 'conversation' | 'files'

// Shared so the rail can switch the review view that ReviewPage owns.
export const prView = ref<PrView>('overview')

/** Counts the rail shows on each view's button; 0 hides the badge. */
export const viewBadges = reactive<Record<PrView, number>>({ overview: 0, conversation: 0, files: 0 })
