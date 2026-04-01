<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { ArrowLeft, RefreshCw, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import CommentSummaryList from '@/components/CommentSummaryList.vue'
import CommentDetailPanel from '@/components/CommentDetailPanel.vue'
import ViewToggle from '@/components/ViewToggle.vue'
import DiffReviewView from '@/components/DiffReviewView.vue'
import { useReview } from '@/composables/useReview'
import type { ReviewLoadInput } from '@/types/review'

const props = defineProps<{
  prItem: ReviewLoadInput
}>()

const emit = defineEmits<{
  (e: 'close-review'): void
}>()

const prView = ref<'conversation' | 'files'>('conversation')

const VIEW_OPTIONS = [
  { value: 'conversation', label: 'Conversation' },
  { value: 'files', label: 'Files changed' },
]

// ── DiffReviewView bridge ──────────────────────────────────────────────────
const diffReviewRef = ref<InstanceType<typeof DiffReviewView> | null>(null)
const diffCanGoPrev = computed(() => diffReviewRef.value?.canGoPrev ?? false)
const diffCanGoNext = computed(() => diffReviewRef.value?.canGoNext ?? false)
const diffShowOtherThreads = computed(() => diffReviewRef.value?.showOtherThreads ?? false)
const diffFileCount = computed(() => diffReviewRef.value?.files?.length ?? 0)
const diffAdded = computed(() => diffReviewRef.value?.files?.filter(f => f.status === 'added').length ?? 0)
const diffDeleted = computed(() => diffReviewRef.value?.files?.filter(f => f.status === 'removed').length ?? 0)
const diffModified = computed(() => diffReviewRef.value?.files?.filter(f => f.status !== 'added' && f.status !== 'removed').length ?? 0)
function diffPrevFile(): void { diffReviewRef.value?.prevFile() }
function diffNextFile(): void { diffReviewRef.value?.nextFile() }
function diffToggleOtherThreads(): void { diffReviewRef.value?.toggleOtherThreads() }

const {
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
} = useReview(props.prItem)

function handleSelect(index: number): void {
  currentIndex.value = index
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === 'ArrowRight') goNext()
  else if (event.key === 'ArrowLeft') goPrev()
}

function handleReplySent(): void {
  // Reply is appended to the thread in the Go cache and returned via the event.
  // The CommentDetailPanel binds directly to thread.comments, which is updated
  // reactively when ReplyComposer emits reply-sent to CommentDetailPanel which
  // emits it here. Nothing additional needed at this level.
}

function handleSuggestionCommitted(): void {
  // SuggestionBlock manages its own success state.
  // Nothing additional needed at the page level.
}

onMounted(() => {
  loadPR()
})
</script>

