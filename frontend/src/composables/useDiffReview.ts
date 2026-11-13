import { ref, computed } from 'vue'
import { isTestFile } from '@/lib/testFiles'
import * as App from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'
import { errorMessage } from '@/lib/utils'
import { toast } from 'vue-sonner'
import { toggleRef } from '@/composables/useToggles'

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
  /** The file `currentDiff` belongs to; trails `currentFilePath` while the next diff loads. */
  const shownFilePath = ref<string | null>(null)
  const currentDiff = ref<model.ParsedDiffDTO | null>(null)
  const diffLoading = ref(false)
  const diffError = ref('')

  const pendingReview = ref<model.PendingReviewDTO | null>(null)

  const hideViewed = toggleRef('hide_viewed')
  const hideTests = toggleRef('hide_tests')
  /** While test files are hidden, mark them viewed whenever the files load. */
  const autoViewTests = toggleRef('auto_view_tests')

  // ── Derived ────────────────────────────────────────────────────────────────

  const currentFile = computed<model.PRFileDTO | null>(
    () => files.value.find((f) => f.filename === currentFilePath.value) ?? null,
  )
  const shownFile = computed<model.PRFileDTO | null>(
    () => files.value.find((f) => f.filename === shownFilePath.value) ?? null,
  )

  const isHidden = (f: model.PRFileDTO) => (hideViewed.value && f.viewed) || (hideTests.value && isTestFile(f.filename))
  const visibleFiles = computed(() => files.value.filter((f) => !isHidden(f)))

  /** Files left to review; test files count only while shown, and nothing counts while viewed state is unknown. */
  const unviewedCount = computed(() =>
    viewedError.value ? 0 : files.value.filter((f) => !f.viewed && !(hideTests.value && isTestFile(f.filename))).length,
  )

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

  /** An earlier error stays until this load answers, so a retry does not blank the message first. */
  async function loadFiles(): Promise<void> {
    filesLoading.value = true
    try {
      const pending = new Map(files.value.filter((f) => viewedSaving.value.has(f.filename)).map((f) => [f.filename, f.viewed]))
      files.value = (await App.GetPRFiles()).map((f) => {
        const viewed = pending.get(f.filename)
        return viewed === undefined ? f : { ...f, viewed }
      })
      filesError.value = ''
      await loadViewed()
      if (startFile.value && currentFilePath.value === null) {
        await selectFile(startFile.value.filename)
      }
    } catch (err) {
      filesError.value = errorMessage(err)
      // The error screen only shows when there are no files, so a failed refresh needs its own notice.
      if (files.value.length) toast.error('Could not refresh files: ' + filesError.value)
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
      viewedError.value = errorMessage(err)
    }
  }

  let diffRequest = 0

  /**
   * The diff on screen (or the error) stays until the next one answers, so files swap without a blank in between.
   * `keep` refetches the open file's diff without marking it loading, so its view and scroll stay.
   */
  async function selectFile(path: string, keep = false): Promise<void> {
    const request = ++diffRequest
    currentFilePath.value = path
    if (!keep) diffLoading.value = true
    try {
      const diff = await App.GetFileDiff(path)
      if (request !== diffRequest) return
      currentDiff.value = diff
      shownFilePath.value = path
      diffError.value = ''
    } catch (err) {
      if (request === diffRequest) diffError.value = errorMessage(err)
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
    await setViewedMany([path], viewed)
  }

  /** Marks files shown at once; on a failure the files not yet saved roll back and the error is thrown. */
  async function setViewedMany(paths: string[], viewed: boolean): Promise<void> {
    const todo = files.value.filter((f) => paths.includes(f.filename) && f.viewed !== viewed && !viewedSaving.value.has(f.filename))
    if (!todo.length) return
    for (const f of todo) f.viewed = viewed
    const saving = todo.map((f) => f.filename)
    viewedSaving.value = new Set([...viewedSaving.value, ...saving])
    let i = 0
    try {
      // One at a time: the backend's viewed map is not safe for concurrent writes.
      for (; i < saving.length; i++) await App.SetFileViewed(saving[i], viewed)
    } catch (err) {
      // Look the files up again: a refresh meanwhile replaces the file objects.
      const unsaved = new Set(saving.slice(i))
      for (const f of files.value) if (unsaved.has(f.filename)) f.viewed = !viewed
      throw err
    } finally {
      const next = new Set(viewedSaving.value)
      for (const path of saving) next.delete(path)
      viewedSaving.value = next
    }
  }

  async function addDraftComment(comment: model.DraftCommentDTO): Promise<void> {
    try {
      pendingReview.value = await App.AddDraftComment(comment)
    } catch (err) {
      throw new Error(errorMessage(err))
    }
  }

  async function postImmediateComment(comment: model.DraftCommentDTO): Promise<model.ReviewCommentDTO> {
    return App.PostImmediateComment(comment)
  }

  async function submitReview(req: model.ReviewSubmitRequest): Promise<model.ReviewSubmitResult> {
    const result = await App.SubmitReview(req)
    pendingReview.value = null
    return result
  }

  const draftIndex = (draft: model.DraftCommentDTO) => pendingReview.value?.comments?.indexOf(draft) ?? -1

  async function updateDraftComment(draft: model.DraftCommentDTO, base: string, next: string): Promise<void> {
    pendingReview.value = await App.UpdateDraftComment(draftIndex(draft), base, next)
  }

  async function deleteDraftComment(draft: model.DraftCommentDTO): Promise<void> {
    pendingReview.value = await App.DeleteDraftComment(draftIndex(draft), draft.body)
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
    unviewedCount,
    viewedError,
    loadViewed,
    hideViewed,
    hideTests,
    filesLoading,
    filesError,
    currentFilePath,
    currentFile,
    shownFile,
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
    setViewedMany,
    viewedSaving,
    autoViewTests,
    addDraftComment,
    postImmediateComment,
    submitReview,
    discardPendingReview,
    loadPendingReview,
    draftIndex,
    updateDraftComment,
    deleteDraftComment,
  }
}
