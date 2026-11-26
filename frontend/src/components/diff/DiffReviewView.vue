<script setup lang="ts">
import { errorMessage } from '@/lib/utils'
import { onMounted, computed, markRaw, nextTick, provide, reactive, ref, watch, watchEffect } from 'vue'
import { ChevronLeft, ChevronRight, Copy, FileCheck2, GitFork, History, Info, Send, SlidersHorizontal, Trash2 } from '@lucide/vue'
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import Reveal from '@/components/motion/Reveal.vue'
import { loaderShown, SWAP, TEXT_SWAP } from '@/components/motion/motion'
import { confetti, reducedMotion, vPointer } from '@/components/motion/wow'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import DiffFileSidebar from './DiffFileSidebar.vue'
import DiffFileView from './DiffFileView.vue'
import ReviewSubmittedNote from './ReviewSubmittedNote.vue'
import VerdictPicker, { type Verdict } from './VerdictPicker.vue'
import SubmoduleReviewChoices, { inFlightReview, keepChoices, submitBlocker, submoduleRequest, type SubmoduleChoice } from './SubmoduleReviewChoices.vue'
import type { DraftActions } from './DiffDraftComment.vue'
import { useDiffReview } from '@/composables/useDiffReview'
import { useViewShortcuts } from '@/composables/useViewShortcuts'
import { useReviewTabs } from '@/composables/useReviewTabs'
import { useKeybindings, keyLabel } from '@/composables/useKeybindings'
import type * as model from '@/bindings/gitura/internal/model'
import { hideWhitespaceChanges, type UnsentComments } from '@/lib/diffLayout'
import { toast } from 'vue-sonner'
import { useLocalStorage, useWindowSize } from '@vueuse/core'
import { toggleRef } from '@/composables/useToggles'
import { GetCommands, RunCommands, SubmoduleDrafts } from '@/bindings/gitura/internal/app/app'
import { defaultCommandFor, loadDefaultCommands } from '@/composables/useCommands'
import { Browser, Clipboard } from '@wailsio/runtime'
import { formatLocalReview } from '@/lib/localReview'
import { isTestFile } from '@/lib/testFiles'
import { fileKey, flattenFileTree, isPointer, keyOf, repoOf } from '@/lib/fileTree'
import { BACKWARDS_NOTICE, DIVERGED_NOTICE, groupedTree, groupSubmodules, prLabel, submoduleNotices } from '@/lib/submodules'

const { tabBadges } = useReviewTabs()

const props = defineProps<{
  threads: model.CommentThreadDTO[]
  showThreads: boolean
  /** GitHub rejects approve/request-changes from the PR author. */
  isOwnPr: boolean
  headSha: string
  /** The Files view is on screen; its [ and ] shortcuts only apply then. */
  active: boolean
  /** Half-written inline comments; owned by the page so a refresh doesn't drop them. */
  unsentComments: UnsentComments
  /** Reviewing a local working tree: comments go to a command or the clipboard instead of GitHub. */
  local?: boolean
}>()

const emit = defineEmits<{
  (e: 'resolve', rootId: number): void
  (e: 'unresolve', rootId: number): void
  (e: 'reply-sent', comment: model.ReviewCommentDTO): void
  /** New threads were created on GitHub; the parent should re-fetch them. */
  (e: 'threads-changed'): void
  (e: 'update:showThreads', value: boolean): void
  (e: 'ran'): void
}>()

const {
  files,
  visibleFiles,
  startFile,
  unviewedCount,
  viewedError,
  loadViewed,
  hideViewed,
  hideTests,
  currentFileIndex,
  filesLoading,
  filesError,
  currentFileKey,
  currentFile,
  shownFile,
  currentDiff,
  diffLoading,
  diffError,
  pendingReview,
  canGoPrev,
  canGoNext,
  loadFiles,
  selectFile,
  nextFile,
  prevFile,
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
} = useDiffReview(props.local)

const allViewed = () => files.value.length > 0 && files.value.every((f) => f.viewed)
const justViewed = ref(new Set<string>())
let justViewedTimer: ReturnType<typeof setTimeout> | undefined

/** A save the user asked for, of files by `fileKey`: the files just marked show it off, and the one that completes the review fires confetti once saved. */
async function saveViewed(keys: string[], viewed: boolean): Promise<void> {
  const wasDone = allViewed()
  const marked = viewed ? files.value.filter((f) => keys.includes(fileKey(f)) && !f.viewed).map(fileKey) : []
  const saving = setViewedMany(keys, viewed)
  if (marked.length && marked.length <= 15) {
    justViewed.value = new Set(marked)
    clearTimeout(justViewedTimer)
    justViewedTimer = setTimeout(() => (justViewed.value = new Set()), 1500)
  }
  await saving
  if (viewed && !wasDone && allViewed()) confetti(document.querySelector('#viewed-toggle:not([inert] *)'))
}

// Marking a file viewed moves on to the next shown file, while the save finishes in the background.
async function toggleViewed(file: model.PRFileDTO, viewed: boolean): Promise<void> {
  const keepFocus = document.activeElement?.id === 'viewed-toggle'
  const saving = saveViewed([fileKey(file)], viewed)
  if (viewed && canGoNext.value) {
    void nextFile().then(async () => {
      if (!keepFocus) return
      await nextTick()
      // The file being left is still fading out, with a checkbox of its own.
      document.querySelector<HTMLElement>('#viewed-toggle:not([inert] *)')?.focus()
    })
  } else if (viewed) {
    const filtered = hideViewed.value || hideTests.value
    toast(startFile.value && !startFile.value.viewed
      ? 'Last file reached; some earlier files are not viewed yet'
      : filtered ? 'All shown files viewed' : 'All files viewed')
  }
  try {
    await saving
  } catch (err) {
    toast.error(`Could not update viewed state for ${file.filename}: ${errorMessage(err)}`)
  }
}

