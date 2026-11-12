import { ref, computed, watch } from 'vue'
import { Events } from '@wailsio/runtime'
import * as App from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'
import type { PRRef } from '@/types/pr'
import { toast } from 'vue-sonner'
import { errorMessage } from '@/lib/utils'

export interface LoadProgress {
  loaded: number
  total: number
}

/**
 * useCommentThreads provides reactive state and actions for a PR's comment threads.
 * Call loadPR() on mount to fetch PR metadata and comment threads.
 */
export function useCommentThreads(prItem: PRRef) {
  const threads = ref<model.CommentThreadDTO[]>([])
  const prSummary = ref<model.PullRequestSummary | null>(null)
  const loading = ref(false)
  const error = ref('')
  const showResolved = ref(false)
  const loadProgress = ref<LoadProgress>({ loaded: 0, total: -1 })
  const currentIndex = ref(0)

  /** Active navigation queue — resolved threads excluded when showResolved is false. */
  const queue = computed(() =>
    showResolved.value ? threads.value : threads.value.filter((t) => !t.resolved)
  )

  const currentThread = computed<model.CommentThreadDTO | null>(
    () => queue.value[currentIndex.value] ?? null
  )
  const isAtEnd = computed(() => currentIndex.value >= queue.value.length - 1)
  const canGoForward = computed(() => !isAtEnd.value && queue.value.length > 0)
  const canGoBack = computed(() => currentIndex.value > 0)

  function goNext(): void {
    if (canGoForward.value) currentIndex.value++
  }
  function goPrev(): void {
    if (canGoBack.value) currentIndex.value--
  }

  /** Clamp currentIndex when queue shrinks (e.g. showResolved toggled off). */
  watch(queue, (q) => {
    if (currentIndex.value >= q.length) {
      currentIndex.value = Math.max(0, q.length - 1)
    }
  })

  async function loadPR(): Promise<void> {
    loading.value = true
    error.value = ''
    loadProgress.value = { loaded: 0, total: -1 }
    // A refresh keeps the filter and the open thread.
    const refreshing = !!prSummary.value
    const openRootId = currentThread.value?.root_id
    if (!refreshing) {
      showResolved.value = false
      currentIndex.value = 0
    }

    const progressHandler = (progress: LoadProgress) => {
      loadProgress.value = progress
    }
    const unsubscribeProgress = Events.On('pr:load-progress', (ev) => progressHandler(ev.data))

    try {
      prSummary.value = await App.LoadPR(prItem.owner, prItem.repo, prItem.number)
      threads.value = await App.GetCommentThreads(true)
      const index = refreshing ? queue.value.findIndex((t) => t.root_id === openRootId) : -1
      if (index >= 0) currentIndex.value = index
    } catch (err) {
      error.value = errorMessage(err)
    } finally {
      loading.value = false
      unsubscribeProgress()
    }
  }

  function toggleShowResolved(): void {
    showResolved.value = !showResolved.value
  }

  /** Re-fetches threads and reviews from GitHub so newly posted comments and reviews show up. */
  async function reloadReviewActivity(): Promise<void> {
    try {
      threads.value = await App.ReloadReviewActivity()
    } catch (err) {
      toast.error('Could not refresh comments: ' + errorMessage(err))
    }
  }

  /**
   * resolveThread optimistically marks a thread resolved and calls the backend.
   * Rolls back on error.
   */
  async function resolveThread(rootId: number): Promise<void> {
    const thread = threads.value.find((t) => t.root_id === rootId)
    if (!thread) return
    thread.resolved = true
    try {
      await App.ResolveThread(rootId)
    } catch (err) {
      thread.resolved = false
      toast.error('Could not resolve conversation: ' + errorMessage(err))
    }
  }

  /**
   * unresolveThread optimistically marks a thread unresolved and calls the backend.
   * Rolls back on error.
   */
  async function unresolveThread(rootId: number): Promise<void> {
    const thread = threads.value.find((t) => t.root_id === rootId)
    if (!thread) return
    thread.resolved = false
    try {
      await App.UnresolveThread(rootId)
    } catch (err) {
      thread.resolved = true
      toast.error('Could not unresolve conversation: ' + errorMessage(err))
    }
  }

  /**
   * addReplyToThread inserts a newly-created reply into the matching thread
   * so the UI updates immediately without reloading.
   */
  async function addReplyToThread(comment: model.ReviewCommentDTO): Promise<void> {
    const thread = threads.value.find((t) => t.root_id === comment.in_reply_to_id)
    if (thread) {
      thread.comments.push(comment)
      return
    }

    // Fallback: if the thread is not currently present in local state,
    // refresh from backend cache to avoid dropping the reply in UI.
    try {
      threads.value = await App.GetCommentThreads(true)
    } catch (err) {
      error.value = errorMessage(err)
    }
  }

  return {
    threads,
    prSummary,
    loading,
    error,
    showResolved,
    loadProgress,
    currentIndex,
    queue,
    currentThread,
    isAtEnd,
    canGoForward,
    canGoBack,
    loadPR,
    toggleShowResolved,
    reloadReviewActivity,
    goNext,
    goPrev,
    resolveThread,
    unresolveThread,
    addReplyToThread,
  }
}
