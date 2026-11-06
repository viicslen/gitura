<script setup lang="ts">
import { onMounted, computed, provide, ref } from 'vue'
import { Send, Trash2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import DiffFileSidebar from './DiffFileSidebar.vue'
import DiffFileView from './DiffFileView.vue'
import { useDiffReview } from '@/composables/useDiffReview'
import type { model } from '@/wailsjs/go/models'
import { toast } from 'vue-sonner'

const props = defineProps<{
  threads: model.CommentThreadDTO[]
  showThreads: boolean
  /** GitHub rejects approve/request-changes from the PR author. */
  isOwnPr: boolean
  headSha: string
}>()

const emit = defineEmits<{
  (e: 'resolve', rootId: number): void
  (e: 'unresolve', rootId: number): void
  (e: 'reply-sent', comment: model.CommentDTO): void
  /** New threads were created on GitHub; the parent should re-fetch them. */
  (e: 'threads-changed'): void
}>()

const {
  files,
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
  addDraftComment,
  postImmediateComment,
  submitReview,
  discardPendingReview,
  loadPendingReview,
} = useDiffReview()

const threadsForFile = computed(() => props.threads.filter((t) => t.path === currentFilePath.value))
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
const pendingCount = computed(() => pendingReview.value?.comments.length ?? 0)
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
    await selectFile(selectedPath)
  } else if (files.value.length > 0) {
    await selectFile(files.value[0].filename)
  }

  if (filesError.value) {
    throw new Error(filesError.value)
  }
  if (diffError.value) {
    throw new Error(diffError.value)
  }
}

// ── Keyboard nav ──────────────────────────────────────────────────────────
function handleKeydown(event: KeyboardEvent): void {
  if ((event.target as HTMLElement).closest('input, textarea, [contenteditable]')) return
  if (event.key === ']') nextFile()
  else if (event.key === '[') prevFile()
}

// ── Exposed interface for parent ──────────────────────────────────────────
defineExpose({ canGoPrev, canGoNext, prevFile, nextFile, files, reload })
</script>

<template>
  <div
    class="flex flex-col outline-none"
    tabindex="-1"
    @keydown="handleKeydown"
  >
    <!-- ── Top toolbar ────────────────────────────────────────────────────── -->
    <div class="flex items-center gap-2 px-3 py-1.5 border-b border-border shrink-0">
      <span class="text-xs text-muted-foreground truncate flex-1 min-w-0">
        {{ currentFile?.filename ?? 'Select a file' }}
      </span>

      <Badge v-if="pendingCount > 0" variant="secondary" class="gap-1 text-xs">
        {{ pendingCount }} pending
      </Badge>
      <Button
        :variant="pendingCount > 0 ? 'default' : 'outline'"
        size="sm"
        class="gap-1.5"
        @click="submitPanelOpen = true"
      >
        <Send class="h-3.5 w-3.5" />
        {{ pendingCount > 0 ? 'Finish review' : 'Review changes' }}
      </Button>
      <template v-if="pendingReview?.has_pending">
        <Button
          variant="ghost"
          size="icon"
          class="text-destructive hover:text-destructive"
          aria-label="Discard pending review"
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
      <div class="w-64 shrink-0 overflow-hidden">
        <DiffFileSidebar
          :files="files"
          :comment-counts="unresolvedCounts"
          :selected-path="currentFilePath"
          :loading="filesLoading"
          @select="selectFile"
        />
      </div>

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
          class="flex-1 min-h-0 overflow-hidden"
          :post-comment="postComment"
          @resolve="emit('resolve', $event)"
          @unresolve="emit('unresolve', $event)"
          @reply-sent="emit('reply-sent', $event)"
        />
      </div>
    </div>

    <!-- ── Submit review panel ────────────────────────────────────────────── -->
    <Dialog :open="submitPanelOpen" @update:open="submitPanelOpen = $event">
      <DialogContent class="max-w-md">
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

        <div v-if="pendingCount > 0" class="flex flex-col gap-1">
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
          This will delete all {{ pendingReview?.comments.length ?? 0 }} pending
          comment{{ (pendingReview?.comments.length ?? 0) !== 1 ? 's' : '' }} and cannot be undone.
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