<template>
  <div class="flex flex-col h-full" @keydown="handleKeydown" tabindex="-1">
    <!-- ── Top bar ─────────────────────────────────────────────────────────── -->
    <header class="flex items-center gap-3 px-4 py-2.5 border-b border-border shrink-0">
      <Button variant="ghost" size="icon" aria-label="Back to PR list" @click="emit('close-review')">
        <ArrowLeft class="h-4 w-4" />
      </Button>

      <div class="min-w-0">
        <div class="flex items-center gap-2 min-w-0">
          <span class="text-sm font-semibold truncate">
            {{ prSummary?.title ?? prItem.title }}
          </span>
          <span class="text-xs text-muted-foreground shrink-0">
            #{{ prItem.number }}
          </span>
          <Badge v-if="prSummary?.is_draft" variant="secondary" class="text-xs shrink-0">
            Draft
          </Badge>
          <Badge
            v-else-if="prSummary?.state === 'merged'"
            class="text-xs shrink-0 bg-violet-500/15 text-violet-700 dark:text-violet-300 border-violet-500/30"
          >
            Merged
          </Badge>
          <Badge
            v-else-if="prSummary?.state === 'closed'"
            variant="destructive"
            class="text-xs shrink-0"
          >
            Closed
          </Badge>
        </div>
        <div class="text-xs text-muted-foreground mt-0.5">
          {{ prItem.owner }}/{{ prItem.repo }}
        </div>
      </div>

      <!-- View toggle -->
      <ViewToggle
        v-model="prView"
        :options="VIEW_OPTIONS"
        class="shrink-0"
      />

      <!-- Sub-toggle: shown immediately after the view toggle -->
      <div v-if="prView === 'conversation'" class="flex items-center gap-2 shrink-0">
        <Switch
          :model-value="showResolved"
          aria-label="Show resolved threads"
          @update:model-value="toggleShowResolved()"
        />
        <span class="text-xs text-muted-foreground select-none">Show resolved</span>
      </div>
      <div v-else class="flex items-center gap-2 shrink-0">
        <Switch
          :model-value="diffShowOtherThreads"
          aria-label="Show reviewer comments"
          @update:model-value="diffToggleOtherThreads()"
        />
        <span class="text-xs text-muted-foreground select-none">Reviewer comments</span>
      </div>

      <!-- Spacer -->
      <div class="flex-1" />

      <!-- Comment counts (conversation only) -->
      <div v-if="prSummary && prView === 'conversation'" class="flex items-center gap-2 shrink-0 text-xs text-muted-foreground">
        <span>{{ prSummary.unresolved_count }} unresolved</span>
        <span class="text-border">·</span>
        <span>{{ prSummary.comment_count }} total</span>
      </div>

      <!-- File count (files view only) -->
      <div v-if="prView === 'files' && diffFileCount > 0" class="flex items-center gap-2 shrink-0 text-xs text-muted-foreground">
        <template v-if="diffModified > 0">
          <span>{{ diffModified }} modified</span>
        </template>
        <template v-if="diffAdded > 0">
          <span v-if="diffModified > 0" class="text-border">·</span>
          <span>{{ diffAdded }} added</span>
        </template>
        <template v-if="diffDeleted > 0">
          <span v-if="diffModified > 0 || diffAdded > 0" class="text-border">·</span>
          <span>{{ diffDeleted }} deleted</span>
        </template>
        <span class="text-border">·</span>
        <span>{{ diffFileCount }} total</span>
      </div>

      <!-- Prev/Next (both views) -->
      <Button
        variant="ghost"
        size="icon"
        :disabled="prView === 'conversation' ? !canGoBack : !diffCanGoPrev"
        :aria-label="prView === 'conversation' ? 'Previous comment' : 'Previous file'"
        @click="prView === 'conversation' ? goPrev() : diffPrevFile()"
      >
        <ChevronLeft class="h-4 w-4" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        :disabled="prView === 'conversation' ? !canGoForward : !diffCanGoNext"
        :aria-label="prView === 'conversation' ? 'Next comment' : 'Next file'"
        @click="prView === 'conversation' ? goNext() : diffNextFile()"
      >
        <ChevronRight class="h-4 w-4" />
      </Button>
    </header>

    <!-- ── Loading state ───────────────────────────────────────────────────── -->
    <div v-if="loading" class="flex-1 flex flex-col items-center justify-center gap-3 text-muted-foreground">
      <RefreshCw class="h-6 w-6 animate-spin" />
      <div class="text-sm">Loading review…</div>
      <div v-if="loadProgress.loaded > 0" class="text-xs">
        {{ loadProgress.loaded }} thread{{ loadProgress.loaded !== 1 ? 's' : '' }} loaded
        <span v-if="loadProgress.total > 0"> of {{ loadProgress.total }}</span>
      </div>
    </div>

    <!-- ── Error state ─────────────────────────────────────────────────────── -->
    <div
      v-else-if="error"
      class="flex-1 flex flex-col items-center justify-center gap-3 p-6"
    >
      <p class="text-sm text-destructive text-center">{{ error }}</p>
      <Button variant="outline" size="sm" @click="loadPR()">
        Retry
      </Button>
    </div>

    <!-- ── Content area ────────────────────────────────────────────────────── -->
    <template v-else>
      <!-- Conversation view -->
      <div v-show="prView === 'conversation'" class="flex flex-1 min-h-0 overflow-hidden">
        <!-- Left: comment summary list -->
        <div class="w-72 shrink-0 border-r border-border overflow-y-auto">
          <CommentSummaryList
            :threads="queue"
            :current-index="currentIndex"
            :show-resolved="showResolved"
            @select="handleSelect"
          />
        </div>

        <!-- Right: detail + navigation -->
        <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
          <CommentDetailPanel
            :thread="currentThread"
            :is-at-end="isAtEnd && queue.length > 0"
            class="flex-1 overflow-hidden"
            @resolve="resolveThread"
            @unresolve="unresolveThread"
            @reply-sent="handleReplySent"
            @suggestion-committed="handleSuggestionCommitted"
          />
        </div>
      </div>

      <!-- Files changed view -->
      <DiffReviewView
        ref="diffReviewRef"
        v-show="prView === 'files'"
        class="flex-1 min-h-0 overflow-hidden"
      />
    </template>
  </div>
</template>
