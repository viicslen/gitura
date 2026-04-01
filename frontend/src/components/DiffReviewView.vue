<script setup lang="ts">
import { onMounted } from 'vue'
import { ChevronLeft, ChevronRight, Eye, EyeOff, Send, Trash2 } from 'lucide-vue-next'
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
import { ref } from 'vue'
import type { model } from '@/wailsjs/go/models'
import { toast } from 'vue-sonner'

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
  showOtherThreads,
  canGoPrev,
  canGoNext,
  threadsForFile,
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
} = useDiffReview()

// ── Review submit panel state ──────────────────────────────────────────────
const submitPanelOpen = ref(false)
const verdict = ref<'APPROVE' | 'REQUEST_CHANGES' | 'COMMENT'>('COMMENT')
const reviewBody = ref('')
const submitLoading = ref(false)

const VERDICT_OPTIONS: { value: 'APPROVE' | 'REQUEST_CHANGES' | 'COMMENT'; label: string; desc: string }[] = [
  { value: 'COMMENT', label: 'Comment', desc: 'Submit general feedback without explicit approval.' },
  { value: 'APPROVE', label: 'Approve', desc: 'Submit feedback and approve merging these changes.' },
  { value: 'REQUEST_CHANGES', label: 'Request changes', desc: 'Submit feedback that must be addressed before merging.' },
]

// ── Discard confirmation dialog ────────────────────────────────────────────
const discardDialogOpen = ref(false)
const discardLoading = ref(false)

// ── Lifecycle ─────────────────────────────────────────────────────────────
onMounted(async () => {
  await loadFiles()
  await loadPendingReview()
})

// ── Comment handlers ──────────────────────────────────────────────────────
async function handleDraftComment(comment: model.DraftCommentDTO): Promise<void> {
  try {
    await addDraftComment(comment)
    toast('Comment added to review')
  } catch (err) {
    toast.error(String(err))
  }
}

async function handleImmediateComment(comment: model.DraftCommentDTO): Promise<void> {
  try {
    await postImmediateComment(comment)
    toast('Comment posted')
  } catch (err) {
    toast.error(String(err))
  }
}

// ── Submit review ─────────────────────────────────────────────────────────
async function handleSubmitReview(): Promise<void> {
  submitLoading.value = true
  try {
    await submitReview({ verdict: verdict.value, body: reviewBody.value })
    submitPanelOpen.value = false
    reviewBody.value = ''
    toast('Review submitted')
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

// ── Keyboard nav ──────────────────────────────────────────────────────────
function handleKeydown(event: KeyboardEvent): void {
  if ((event.target as HTMLElement).tagName === 'TEXTAREA') return
  if (event.key === ']') nextFile()
  else if (event.key === '[') prevFile()
}
</script>

<template>
  <div
    class="flex flex-col outline-none"
    tabindex="-1"
    @keydown="handleKeydown"
  >
    <!-- ── Top toolbar ────────────────────────────────────────────────────── -->
    <div class="flex items-center gap-2 px-3 py-1.5 border-b border-border shrink-0">
      <!-- File nav -->
      <Button
        variant="ghost"
        size="icon"
        :disabled="!canGoPrev"
        aria-label="Previous file"
        @click="prevFile"
      >
        <ChevronLeft class="h-4 w-4" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        :disabled="!canGoNext"
        aria-label="Next file"
        @click="nextFile"
      >
        <ChevronRight class="h-4 w-4" />
      </Button>

      <span class="text-xs text-muted-foreground truncate flex-1 min-w-0">
        {{ currentFile?.filename ?? 'Select a file' }}
      </span>

      <!-- Other reviewer threads toggle -->
      <Button
        variant="ghost"
        size="sm"
        class="gap-1.5"
        :aria-label="showOtherThreads ? 'Hide reviewer comments' : 'Show reviewer comments'"
        @click="toggleOtherThreads"
      >
        <Eye v-if="!showOtherThreads" class="h-3.5 w-3.5" />
        <EyeOff v-else class="h-3.5 w-3.5" />
        <span class="hidden sm:inline text-xs">Reviewer comments</span>
      </Button>

      <!-- Pending review badge + actions -->
      <template v-if="pendingReview?.has_pending">
        <Badge variant="secondary" class="gap-1 text-xs">
          {{ pendingReview.comments.length }} pending
        </Badge>
        <Button
          variant="outline"
          size="sm"
          class="gap-1.5"
          aria-label="Submit review"
          @click="submitPanelOpen = true"
        >
          <Send class="h-3.5 w-3.5" />
          Submit review
        </Button>
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
          :other-threads="threadsForFile"
          :show-other-threads="showOtherThreads"
          class="flex-1 min-h-0 overflow-hidden"
          @draft-comment="handleDraftComment"
          @immediate-comment="handleImmediateComment"
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
            class="flex items-start gap-2.5 rounded-md border border-border px-3 py-2 cursor-pointer hover:bg-accent transition-colors"
            :class="verdict === option.value ? 'border-primary bg-accent' : ''"
          >
            <input
              type="radio"
              name="verdict"
              :value="option.value"
              :checked="verdict === option.value"
              class="mt-0.5"
              @change="verdict = option.value"
            />
            <div>
              <div class="text-sm font-medium">{{ option.label }}</div>
              <div class="text-xs text-muted-foreground">{{ option.desc }}</div>
            </div>
          </label>
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
          <Button :disabled="submitLoading" @click="handleSubmitReview">
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