async function markViewed(keys: string[], viewed: boolean, byUser = false): Promise<void> {
  try {
    await (byUser ? saveViewed(keys, viewed) : setViewedMany(keys, viewed))
  } catch (err) {
    toast.error(`Could not update viewed state: ${errorMessage(err)}`)
  }
}

// Runs once per load (not on every viewed change), so a rejected save is not retried in a loop.
watch(() => hideTests.value && autoViewTests.value && !filesLoading.value && !viewedError.value && files.value.length > 0, (ready) => {
  if (ready) void markViewed(files.value.filter((f) => isTestFile(f.filename)).map(fileKey), true)
})

watchEffect(() => {
  tabBadges.files = unviewedCount.value
})

const notices = computed(() => (shownFile.value ? submoduleNotices(shownFile.value, files.value, !!props.local) : null))
const shownRepo = computed(() => (shownFile.value ? repoOf(shownFile.value) : ''))
const viewedLocalTitle = computed(() => {
  if (props.local || !shownFile.value?.viewed_local) return undefined
  const holder = files.value.find((f) => isPointer(f) && f.filename === shownRepo.value)
  if (holder?.submodule_pr) return `Kept in gitura only: this file is outside ${holder.submodule_pr}`
  return holder?.submodule_pr_unknown ? "Kept in gitura only: couldn't check this submodule for a pull request" : 'Kept in gitura only: this submodule has no pull request to mark it on'
})

const submoduleRange = computed(() => {
  const repo = shownRepo.value
  return repo ? files.value.find((f) => isPointer(f) && f.filename === repo)?.submodule_range : undefined
})
// The head and range the shown diff was loaded at: a refresh moves both before the diff it reloads arrives.
const diffPin = ref<{ headSha: string; range?: string }>({ headSha: '' })
watch([currentDiff, () => shownFile.value && fileKey(shownFile.value)], () => {
  diffPin.value = { headSha: props.headSha, range: submoduleRange.value }
}, { immediate: true })
// Review threads are on the loaded PR's own files by path; a submodule file sharing their path has none.
const threadsForFile = computed(() => props.threads.filter((t) => shownFile.value && !shownRepo.value && t.path === shownFile.value.filename))
const currentFilePosition = computed(() => currentFileIndex.value + 1)
const draftKey = (c: model.DraftCommentDTO) => keyOf(c.submodule ?? '', c.path, !!c.pointer)
const draftsForFile = computed(() => {
  const key = shownFile.value && fileKey(shownFile.value)
  return (pendingReview.value?.comments ?? []).filter((c) => draftKey(c) === key)
})
// Threads count on the main-repository rows at their path only, so submodule rows sharing it count none.
const unresolvedCounts = computed(() => {
  const byPath: Record<string, number> = {}
  for (const t of props.threads) {
    if (!t.resolved) byPath[t.path] = (byPath[t.path] ?? 0) + 1
  }
  return Object.fromEntries(files.value.filter((f) => !repoOf(f) && byPath[f.filename]).map((f) => [fileKey(f), byPath[f.filename]]))
})
const SIDEBAR_MIN = 180
const { width: windowWidth } = useWindowSize()
// Keep at least 60% of the window for the diff, so a width saved on a wide screen can't squeeze it.
const SIDEBAR_MAX = computed(() => Math.max(SIDEBAR_MIN, Math.min(640, Math.round(windowWidth.value * 0.4))))
const sidebarWidth = useLocalStorage('gitura.diffSidebarWidth', 256)
const wrapLines = toggleRef('wrap_lines')
const hideWhitespace = toggleRef('hide_whitespace')
const shownDiff = computed(() =>
  hideWhitespace.value && currentDiff.value ? { ...currentDiff.value, hunks: hideWhitespaceChanges(currentDiff.value.hunks ?? []) } : currentDiff.value,
)

const clampWidth = (w: number) => Math.round(Math.min(SIDEBAR_MAX.value, Math.max(SIDEBAR_MIN, w)))
let resizeFrom: { x: number; width: number } | null = null

function startResize(event: PointerEvent): void {
  resizeFrom = { x: event.clientX, width: sidebarWidth.value }
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}

const stopResize = () => (resizeFrom = null)

function resize(event: PointerEvent): void {
  if (resizeFrom) sidebarWidth.value = clampWidth(resizeFrom.width + event.clientX - resizeFrom.x)
}

const pendingCount = computed(() => pendingReview.value?.comments?.length ?? 0)
// An unsettled submit may have left a pending review on GitHub even with no drafts here.
const canDiscard = computed(() => !!pendingReview.value?.has_pending || !!pendingReview.value?.in_flight_review)
const draftsStale = computed(() => {
  const pinned = pendingReview.value?.head_sha
  return !!pinned && !!props.headSha && pinned !== props.headSha
})
// Only drafts on the PR's own files are pinned to its head; a submodule's are placed by their range.
const shownDraftsStale = computed(() => draftsStale.value && !shownRepo.value)

