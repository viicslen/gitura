<script setup lang="ts">
import { onMounted, computed, nextTick, provide, ref, watch, watchEffect } from 'vue'
import { ChevronLeft, ChevronRight, Copy, Send, SlidersHorizontal, Trash2 } from 'lucide-vue-next'
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
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
import type { DraftActions } from './DiffDraftComment.vue'
import { useDiffReview } from '@/composables/useDiffReview'
import { useViewShortcuts } from '@/composables/useViewShortcuts'
import { useReviewTabs } from '@/composables/useReviewTabs'
import { useKeybindings, keyLabel } from '@/composables/useKeybindings'
import type { model } from '@/wailsjs/go/models'
import type { UnsentComments } from '@/lib/diffLayout'
import { toast } from 'vue-sonner'
import { useLocalStorage, useWindowSize } from '@vueuse/core'
import { GetCommands, GetDefaultCommandName, RunCommands } from '@/wailsjs/go/app/App'
import { ClipboardSetText } from '@/wailsjs/runtime/runtime'
import { formatLocalReview } from '@/lib/localReview'
import { isTestFile } from '@/lib/testFiles'

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
  currentFilePath,
  currentFile,
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
} = useDiffReview()

// Marking a file viewed moves on to the next shown file, while the save finishes in the background.
async function toggleViewed(path: string, viewed: boolean): Promise<void> {
  const keepFocus = document.activeElement?.id === 'viewed-toggle'
  const saving = setViewed(path, viewed)
  if (viewed && canGoNext.value) {
    void nextFile().then(async () => {
      if (!keepFocus) return
      await nextTick()
      document.getElementById('viewed-toggle')?.focus()
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
    toast.error(`Could not update viewed state for ${path}: ${String(err)}`)
  }
}

async function markViewed(paths: string[], viewed: boolean): Promise<void> {
  try {
    await setViewedMany(paths, viewed)
  } catch (err) {
    toast.error(`Could not update viewed state: ${String(err)}`)
  }
}

// Runs once per load (not on every viewed change), so a rejected save is not retried in a loop.
watch(() => hideTests.value && autoViewTests.value && !filesLoading.value && !viewedError.value && files.value.length > 0, (ready) => {
  if (ready) void markViewed(files.value.filter((f) => isTestFile(f.filename)).map((f) => f.filename), true)
})

watchEffect(() => {
  tabBadges.files = unviewedCount.value
})

const threadsForFile = computed(() => props.threads.filter((t) => t.path === currentFilePath.value))
const currentFilePosition = computed(() => currentFileIndex.value + 1)
const draftsForFile = computed(() =>
  (pendingReview.value?.comments ?? []).filter((c) => c.path === currentFilePath.value),
)
const unresolvedCounts = computed(() => {
  const counts: Record<string, number> = {}
  for (const t of props.threads) {
    if (!t.resolved) counts[t.path] = (counts[t.path] ?? 0) + 1
  }
  return counts
})
const SIDEBAR_MIN = 180
const { width: windowWidth } = useWindowSize()
// Keep at least 60% of the window for the diff, so a width saved on a wide screen can't squeeze it.
const SIDEBAR_MAX = computed(() => Math.max(SIDEBAR_MIN, Math.min(640, Math.round(windowWidth.value * 0.4))))
const sidebarWidth = useLocalStorage('gitura.diffSidebarWidth', 256)
const wrapLines = useLocalStorage('gitura.diffWrapLines', true)

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
const draftsStale = computed(() => {
  const pinned = pendingReview.value?.head_sha
  return !!pinned && !!props.headSha && pinned !== props.headSha
})

// ── Review submit panel state ──────────────────────────────────────────────
const submitPanelOpen = ref(false)
const verdict = ref<'APPROVE' | 'REQUEST_CHANGES' | 'COMMENT'>('COMMENT')
const reviewBody = ref('')
const submitLoading = ref(false)

const VERDICT_OPTIONS: { value: 'APPROVE' | 'REQUEST_CHANGES' | 'COMMENT'; label: string; desc: string; authorAllowed: boolean }[] = [
  { value: 'COMMENT', label: 'Comment', desc: 'Submit general feedback without explicit approval.', authorAllowed: true },
  { value: 'APPROVE', label: 'Approve', desc: 'Submit feedback and approve merging these changes.', authorAllowed: false },
  { value: 'REQUEST_CHANGES', label: 'Request changes', desc: 'Submit feedback that must be addressed before merging.', authorAllowed: false },
]

// GitHub rejects COMMENT and REQUEST_CHANGES reviews that have neither a body nor comments.
const canSubmit = computed(
  () => !submitLoading.value && (verdict.value === 'APPROVE' || !!reviewBody.value.trim() || pendingCount.value > 0),
)

// ── Discard confirmation dialog ────────────────────────────────────────────
const discardDialogOpen = ref(false)
const discardLoading = ref(false)

// Provide pending-review state to deep descendants (e.g. InlineCommentForm)
provide('hasPendingReview', computed(() => pendingReview.value?.has_pending ?? false))
provide('draftsStale', draftsStale)
provide('localReview', props.local)
provide<DraftActions>('draftActions', { indexOf: draftIndex, update: updateDraftComment, remove: deleteDraftComment })
const changedPaths = computed(() => new Set(files.value.map((f) => f.filename)))

// ── Lifecycle ─────────────────────────────────────────────────────────────
onMounted(async () => {
  await loadFiles()
  await loadPendingReview()
})

// ── Comment handlers ──────────────────────────────────────────────────────
async function postComment(comment: model.DraftCommentDTO, mode: 'draft' | 'immediate'): Promise<boolean> {
  try {
    if (mode === 'draft') {
      await addDraftComment(comment)
      toast('Comment added to review')
    } else {
      await postImmediateComment(comment)
      toast('Comment posted')
      emit('threads-changed')
    }
    return true
  } catch (err) {
    toast.error(String(err))
    return false
  }
}

// ── Submit review ─────────────────────────────────────────────────────────
async function handleSubmitReview(): Promise<void> {
  submitLoading.value = true
  try {
    await submitReview({ verdict: verdict.value, body: reviewBody.value })
    submitPanelOpen.value = false
    reviewBody.value = ''
    verdict.value = 'COMMENT'
    toast('Review submitted')
    emit('threads-changed')
  } catch (err) {
    toast.error(String(err))
  } finally {
    submitLoading.value = false
  }
}

// ── Local review: hand comments to a command or the clipboard ────────────
const commands = ref<model.CommandDTO[]>([])
const commandName = ref('')
const localMarkdown = computed(() => formatLocalReview(pendingReview.value?.comments ?? []))

async function openLocalPanel(): Promise<void> {
  submitPanelOpen.value = true
  try {
    commands.value = await GetCommands()
    const fallback = await GetDefaultCommandName()
    if (!commands.value.some((c) => c.name === commandName.value)) {
      commandName.value = commands.value.some((c) => c.name === fallback) ? fallback : commands.value[0]?.name ?? ''
    }
  } catch (err) {
    toast.error('Could not load commands: ' + String(err))
  }
}

async function copyLocalReview(): Promise<void> {
  if (await ClipboardSetText(localMarkdown.value)) toast('Comments copied as markdown')
  else toast.error('Could not copy to the clipboard')
}

async function sendLocalReview(): Promise<void> {
  submitLoading.value = true
  try {
    await RunCommands([commandName.value], localMarkdown.value, {} as model.RunContext)
    await discardPendingReview()
    submitPanelOpen.value = false
    emit('ran')
  } catch (err) {
    toast.error(String(err))
  } finally {
    submitLoading.value = false
  }
}

// ── Discard review ────────────────────────────────────────────────────────
async function handleDiscardReview(): Promise<void> {
  discardLoading.value = true
  try {
    await discardPendingReview()
    discardDialogOpen.value = false
    toast('Review discarded')
  } catch (err) {
    toast.error(String(err))
  } finally {
    discardLoading.value = false
  }
}

async function reload(): Promise<void> {
  const selectedPath = currentFilePath.value

  await Promise.all([
    loadFiles(),
    loadPendingReview(),
  ])

  if (selectedPath && files.value.some((file) => file.filename === selectedPath)) {
    await selectFile(selectedPath, true)
  } else if (startFile.value) {
    await selectFile(startFile.value.filename)
  }

  if (filesError.value) {
    throw new Error(filesError.value)
  }
  if (diffError.value) {
    throw new Error(diffError.value)
  }
}

// Coming back to Files while the open file is already viewed picks up at the first unviewed one;
// a file still being reviewed stays open.
watch(() => props.active, (active) => {
  const start = startFile.value
  if (active && currentFile.value?.viewed && start && !start.viewed) void selectFile(start.filename)
})

// ── Keyboard nav ──────────────────────────────────────────────────────────
const { bindings } = useKeybindings()
useViewShortcuts(() => props.active, () => ({ [bindings.value.nextFile]: nextFile, [bindings.value.prevFile]: prevFile }))

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
        <span class="ml-2 hidden text-muted-foreground/70 lg:inline">{{ keyLabel(bindings.prevFile) }} {{ keyLabel(bindings.nextFile) }} to move</span>
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
        </DropdownMenuContent>
      </DropdownMenu>

      <Badge v-if="pendingCount > 0" variant="secondary" class="gap-1 text-xs">
        {{ pendingCount }} pending
      </Badge>
      <Button
        :variant="pendingCount > 0 ? 'default' : 'outline'"
        size="sm"
        class="h-7 gap-1.5"
        :disabled="local && pendingCount === 0"
        @click="local ? openLocalPanel() : (submitPanelOpen = true)"
      >
        <Send class="h-3.5 w-3.5" />
        {{ local ? 'Send comments' : pendingCount > 0 ? 'Finish review' : 'Review changes' }}
      </Button>
      <template v-if="pendingReview?.has_pending">
        <Button
          variant="ghost"
          size="icon"
          class="text-destructive hover:text-destructive"
          aria-label="Discard pending review"
          title="Discard pending review"
          @click="discardDialogOpen = true"
        >
          <Trash2 class="h-4 w-4" />
        </Button>
      </template>
    </div>

    <!-- ── Error (files load) ─────────────────────────────────────────────── -->
    <div
      v-if="filesError"
      class="flex-1 flex flex-col items-center justify-center gap-3 p-6"
    >
      <p class="text-sm text-destructive text-center">{{ filesError }}</p>
      <Button variant="outline" size="sm" @click="loadFiles">Retry</Button>
    </div>

    <!-- ── Main split ─────────────────────────────────────────────────────── -->
    <div v-else class="flex flex-1 min-h-0 overflow-hidden">
      <!-- Sidebar -->
      <div class="shrink-0 overflow-hidden" :style="{ width: `${clampWidth(sidebarWidth)}px` }">
        <DiffFileSidebar
          v-model:hide-viewed="hideViewed"
          v-model:hide-tests="hideTests"
          v-model:auto-view-tests="autoViewTests"
          :files="files"
          :visible-files="visibleFiles"
          :viewed-error="viewedError"
          @retry-viewed="loadViewed"
          :comment-counts="unresolvedCounts"
          :selected-path="currentFilePath"
          :active="active"
          :loading="filesLoading && !files.length"
          @select="selectFile"
          @set-viewed="markViewed"
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

      <!-- Diff panel -->
      <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
        <!-- Diff loading -->
        <div
          v-if="diffLoading"
          class="flex-1 flex items-center justify-center text-sm text-muted-foreground"
        >
          Loading diff…
        </div>

        <!-- Diff error -->
        <div
          v-else-if="diffError"
          class="flex-1 flex flex-col items-center justify-center gap-3 p-6"
        >
          <p class="text-sm text-destructive text-center">{{ diffError }}</p>
          <Button
            v-if="currentFilePath"
            variant="outline"
            size="sm"
            @click="selectFile(currentFilePath)"
          >
            Retry
          </Button>
        </div>

        <!-- No file selected -->
        <div
          v-else-if="!currentFile"
          class="flex-1 flex items-center justify-center text-sm text-muted-foreground"
        >
          Select a file to view its diff
        </div>

        <!-- Diff content -->
        <DiffFileView
          v-else
          :file="currentFile"
          :diff="currentDiff"
          :commentable="true"
          :threads="threadsForFile"
          :drafts="draftsForFile"
          :drafts-stale="draftsStale"
          :show-threads="showThreads"
          :wrap-lines="wrapLines"
          :unsent-comments="unsentComments"
          class="flex-1 min-h-0 overflow-hidden"
          :post-comment="postComment"
          @resolve="emit('resolve', $event)"
          @unresolve="emit('unresolve', $event)"
          @reply-sent="emit('reply-sent', $event)"
        >
          <template #header-actions>
            <label
              class="flex shrink-0 cursor-pointer select-none items-center gap-1.5 border-l border-border pl-3 text-xs text-muted-foreground"
              :title="currentFile.viewed ? 'Mark as not viewed' : 'Mark as viewed and go to the next file'"
            >
              <Checkbox
                id="viewed-toggle"
                :model-value="currentFile.viewed"
                :disabled="viewedSaving.has(currentFile.filename)"
                @update:model-value="(v) => currentFile && toggleViewed(currentFile.filename, v === true)"
              />
              Viewed
            </label>
          </template>
        </DiffFileView>
      </div>
    </div>

    <!-- ── Local review panel ─────────────────────────────────────────────── -->
    <Dialog v-if="local" :open="submitPanelOpen" @update:open="submitPanelOpen = $event">
      <DialogContent class="max-w-md max-h-[calc(100vh-2rem)] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Send comments</DialogTitle>
        </DialogHeader>
        <div v-if="pendingCount > 0" class="flex min-w-0 flex-col gap-1">
          <p class="text-xs font-medium text-muted-foreground">
            {{ pendingCount }} pending comment{{ pendingCount === 1 ? '' : 's' }}
          </p>
          <ul class="max-h-40 overflow-y-auto rounded-md border border-border text-xs">
            <li
              v-for="(c, i) in pendingReview?.comments ?? []"
              :key="i"
              class="flex flex-col gap-0.5 border-b border-border px-2 py-1.5 last:border-b-0"
            >
              <span class="truncate font-mono text-muted-foreground">
                {{ c.path }}:{{ c.start_line ? `${c.start_line}–` : '' }}{{ c.line }}
                <span v-if="!changedPaths.has(c.path)" class="text-amber-700 dark:text-amber-300">· file no longer changed</span>
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
      <DialogContent class="max-w-md max-h-[calc(100vh-2rem)] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Submit review</DialogTitle>
        </DialogHeader>

        <!-- Verdict selection -->
        <div class="flex flex-col gap-2 my-2" role="radiogroup" aria-label="Review verdict">
          <label
            v-for="option in VERDICT_OPTIONS"
            :key="option.value"
            class="flex items-start gap-2.5 rounded-md border border-border px-3 py-2 transition-colors"
            :class="[
              verdict === option.value ? 'border-primary bg-accent' : '',
              isOwnPr && !option.authorAllowed ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer hover:bg-accent',
            ]"
          >
            <input
              type="radio"
              name="verdict"
              :value="option.value"
              :checked="verdict === option.value"
              :disabled="isOwnPr && !option.authorAllowed"
              class="mt-0.5"
              @change="verdict = option.value"
            />
            <div>
              <div class="text-sm font-medium">{{ option.label }}</div>
              <div class="text-xs text-muted-foreground">
                {{ isOwnPr && !option.authorAllowed ? 'Not available on your own pull request.' : option.desc }}
              </div>
            </div>
          </label>
        </div>

        <div v-if="pendingCount > 0" class="flex min-w-0 flex-col gap-1">
          <p class="text-xs font-medium text-muted-foreground">
            {{ pendingCount }} pending comment{{ pendingCount === 1 ? '' : 's' }}
            <span v-if="draftsStale"> · will be posted on earlier commit {{ pendingReview?.head_sha.slice(0, 7) }}</span>
          </p>
          <ul class="max-h-40 overflow-y-auto rounded-md border border-border text-xs">
            <li
              v-for="(c, i) in pendingReview?.comments ?? []"
              :key="i"
              class="flex flex-col gap-0.5 border-b border-border px-2 py-1.5 last:border-b-0"
            >
              <span class="truncate font-mono text-muted-foreground">
                {{ c.path }}:{{ c.start_line ? `${c.start_line}–` : '' }}{{ c.line }}
                <span v-if="!changedPaths.has(c.path)" class="text-amber-700 dark:text-amber-300">· file no longer in this PR</span>
              </span>
              <span class="whitespace-pre-wrap break-words">{{ c.body }}</span>
            </li>
          </ul>
        </div>

        <!-- Review body -->
        <Textarea
          v-model="reviewBody"
          placeholder="Leave a comment (optional)…"
          rows="4"
          class="text-sm resize-none"
          aria-label="Review body"
        />

        <DialogFooter>
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
          This will delete all {{ pendingReview?.comments?.length ?? 0 }} pending
          comment{{ (pendingReview?.comments?.length ?? 0) !== 1 ? 's' : '' }} and cannot be undone.
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
