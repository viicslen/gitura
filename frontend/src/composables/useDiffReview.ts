import { ref, computed } from 'vue'
import * as App from '../wailsjs/go/main/App'
import type { model } from '../wailsjs/go/models'

/**
 * useDiffReview provides reactive state and actions for the diff review view.
 * Manages file list, current file selection, diff loading, pending review state,
 * and other-reviewer thread visibility.
 */
export function useDiffReview() {
  const files = ref<model.PRFileDTO[]>([])
  const filesLoading = ref(false)
  const filesError = ref('')

  const currentFilePath = ref<string | null>(null)
  const currentDiff = ref<model.ParsedDiffDTO | null>(null)
  const diffLoading = ref(false)
  const diffError = ref('')

  const pendingReview = ref<model.PendingReviewDTO | null>(null)

  const showOtherThreads = ref(false)
  const otherThreads = ref<model.CommentThreadDTO[]>([])

  // ── Derived ────────────────────────────────────────────────────────────────

  const currentFile = computed<model.PRFileDTO | null>(
    () => files.value.find((f) => f.filename === currentFilePath.value) ?? null,
  )

  const currentFileIndex = computed<number>(
    () => files.value.findIndex((f) => f.filename === currentFilePath.value),
  )

  const canGoPrev = computed(() => currentFileIndex.value > 0)
  const canGoNext = computed(() => currentFileIndex.value < files.value.length - 1)

  /** Returns threads anchored to the current file. */
  const threadsForFile = computed<model.CommentThreadDTO[]>(() => {
    if (!currentFilePath.value) return []
    return otherThreads.value.filter((t) => t.path === currentFilePath.value)
  })

  // ── Actions ────────────────────────────────────────────────────────────────

  async function loadFiles(): Promise<void> {
    filesLoading.value = true
    filesError.value = ''
    try {
      files.value = await App.GetPRFiles()
      if (files.value.length > 0 && currentFilePath.value === null) {
        await selectFile(files.value[0].filename)
      }
    } catch (err) {
      filesError.value = String(err)
    } finally {
      filesLoading.value = false
    }
  }

  async function selectFile(path: string): Promise<void> {
    currentFilePath.value = path
    diffLoading.value = true
    diffError.value = ''
    currentDiff.value = null
    try {
      currentDiff.value = await App.GetFileDiff(path)
    } catch (err) {
      diffError.value = String(err)
    } finally {
      diffLoading.value = false
    }
  }

  async function nextFile(): Promise<void> {
    if (canGoNext.value) {
      await selectFile(files.value[currentFileIndex.value + 1].filename)
    }
  }

  async function prevFile(): Promise<void> {
    if (canGoPrev.value) {
      await selectFile(files.value[currentFileIndex.value - 1].filename)
    }
  }

  async function addDraftComment(comment: model.DraftCommentDTO): Promise<void> {
    try {
      pendingReview.value = await App.AddDraftComment(comment)
    } catch (err) {
      throw new Error(String(err))
    }
  }

  async function postImmediateComment(comment: model.DraftCommentDTO): Promise<model.CommentDTO> {
    return App.PostImmediateComment(comment)
  }

  async function submitReview(req: model.ReviewSubmitDTO): Promise<model.ReviewSubmitResult> {
    const result = await App.SubmitReview(req)
    pendingReview.value = null
    return result
  }

  async function discardPendingReview(): Promise<void> {
    await App.DiscardPendingReview()
    pendingReview.value = null
  }

  async function loadPendingReview(): Promise<void> {
    try {
      pendingReview.value = await App.GetPendingReview()
    } catch {
      pendingReview.value = null
    }
  }

  async function toggleOtherThreads(): Promise<void> {
    showOtherThreads.value = !showOtherThreads.value
    if (showOtherThreads.value && otherThreads.value.length === 0) {
      try {
        otherThreads.value = await App.GetCommentThreads(false)
      } catch {
        // non-fatal; threads simply won't show
      }
    }
  }

  return {
    // state
    files,
    filesLoading,
    filesError,
    currentFilePath,
    currentFile,
    currentDiff,
    diffLoading,
    diffError,
    pendingReview,
    showOtherThreads,
    otherThreads,
    // derived
    currentFileIndex,
    canGoPrev,
    canGoNext,
    threadsForFile,
    // actions
    loadFiles,
    selectFile,
    nextFile,
    prevFile,
    addDraftComment,
    postImmediateComment,
    submitReview,
    discardPendingReview,
    loadPendingReview,
    toggleOtherThreads,
  }
}