// ── Review submit panel state ──────────────────────────────────────────────
// A dialog is still fading out while the review empties: it lists a snapshot taken when it opens, and resets on the next open.
const dialog = reactive({ comments: [] as model.DraftCommentDTO[], staleSha: '' })
let submitted = false
let discarded = false
function snapshotReview(): void {
  dialog.comments = pendingReview.value?.comments ?? []
  dialog.staleSha = draftsStale.value ? pendingReview.value?.head_sha.slice(0, 7) ?? '' : ''
}
async function openSubmitPanel(): Promise<void> {
  if (submitted) {
    submitted = submitLoading.value = false
    reviewBody.value = ''
    verdict.value = 'COMMENT'
    submoduleChoices.value = {}
  }
  snapshotReview()
  submitPanelOpen.value = true
  if (props.local) return
  // Opened before the first sync of the pending review: its drafts, and the choices they need, are still on the way.
  submoduleDraftsLoading.value = true
  await firstSync
  snapshotReview()
  void loadSubmoduleDrafts()
}
let draftsRequest = 0
/** `retry` looks up again a submodule PR whose lookup failed, without waiting out the backend's backoff. */
async function loadSubmoduleDrafts(retry = false): Promise<void> {
  const request = ++draftsRequest
  submoduleDrafts.value = []
  submoduleDraftsError.value = ''
  submoduleDraftsLoading.value = !!pendingReview.value?.comments?.some((c) => c.submodule)
  if (!submoduleDraftsLoading.value) return
  try {
    const drafts = await SubmoduleDrafts(retry)
    if (request !== draftsRequest) return
    submoduleDrafts.value = drafts
    submoduleChoices.value = keepChoices(drafts, submoduleChoices.value)
    // The backend looked a submodule's PR up again and found otherwise: the file list's routing and Viewed marks are stale.
    const pointers = new Map(files.value.filter(isPointer).map((f) => [f.filename, f]))
    const moved = (d: model.SubmoduleDraftsDTO, p = pointers.get(d.path)) =>
      !!p && ((p.submodule_pr ?? '') !== (d.pr ?? '') || !!p.submodule_pr_unknown !== !!d.pr_unknown)
    if (drafts.some((d) => d.rerouted || moved(d))) void loadFiles()
  } catch (err) {
    if (request === draftsRequest) submoduleDraftsError.value = errorMessage(err)
  } finally {
    if (request === draftsRequest) submoduleDraftsLoading.value = false
  }
}
function openDiscardDialog(): void {
  if (discarded) discarded = discardLoading.value = false
  snapshotReview()
  discardDialogOpen.value = true
}
const submitPanelOpen = ref(false)
const verdict = ref<Verdict>('COMMENT')
const reviewBody = ref('')
const submitLoading = ref(false)
const submoduleDrafts = ref<model.SubmoduleDraftsDTO[]>([])
const submoduleChoices = ref<Record<string, SubmoduleChoice>>({})
const submoduleDraftsLoading = ref(false)
const submoduleDraftsError = ref('')
watch(() => pendingReview.value?.in_flight_review, (r) => {
  const held = inFlightReview(r)
  if (held) [verdict.value, reviewBody.value] = [held.verdict, held.body]
}, { immediate: true })

const submitBlocked = computed(() =>
  submitBlocker(verdict.value, reviewBody.value, pendingReview.value?.comments ?? [], submoduleDrafts.value, submoduleChoices.value),
)
const canSubmit = computed(
  () => !submitLoading.value && !submoduleDraftsLoading.value && !submoduleDraftsError.value && !submitBlocked.value,
)

// ── Discard confirmation dialog ────────────────────────────────────────────
const discardDialogOpen = ref(false)
const discardLoading = ref(false)

// Provide pending-review state to deep descendants (e.g. InlineCommentForm)
provide('hasPendingReview', computed(() => pendingReview.value?.has_pending ?? false))
provide('localReview', props.local)
provide<DraftActions>('draftActions', { indexOf: draftIndex, update: updateDraftComment, remove: deleteDraftComment, ran: () => emit('ran') })

// Holding two diffs at once drops frames once either is large: then the old one goes in one frame and only the new one moves.
const HEAVY_DIFF_LINES = 200
const diffLines = (diff: model.ParsedDiffDTO | null) => (diff?.hunks ?? []).reduce((n, h) => n + h.lines.length, 0)
const heavySwap = ref(false)
watch(currentDiff, (next, previous) => {
  heavySwap.value = Math.max(diffLines(next), diffLines(previous)) > HEAVY_DIFF_LINES
}, { flush: 'sync' })

// A file further down the tree springs in from below, over the open one as it leaves upward; one further up, the other way.
// Transform only: fading a pane-sized diff while it moves halves the frame rate.
const treeOrder = computed(() => flattenFileTree(groupedTree(groupSubmodules(files.value)), new Set<string>()).flatMap((row) => (row.type === 'file' ? [fileKey(row.file)] : [])))
const swapDown = ref(true)
watch(() => shownFile.value && fileKey(shownFile.value), (next, previous) => {
  if (next && previous) swapDown.value = treeOrder.value.indexOf(next) > treeOrder.value.indexOf(previous)
}, { flush: 'sync' })
const fileSwap = computed(() => heavySwap.value
  ? {
      enterActiveClass: 'transition-[translate] duration-show ease-spring',
      enterFromClass: swapDown.value ? 'translate-y-10' : '-translate-y-10',
      leaveActiveClass: 'hidden',
    }
  : {
      enterActiveClass: 'bg-background transition-[translate,scale] duration-show ease-spring',
      enterFromClass: `${swapDown.value ? 'translate-y-24' : '-translate-y-24'} scale-95`,
      leaveActiveClass: 'pointer-events-none absolute inset-0 transition-[translate] duration-base',
      leaveToClass: swapDown.value ? '-translate-y-16' : 'translate-y-16',
    })

