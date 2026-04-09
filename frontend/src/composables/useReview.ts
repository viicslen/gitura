import { ref, computed, watch } from 'vue'
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'
import * as App from '../wailsjs/go/main/App'
import type { model } from '../wailsjs/go/models'
import type { ReviewLoadInput } from '@/types/review'

export type ResolveError = { rootId: number; message: string }

export interface LoadProgress {
  loaded: number
  total: number
}

/**
 * useReview provides reactive state and actions for the PR review page.
 * Call loadPR() on mount to fetch PR metadata and comment threads.
 */
export function useReview(prItem: ReviewLoadInput) {
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
    showResolved.value = false
    loadProgress.value = { loaded: 0, total: -1 }
    currentIndex.value = 0

    const progressHandler = (progress: LoadProgress) => {
      loadProgress.value = progress
    }
    EventsOn('pr:load-progress', progressHandler)

    try {
      prSummary.value = await App.LoadPullRequest(prItem.owner, prItem.repo, prItem.number)
      threads.value = await App.GetCommentThreads(showResolved.value)
    } catch (err) {
      error.value = String(err)
    } finally {
      loading.value = false
      EventsOff('pr:load-progress')
    }
  }

  /**
   * toggleShowResolved flips the showResolved flag and re-fetches threads
   * from the in-memory Go cache (no network call).
   */
  async function toggleShowResolved(): Promise<void> {
    showResolved.value = !showResolved.value
    try {
      threads.value = await App.GetCommentThreads(showResolved.value)
    } catch (err) {
      error.value = String(err)
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
    } catch {
      thread.resolved = false
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
    } catch {
      thread.resolved = true
    }
  }

  /**
   * addReplyToThread inserts a newly-created reply into the matching thread
   * so the UI updates immediately without reloading.
   */
  async function addReplyToThread(comment: model.CommentDTO): Promise<void> {
    const thread = threads.value.find((t) => t.root_id === comment.in_reply_to_id)
    if (thread) {
      thread.comments.push(comment)
      return
    }

    // Fallback: if the thread is not currently present in local state,
    // refresh from backend cache to avoid dropping the reply in UI.
    try {
      threads.value = await App.GetCommentThreads(showResolved.value)
    } catch (err) {
      error.value = String(err)
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
    goNext,
    goPrev,
    resolveThread,
    unresolveThread,
    addReplyToThread,
  }
}
