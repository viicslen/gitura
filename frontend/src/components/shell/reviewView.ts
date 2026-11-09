import { ref } from 'vue'

// Shared so the rail can switch the review view that ReviewPage owns.
export const prView = ref<'overview' | 'conversation' | 'files'>('overview')