// Coming to the Files view wipes the diff in from behind a cover; re-keyed so every arrival replays it.
const wipe = ref(0)
watch(() => props.active && !!shownFile.value, (shown) => {
  if (shown && !reducedMotion()) wipe.value++
}, { immediate: true })

/** Where the paper plane takes off from, in window coordinates. */
const flight = ref<{ x: number; y: number } | null>(null)
function sendOff(from: DOMRect): void {
  if (!reducedMotion()) flight.value = { x: from.x + from.width / 2, y: from.y + from.height / 2 }
}
const changedKeys = computed(() => new Set(files.value.map(fileKey)))

// ── Lifecycle ─────────────────────────────────────────────────────────────
let firstSync: Promise<void> = Promise.resolve()
onMounted(() => {
  firstSync = loadFiles().then(loadPendingReview)
})

// ── Comment handlers ──────────────────────────────────────────────────────
/** `comment` carries the head and file its form was opened on, so the backend refuses it if the code moved since. */
async function postComment(comment: model.DraftCommentDTO, mode: 'draft' | 'immediate'): Promise<boolean> {
  try {
    if (mode === 'draft') {
      await addDraftComment(comment)
      toast('Comment added to review')
    } else {
      const url = (await postImmediateComment(comment)).html_url
      toast(url ? `Comment posted on ${prLabel(url)}` : 'Comment posted', url ? { action: { label: 'Open', onClick: () => void Browser.OpenURL(url) } } : {})
      emit('threads-changed')
      // Checking the submodule PR again can move or dismiss Viewed marks.
      void loadFiles()
    }
    return true
  } catch (err) {
    const message = errorMessage(err)
    const moved = message.includes('conflict:') && !!comment.head_sha && comment.head_sha !== props.headSha
    toast.error(moved ? 'The code changed since this comment was placed. Your text is kept: place the comment again on the current diff.' : message)
    return false
  }
}

// ── Submit review ─────────────────────────────────────────────────────────
async function handleSubmitReview(event: MouseEvent): Promise<void> {
  const from = (event.currentTarget as HTMLElement).getBoundingClientRect()
  submitLoading.value = true
  try {
    const result = await submitReview({
      verdict: verdict.value,
      body: reviewBody.value,
      submodules: submoduleRequest(submoduleDrafts.value, submoduleChoices.value),
    })
    submitted = true
    submitPanelOpen.value = false
    sendOff(from)
    if (verdict.value === 'APPROVE') confetti()
    const extra = !!result.submodule_review_urls?.length || !!result.submodule_comment_urls?.length || !!result.folded_comments
    toast('Review submitted', extra ? { description: markRaw(ReviewSubmittedNote), componentProps: { result }, duration: 10000 } : {})
    emit('threads-changed')
    void loadFiles()
  } catch (err) {
    const before = pendingCount.value
    // Drafts already posted to a submodule PR before the failure leave the pending review; a submodule PR found
    // closed or moved on no longer takes comments or Viewed marks, which the file list shows too.
    await loadPendingReview()
    await Promise.all([loadSubmoduleDrafts(), errorMessage(err).includes('conflict:') && loadFiles()])
    snapshotReview()
    toast.error(errorMessage(err), pendingCount.value < before ? { description: 'Some comments were already posted and are no longer pending.' } : {})
    submitLoading.value = false
  }
}

// ── Local review: hand comments to a command or the clipboard ────────────
const commands = ref<model.CommandDTO[]>([])
const commandName = ref('')
const localMarkdown = computed(() => formatLocalReview(pendingReview.value?.comments ?? []))

async function openLocalPanel(): Promise<void> {
  void openSubmitPanel()
  try {
    commands.value = await GetCommands()
    await loadDefaultCommands()
    const fallback = defaultCommandFor('review')
    if (!commands.value.some((c) => c.name === commandName.value)) {
      commandName.value = commands.value.some((c) => c.name === fallback) ? fallback : commands.value[0]?.name ?? ''
    }
  } catch (err) {
    toast.error('Could not load commands: ' + errorMessage(err))
  }
}

async function copyLocalReview(): Promise<void> {
  try {
    await Clipboard.SetText(localMarkdown.value)
    toast('Comments copied as markdown')
  } catch {
    toast.error('Could not copy to the clipboard')
  }
}

async function sendLocalReview(event: MouseEvent): Promise<void> {
  const from = (event.currentTarget as HTMLElement).getBoundingClientRect()
  submitLoading.value = true
  try {
    await RunCommands([commandName.value], localMarkdown.value, {} as model.RunContext)
    await discardPendingReview()
    submitted = true
    submitPanelOpen.value = false
    sendOff(from)
    emit('ran')
  } catch (err) {
    toast.error(errorMessage(err))
    submitLoading.value = false
  }
}

// ── Discard review ────────────────────────────────────────────────────────
async function handleDiscardReview(): Promise<void> {
  discardLoading.value = true
  try {
    await discardPendingReview()
    discarded = true
    discardDialogOpen.value = false
    toast('Review discarded')
  } catch (err) {
    toast.error(errorMessage(err))
    discardLoading.value = false
  }
}

async function reload(): Promise<void> {
  const selectedKey = currentFileKey.value

  await Promise.all([
    loadFiles(),
    loadPendingReview(),
  ])

  if (selectedKey && changedKeys.value.has(selectedKey)) {
    await selectFile(selectedKey, true)
  } else if (startFile.value) {
    await selectFile(fileKey(startFile.value))
  }

  if (filesError.value) {
    throw new Error(filesError.value)
  }
  if (diffError.value) {
    throw new Error(diffError.value)
  }
}

