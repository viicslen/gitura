import { ref, computed } from 'vue'
import { useLocalStorage } from '@vueuse/core'
import { isTestFile } from '@/lib/testFiles'
import * as App from '../wailsjs/go/main/App'
import type { model } from '../wailsjs/go/models'

/**
 * useDiffReview provides reactive state and actions for the diff review view.
 * Manages file list, current file selection, diff loading and pending review state.
 */
export function useDiffReview() {
  const files = ref<model.PRFileDTO[]>([])
  const filesLoading = ref(false)
  const filesError = ref('')
  /** Set when GitHub's viewed state could not be read; every file then shows as not viewed. */
  const viewedError = ref('')

  const currentFilePath = ref<string | null>(null)
  const currentDiff = ref<model.ParsedDiffDTO | null>(null)
  const diffLoading = ref(false)
  const diffError = ref('')

  const pendingReview = ref<model.PendingReviewDTO | null>(null)

  const hideViewed = useLocalStorage('gitura.diffHideViewed', false)
  const hideTests = useLocalStorage('gitura.diffHideTests', false)

  // ── Derived ────────────────────────────────────────────────────────────────

  const currentFile = computed<model.PRFileDTO | null>(
    () => files.value.find((f) => f.filename === currentFilePath.value) ?? null,
  )

  const isHidden = (f: model.PRFileDTO) => (hideViewed.value && f.viewed) || (hideTests.value && isTestFile(f.filename))
  const visibleFiles = computed(() => files.value.filter((f) => !isHidden(f)))

  /** Where reviewing picks up: the first shown file not yet viewed, else the first shown file. */
  const startFile = computed(() => visibleFiles.value.find((f) => !f.viewed) ?? visibleFiles.value[0] ?? files.value[0] ?? null)

  /** Position in the full PR file list, so it stays put when filters hide files around it. */
  const currentFileIndex = computed<number>(
    () => files.value.findIndex((f) => f.filename === currentFilePath.value),
  )

  // Moves skip hidden files but start from the open file even when it is hidden itself
  // (e.g. just marked viewed while viewed files are hidden).
  const nextVisible = computed(() => files.value.find((f, i) => i > currentFileIndex.value && !isHidden(f)) ?? null)
  const prevVisible = computed(() => {
    for (let i = currentFileIndex.value - 1; i >= 0; i--) {
      if (!isHidden(files.value[i])) return files.value[i]
    }
    return null
  })

  const canGoPrev = computed(() => prevVisible.value !== null)
  const canGoNext = computed(() => nextVisible.value !== null)

  // ── Actions ────────────────────────────────────────────────────────────────

  async function loadFiles(): Promise<void> {
    filesLoading.value = true
    filesError.value = ''
    try {
      const pending = new Map(files.value.filter((f) => viewedSaving.value.has(f.filename)).map((f) => [f.filename, f.viewed]))
      files.value = (await App.GetPRFiles()).map((f) => (pending.has(f.filename) ? { ...f, viewed: pending.get(f.filename)! } : f))
      await loadViewed()
      if (startFile.value && currentFilePath.value === null) {
        await selectFile(startFile.value.filename)
      }
    } catch (err) {
      filesError.value = String(err)
    } finally {
      filesLoading.value = false
    }
  }

  async function loadViewed(): Promise<void> {
    try {
      const viewed = new Set(await App.GetViewedFiles())
      // A save still in flight is newer than what GitHub just reported.
      for (const f of files.value) if (!viewedSaving.value.has(f.filename)) f.viewed = viewed.has(f.filename)
      viewedError.value = ''
    } catch (err) {
      viewedError.value = String(err)
    }
  }

  let diffRequest = 0

  /** `keep` refetches the open file's diff without clearing it, so its view and scroll stay. */
  async function selectFile(path: string, keep = false): Promise<void> {
    const request = ++diffRequest
    currentFilePath.value = path
    diffError.value = ''
    if (!keep) {
      diffLoading.value = true
      currentDiff.value = null
    }
    try {
      const diff = await App.GetFileDiff(path)
      if (request === diffRequest) currentDiff.value = diff
    } catch (err) {
      if (request === diffRequest) diffError.value = String(err)
    } finally {
      if (request === diffRequest) diffLoading.value = false
    }
  }

  async function nextFile(): Promise<void> {
    if (nextVisible.value) await selectFile(nextVisible.value.filename)
  }

  async function prevFile(): Promise<void> {
    if (prevVisible.value) await selectFile(prevVisible.value.filename)
  }

  /** Paths whose viewed state is being saved; their checkbox stays locked until GitHub answers. */
  const viewedSaving = ref(new Set<string>())

  async function setViewed(path: string, viewed: boolean): Promise<void> {
    const file = files.value.find((f) => f.filename === path)
    if (!file || viewedSaving.value.has(path)) return
    file.viewed = viewed
    viewedSaving.value = new Set(viewedSaving.value).add(path)
    try {
      await App.SetFileViewed(path, viewed)
    } catch (err) {
      // Look the file up again: a refresh meanwhile replaces the file objects.
      const current = files.value.find((f) => f.filename === path)
      if (current) current.viewed = !viewed
      throw err
    } finally {
      const next = new Set(viewedSaving.value)
      next.delete(path)
      viewedSaving.value = next
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
      pendingReview.value = await App.SyncPendingReview()
    } catch {
      pendingReview.value = null
    }
  }

  return {
    // state
    files,
    visibleFiles,
    startFile,
    viewedError,
    loadViewed,
    hideViewed,
    hideTests,
    filesLoading,
    filesError,
    currentFilePath,
    currentFile,
    currentDiff,
    diffLoading,
    diffError,
    pendingReview,
    // derived
    currentFileIndex,
    canGoPrev,
    canGoNext,
    // actions
    loadFiles,
    selectFile,
    nextFile,
    prevFile,
    setViewed,
    viewedSaving,
    addDraftComment,
    postImmediateComment,
    submitReview,
    discardPendingReview,
    loadPendingReview,
  }
}