// Coming back to Files while the open file is already viewed picks up at the first unviewed one;
// a file still being reviewed stays open. Either way focus lands in the diff, so keys scroll it straight away.
watch(() => props.active, async (active) => {
  if (!active) return
  const from = document.activeElement
  const start = startFile.value
  if (currentFile.value?.viewed && start && !start.viewed) await selectFile(fileKey(start))
  await nextTick()
  // Focus moved by the user while the file loaded stays put; body means the view they left hid it.
  const now = document.activeElement
  if (props.active && (now === from || now === document.body)) fileView.value?.focus()
})

// ── Keyboard nav ──────────────────────────────────────────────────────────
const { bindings } = useKeybindings()
const fileView = ref<InstanceType<typeof DiffFileView> | null>(null)
async function jumpChange(dir: 1 | -1): Promise<void> {
  if (fileView.value?.jumpChange(dir)) return
  if (dir > 0) {
    if (canGoNext.value) await nextFile()
    return
  }
  if (!canGoPrev.value) return
  await prevFile()
  await nextTick()
  // Going back lands on the file's last change, so the next p carries on through it.
  if (!diffError.value && shownFile.value && fileKey(shownFile.value) === currentFileKey.value) fileView.value?.jumpChange(1, true)
}
function toggleShownViewed(): void {
  const file = shownFile.value
  if (file && !viewedSaving.value.has(fileKey(file))) void toggleViewed(file, !file.viewed)
}
useViewShortcuts(() => props.active, () => ({
  [bindings.value.nextFile]: nextFile,
  [bindings.value.prevFile]: prevFile,
  [bindings.value.nextChange]: () => jumpChange(1),
  [bindings.value.prevChange]: () => jumpChange(-1),
  [bindings.value.toggleViewed]: toggleShownViewed,
}))

// ── Exposed interface for parent ──────────────────────────────────────────
defineExpose({ canGoPrev, canGoNext, prevFile, nextFile, files, reload })
</script>

<template>
  <div
    class="flex flex-col outline-none"
  >
    <!-- ── Top toolbar ────────────────────────────────────────────────────── -->
    <div class="flex h-10 items-center gap-2 px-3 border-b border-border shrink-0">
      <div class="flex items-center shrink-0">
        <Button variant="ghost" size="sm" class="h-7 gap-1 px-2 text-xs" :disabled="!canGoPrev" aria-label="Previous file" :title="`Previous file (${keyLabel(bindings.prevFile)})`" @click="prevFile">
          <ChevronLeft class="h-4 w-4" /><span class="hidden xl:inline">Previous</span>
        </Button>
        <Button variant="ghost" size="sm" class="h-7 gap-1 px-2 text-xs" :disabled="!canGoNext" aria-label="Next file" :title="`Next file (${keyLabel(bindings.nextFile)})`" @click="nextFile">
          <span class="hidden xl:inline">Next</span><ChevronRight class="h-4 w-4" />
        </Button>
      </div>
      <span class="min-w-0 flex-1 truncate text-xs text-muted-foreground tabular-nums">
        {{ currentFilePosition > 0 ? `File ${currentFilePosition} of ${files.length}` : 'Select a file' }}
        <span class="ml-2 hidden text-muted-foreground/70 lg:inline">{{ keyLabel(bindings.prevFile) }} {{ keyLabel(bindings.nextFile) }} to move, {{ keyLabel(bindings.prevChange) }} {{ keyLabel(bindings.nextChange) }} through changes</span>
      </span>

      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button variant="ghost" size="sm" class="h-7 shrink-0 gap-1.5 px-2 text-xs text-muted-foreground" title="Diff view options">
            <SlidersHorizontal class="h-3.5 w-3.5" />
            View
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-44">
          <DropdownMenuCheckboxItem
            v-if="!local"
            :model-value="showThreads"
            @update:model-value="(v: boolean) => emit('update:showThreads', v)"
            @select.prevent
          >
            Show comments
          </DropdownMenuCheckboxItem>
          <DropdownMenuCheckboxItem v-model="wrapLines" @select.prevent>
            Wrap lines
          </DropdownMenuCheckboxItem>
          <DropdownMenuCheckboxItem v-model="hideWhitespace" @select.prevent>
            Hide whitespace
          </DropdownMenuCheckboxItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Reveal axis="x" :open="pendingCount > 0" class="shrink-0" :class="{ '-ml-2': !pendingCount }">
        <Badge variant="secondary" class="gap-1 text-xs tabular-nums">
          {{ pendingCount }} pending
        </Badge>
      </Reveal>
      <!-- The filled variant has no border; a clear one keeps the width the outline variant has. -->
      <Button
        :variant="pendingCount > 0 ? 'default' : 'outline'"
        size="sm"
        class="h-7 gap-1.5"
        :class="{ 'border border-transparent': pendingCount > 0 }"
        :disabled="local && pendingCount === 0"
        @click="local ? openLocalPanel() : openSubmitPanel()"
      >
        <Send class="h-3.5 w-3.5" />
        <template v-if="local">Send comments</template>
        <!-- Both labels share one cell, so the button keeps its width when they swap. -->
        <span v-else class="grid text-center">
          <span
            v-for="(label, pending) in ['Review changes', 'Finish review']"
            :key="label"
            class="[grid-area:1/1] transition-[opacity,visibility] duration-fast"
            :class="{ 'invisible opacity-0': (pendingCount > 0) !== !!pending }"
          >{{ label }}</span>
        </span>
      </Button>
      <Reveal axis="x" :open="canDiscard" class="shrink-0" :class="{ '-ml-2': !canDiscard }">
        <!-- The ring is inset because the reveal clips at the button's edge. -->
        <Button
          variant="ghost"
          size="icon"
          class="text-destructive hover:text-destructive focus-visible:ring-inset"
          aria-label="Discard pending review"
          title="Discard pending review"
          @click="openDiscardDialog"
        >
          <Trash2 class="h-4 w-4" />
        </Button>
      </Reveal>
    </div>

    <div class="relative flex min-h-0 flex-1 flex-col">
      <Transition v-bind="SWAP">
        <!-- ── Error (files load) ─────────────────────────────────────────────── -->
        <!-- Only when there is nothing to show: a failed refresh keeps the files already loaded. -->
        <div
          v-if="filesError && !files.length"
          class="flex-1 flex flex-col items-center justify-center gap-3 p-6"
        >
          <!-- Stays up while the retry runs, and dims once the wait is real. A different message cross-fades in the same cell. -->
          <div class="grid w-full transition-opacity" :class="{ 'opacity-50 delay-200': filesLoading }">
            <Transition v-bind="TEXT_SWAP">
              <p :key="filesError" class="text-sm text-destructive text-center [grid-area:1/1]">{{ filesError }}</p>
            </Transition>
          </div>
          <Button variant="outline" size="sm" @click="loadFiles">Retry</Button>
        </div>

        <!-- ── Main split ─────────────────────────────────────────────────────── -->
        <div v-else class="flex flex-1 min-h-0 overflow-hidden">
          <!-- Sidebar -->
          <div class="shrink-0 overflow-hidden" :style="{ width: `${clampWidth(sidebarWidth)}px` }">
            <DiffFileSidebar
              data-vim-pane
              v-model:hide-viewed="hideViewed"
              v-model:hide-tests="hideTests"
              v-model:auto-view-tests="autoViewTests"
              :files="files"
              :visible-files="visibleFiles"
              :viewed-error="viewedError"
              :local="local"
              @retry-viewed="loadViewed"
              :comment-counts="unresolvedCounts"
              :selected-key="currentFileKey"
              :active="active"
              :loading="filesLoading && !files.length"
              :just-viewed="justViewed"
              @select="selectFile"
              @set-viewed="(paths, viewed) => markViewed(paths, viewed, true)"
            />
          </div>
          <div
            role="separator"
            aria-orientation="vertical"
            aria-label="Resize file list"
            :aria-valuenow="clampWidth(sidebarWidth)"
            :aria-valuemin="SIDEBAR_MIN"
            :aria-valuemax="SIDEBAR_MAX"
            tabindex="0"
            class="-ml-1 w-1 shrink-0 cursor-col-resize touch-none outline-none hover:bg-primary/40 focus-visible:bg-primary/40"
            @pointerdown="startResize"
            @pointermove="resize"
            @pointerup="stopResize"
            @pointercancel="stopResize"
            @keydown.left.prevent="sidebarWidth = clampWidth(sidebarWidth - 16)"
            @keydown.right.prevent="sidebarWidth = clampWidth(sidebarWidth + 16)"
          />

          <!-- Diff panel: what it shows stays until the next file's diff is here, then the two trade places. -->
          <div class="relative flex-1 flex flex-col min-h-0 overflow-hidden">
            <Transition v-bind="fileSwap" @before-leave="(el) => el.setAttribute('inert', '')">
              <!-- Diff error -->
              <div
                v-if="diffError"
                class="flex-1 flex flex-col items-center justify-center gap-3 p-6"
              >
                <div class="grid w-full">
                  <Transition v-bind="TEXT_SWAP">
                    <p :key="diffError" class="text-sm text-destructive text-center [grid-area:1/1]">{{ diffError }}</p>
                  </Transition>
                </div>
                <Button
                  v-if="currentFileKey"
                  variant="outline"
                  size="sm"
                  @click="selectFile(currentFileKey)"
                >
                  Retry
                </Button>
              </div>

              <!-- No file selected; blank while the first load runs, so the hint is not painted before the data -->
              <div
                v-else-if="!shownFile"
                v-pointer
                class="relative flex flex-1 items-center justify-center overflow-hidden text-sm text-muted-foreground"
              >
                <template v-if="!filesLoading && !diffLoading">
                  <template v-if="files.length">Select a file to view its diff</template>
                  <template v-else>
                    <div class="wow-tilt wow-focus-in relative flex max-w-sm flex-col items-center gap-2 rounded-xl border border-border bg-card/80 px-10 py-8 text-center shadow-lg">
                      <FileCheck2 class="mb-1 size-8 text-wow-2 wow-pop [--i:4]" aria-hidden="true" />
                      <p class="text-base font-semibold text-foreground">{{ local ? 'Nothing to review' : 'No changed files' }}</p>
                      <p>{{ local ? 'This scope has no changes. Edit a file or pick another scope.' : 'This pull request does not change any files.' }}</p>
                    </div>
                  </template>
                </template>
              </div>

              <!-- Diff content -->
              <DiffFileView
                data-vim-pane
                v-else
                ref="fileView"
                :key="fileKey(shownFile)"
                :inert="diffLoading || undefined"
                :file="shownFile"
                :diff="shownDiff"
                :commentable="true"
                :threads="threadsForFile"
                :drafts="draftsForFile"
                :drafts-stale="shownDraftsStale"
                :show-threads="showThreads"
                :wrap-lines="wrapLines"
                :unsent-comments="unsentComments"
                :head-sha="diffPin.headSha"
                :submodule-range="diffPin.range"
                class="flex-1 min-h-0 overflow-hidden"
                :post-comment="postComment"
                @resolve="emit('resolve', $event)"
                @unresolve="emit('unresolve', $event)"
                @reply-sent="emit('reply-sent', $event)"
                @ran="emit('ran')"
              >
                <template v-if="notices?.backwards || notices?.diverged || notices?.routing" #notices>
                  <div class="flex shrink-0 flex-col gap-1 border-b border-border bg-muted/40 px-4 py-1.5 text-xs text-muted-foreground">
                    <p v-if="notices.backwards" class="flex items-center gap-1.5 text-amber-700 dark:text-amber-300">
                      <History class="h-3.5 w-3.5 shrink-0" />{{ BACKWARDS_NOTICE }}
                    </p>
                    <p v-if="notices.diverged" class="flex items-center gap-1.5 text-amber-700 dark:text-amber-300">
                      <GitFork class="h-3.5 w-3.5 shrink-0" />{{ DIVERGED_NOTICE }}
                    </p>
                    <p v-if="notices.routing" class="flex items-center gap-1.5">
                      <Info class="h-3.5 w-3.5 shrink-0" />
                      <span>{{ notices.routing.text }}
                        <a v-if="notices.routing.pr" :href="notices.routing.url" class="text-foreground underline underline-offset-2">{{ notices.routing.pr }}</a>{{ notices.routing.after }}</span>
                    </p>
                  </div>
                </template>
                <template #header-actions>
                  <label
                    class="flex shrink-0 cursor-pointer select-none items-center gap-1.5 border-l border-border pl-3 text-xs text-muted-foreground"
                    :title="`${shownFile.viewed ? 'Mark as not viewed' : 'Mark as viewed and go to the next file'} (${keyLabel(bindings.toggleViewed)})`"
                  >
                    <span class="relative flex rounded-sm" :class="{ 'wow-ring': justViewed.has(fileKey(shownFile)) }">
                      <Checkbox
                        id="viewed-toggle"
                        :model-value="shownFile.viewed"
                        :disabled="viewedSaving.has(fileKey(shownFile))"
                        @update:model-value="(v) => shownFile && toggleViewed(shownFile, v === true)"
                      />
                    </span>
                    <span :title="viewedLocalTitle">{{ viewedLocalTitle ? 'Viewed (gitura only)' : 'Viewed' }}</span>
                  </label>
                </template>
              </DiffFileView>
            </Transition>
            <div v-if="wipe" :key="wipe" aria-hidden="true" class="diff-wipe pointer-events-none absolute inset-0 z-20 bg-background" @animationend="wipe = 0" />
            <div
              class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-background text-sm text-muted-foreground transition-[opacity,visibility]"
              :class="loaderShown(diffLoading)"
            >
              Loading diff…
            </div>
          </div>
        </div>
      </Transition>
    </div>

    <Teleport to="body">
      <div
        v-if="flight"
        aria-hidden="true"
        class="send-off pointer-events-none fixed z-100 text-wow-2"
        :style="{ left: `${flight.x}px`, top: `${flight.y}px` }"
        @animationend="flight = null"
      >
        <Send class="size-8" />
      </div>
    </Teleport>

    <!-- ── Local review panel ─────────────────────────────────────────────── -->
    <Dialog v-if="local" :open="submitPanelOpen" @update:open="submitPanelOpen = $event">
      <DialogContent class="max-w-md max-h-[calc(100vh-2rem)] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Send comments</DialogTitle>
        </DialogHeader>
        <div v-if="dialog.comments.length" class="flex min-w-0 flex-col gap-1">
          <p class="text-xs font-medium text-muted-foreground">
            {{ dialog.comments.length }} pending comment{{ dialog.comments.length === 1 ? '' : 's' }}
          </p>
          <ul class="max-h-40 overflow-y-auto rounded-md border border-border text-xs">
            <li
              v-for="(c, i) in dialog.comments"
              :key="i"
              class="flex flex-col gap-0.5 border-b border-border px-2 py-1.5 last:border-b-0"
            >
              <span class="truncate font-mono text-muted-foreground">
                {{ c.path }}<template v-if="c.line">:{{ c.start_line ? `${c.start_line}–` : '' }}{{ c.line }}</template>
                <template v-if="c.submodule"> · submodule {{ c.submodule }}</template>
                <span v-if="!changedKeys.has(draftKey(c))" class="text-amber-700 dark:text-amber-300">· file no longer changed</span>
              </span>
              <span class="whitespace-pre-wrap break-words">{{ c.body }}</span>
            </li>
          </ul>
        </div>

        <div v-if="commands.length" class="flex items-center gap-2 text-sm">
          <span id="local-review-command" class="text-muted-foreground">Command</span>
          <Select v-model="commandName">
            <SelectTrigger aria-labelledby="local-review-command" class="h-8 flex-1">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="c in commands" :key="c.name" :value="c.name">{{ c.name }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <p v-else class="text-xs text-muted-foreground">Add a command in Settings to send comments straight to a tool such as a coding agent.</p>

        <DialogFooter>
          <Button variant="ghost" @click="submitPanelOpen = false">Cancel</Button>
          <Button variant="outline" class="gap-1.5" @click="copyLocalReview">
            <Copy class="h-3.5 w-3.5" /> Copy as markdown
          </Button>
          <Button v-if="commands.length" :disabled="submitLoading || !commandName" @click="sendLocalReview">
            {{ submitLoading ? 'Sending…' : `Send to ${commandName}` }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- ── Submit review panel ────────────────────────────────────────────── -->
    <Dialog v-else :open="submitPanelOpen" @update:open="submitPanelOpen = $event">
      <DialogContent class="max-w-md max-h-[calc(100vh-2rem)] grid-rows-[auto_minmax(0,1fr)_auto] gap-0 p-0">
        <DialogHeader class="p-6 pb-4">
          <DialogTitle>Submit review</DialogTitle>
        </DialogHeader>
        <div class="flex min-h-0 flex-col gap-4 overflow-y-auto px-6 pb-4">
          <VerdictPicker v-model="verdict" name="verdict" label="Review verdict" :own-pr="isOwnPr" :disabled="!!pendingReview?.in_flight_review" class="mt-2" />

          <Textarea
            v-model="reviewBody"
            :disabled="!!pendingReview?.in_flight_review"
            placeholder="Leave a comment (optional)…"
            rows="4"
            class="text-sm resize-none"
            aria-label="Review body"
          />
          <p v-if="pendingReview?.in_flight_review" class="-mt-2 text-xs text-muted-foreground">An earlier submit may have reached GitHub; submit again to settle it.</p>

          <SubmoduleReviewChoices v-model:choices="submoduleChoices" :drafts="submoduleDrafts" @retry="loadSubmoduleDrafts(true)" />
          <p v-if="submoduleDraftsLoading" class="text-xs text-muted-foreground">Checking the comments on submodule files…</p>
          <div v-else-if="submoduleDraftsError" role="alert" class="flex items-center gap-2 text-xs text-destructive">
            <span class="min-w-0 flex-1">Couldn't check where the comments on submodule files go: {{ submoduleDraftsError }}</span>
            <Button variant="outline" size="sm" class="h-7 shrink-0" @click="loadSubmoduleDrafts(true)">Retry</Button>
          </div>

          <div v-if="dialog.comments.length" class="flex min-w-0 flex-col gap-1">
            <p class="text-xs font-medium text-muted-foreground">
              {{ dialog.comments.length }} pending comment{{ dialog.comments.length === 1 ? '' : 's' }}
              <span v-if="dialog.staleSha"> · will be posted on earlier commit {{ dialog.staleSha }}</span>
            </p>
            <ul class="rounded-md border border-border text-xs">
              <li
                v-for="(c, i) in dialog.comments"
                :key="i"
                class="flex flex-col gap-0.5 border-b border-border px-2 py-1.5 last:border-b-0"
              >
                <span class="truncate font-mono text-muted-foreground">
                  {{ c.path }}<template v-if="c.line">:{{ c.start_line ? `${c.start_line}–` : '' }}{{ c.line }}</template>
                  <template v-if="c.pointer"> (submodule pointer)</template>
                  <template v-if="c.submodule"> · submodule {{ c.submodule }}</template>
                  <span v-if="!changedKeys.has(draftKey(c))" class="text-amber-700 dark:text-amber-300">· file no longer in this PR</span>
                </span>
                <span class="whitespace-pre-wrap break-words">{{ c.body }}</span>
              </li>
            </ul>
          </div>
        </div>

        <DialogFooter class="border-t border-border px-6 py-4">
          <p v-if="submitBlocked && !submoduleDraftsLoading && !submoduleDraftsError" class="min-w-0 flex-1 self-center text-xs text-muted-foreground">{{ submitBlocked }}</p>
          <Button variant="ghost" @click="submitPanelOpen = false">Cancel</Button>
          <Button :disabled="!canSubmit" @click="handleSubmitReview">
            {{ submitLoading ? 'Submitting…' : 'Submit review' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- ── Discard confirmation ───────────────────────────────────────────── -->
    <Dialog :open="discardDialogOpen" @update:open="discardDialogOpen = $event">
      <DialogContent class="max-w-sm">
        <DialogHeader>
          <DialogTitle>Discard pending review?</DialogTitle>
        </DialogHeader>
        <p class="text-sm text-muted-foreground">
          <template v-if="dialog.comments.length">
            This will delete all {{ dialog.comments.length }} pending
            comment{{ dialog.comments.length !== 1 ? 's' : '' }} and cannot be undone.
          </template>
          <template v-else>This will delete what an unsettled earlier submit left pending on GitHub and cannot be undone.</template>
        </p>
        <DialogFooter>
          <Button variant="ghost" @click="discardDialogOpen = false">Keep review</Button>
          <Button variant="destructive" :disabled="discardLoading" @click="handleDiscardReview">
            {{ discardLoading ? 'Discarding…' : 'Discard review' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
/* The cover slides off to the right behind a bar of light, so the diff under it never moves or repaints. */
.diff-wipe {
  animation: diff-wipe var(--transition-duration-show) var(--ease-out) both;
}
.diff-wipe::before {
  content: "";
  position: absolute;
  inset-block: 0;
  left: -3px;
  width: 3px;
  background: linear-gradient(var(--color-wow-1), var(--color-wow-2), var(--color-wow-3));
  box-shadow: 0 0 2rem 0.5rem color-mix(in oklab, var(--color-wow-2) 60%, transparent);
}
@keyframes diff-wipe {
  to { translate: 100% 0; }
}

.send-off {
  filter: drop-shadow(0 0 0.75rem var(--color-wow-2));
  animation: send-off 900ms cubic-bezier(0.5, 0, 0.8, 0.5) both;
}
@keyframes send-off {
  0% { opacity: 0; scale: 0.4; translate: -50% -50%; }
  20% { opacity: 1; scale: 1.8; translate: calc(-50% - 1rem) calc(-50% + 1rem); }
  100% { opacity: 0; scale: 0.6; translate: calc(-50% + 45vw) calc(-50% - 60vh); }
}
</style>
