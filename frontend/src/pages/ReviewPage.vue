<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { RefreshCw, ChevronLeft, ChevronRight, Terminal, FolderOpen } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import CommentSummaryList from '@/components/CommentSummaryList.vue'
import CommentDetailPanel from '@/components/CommentDetailPanel.vue'
import RunPanel from '@/components/RunPanel.vue'
import ViewToggle from '@/components/ViewToggle.vue'
import DiffReviewView from '@/components/DiffReviewView.vue'
import PROverviewPanel from '@/components/PROverviewPanel.vue'
import { useReview } from '@/composables/useReview'
import { useRuns } from '@/composables/useRuns'
import { useAuth } from '@/composables/useAuth'
import type { ReviewLoadInput } from '@/types/review'
import type { model } from '../wailsjs/go/models'
import { GetCommands, GetDefaultCommandName, GetPRLocalPath, SetPRLocalPath, OpenFolderPicker, GetPROverview } from '../wailsjs/go/main/App'
import { toast } from 'vue-sonner'

const props = defineProps<{
  prItem: ReviewLoadInput
}>()

const emit = defineEmits<{
  (e: 'close-review'): void
  (e: 'update-pr-meta', meta: {
    owner: string
    repo: string
    number: number
    title: string
    state?: string
    is_draft?: boolean
    html_url?: string
  }): void
}>()

const prView = ref<'overview' | 'conversation' | 'files'>('overview')

const VIEW_OPTIONS = [
  { value: 'overview', label: 'Overview' },
  { value: 'conversation', label: 'Conversation' },
  { value: 'files', label: 'Files changed' },
]

// ── Commands ────────────────────────────────────────────────────────────────
const commands = ref<model.CommandDTO[]>([])
const defaultCommandName = ref('')

async function loadCommands(): Promise<void> {
  try {
    commands.value = await GetCommands()
  } catch {
    // Non-fatal: commands panel will just be empty
  }
  try {
    defaultCommandName.value = await GetDefaultCommandName()
  } catch {
    // Non-fatal
  }
}

// ── Run panel ──────────────────────────────────────────────────────────────
const { runs } = useRuns()
const runPanelOpen = ref(false)

const pendingRunCount = computed(() => runs.value.filter((r) => r.running).length)

// ── DiffReviewView bridge ──────────────────────────────────────────────────
const diffReviewRef = ref<InstanceType<typeof DiffReviewView> | null>(null)
const diffCanGoPrev = computed(() => diffReviewRef.value?.canGoPrev ?? false)
const diffCanGoNext = computed(() => diffReviewRef.value?.canGoNext ?? false)
const showDiffThreads = ref(true)
const diffFileCount = computed(() => diffReviewRef.value?.files?.length ?? 0)
const diffAdded = computed(() => diffReviewRef.value?.files?.filter(f => f.status === 'added').length ?? 0)
const diffDeleted = computed(() => diffReviewRef.value?.files?.filter(f => f.status === 'removed').length ?? 0)
const diffModified = computed(() => diffReviewRef.value?.files?.filter(f => f.status !== 'added' && f.status !== 'removed').length ?? 0)
function diffPrevFile(): void { diffReviewRef.value?.prevFile() }
function diffNextFile(): void { diffReviewRef.value?.nextFile() }

const refreshing = ref(false)
const overview = ref<model.PROverviewDTO | null>(null)
const showResolvedReviews = ref(false)

const {
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
  reloadReviewActivity,
} = useReview(props.prItem)

const { authState } = useAuth()
const isOwnPR = computed(() => !!prSummary.value?.author_login && prSummary.value.author_login === authState.value.login)

function handleSelect(index: number): void {
  currentIndex.value = index
}

function handleKeydown(event: KeyboardEvent): void {
  const tag = (event.target as HTMLElement).tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA') return
  if (event.key === 'ArrowRight') goNext()
  else if (event.key === 'ArrowLeft') goPrev()
}

function handleReplySent(comment: model.CommentDTO): void {
  void addReplyToThread(comment)
}
function handleSuggestionCommitted(): void {}

function handleRan(): void {
  // Auto-open the run panel when a run starts
  runPanelOpen.value = true
}

async function refreshOverview(): Promise<void> {
  try {
    overview.value = await GetPROverview(showResolvedReviews.value)
  } catch {
    // Non-fatal; the overview refreshes on the next load.
  }
}

async function handleResolve(rootId: number): Promise<void> {
  await resolveThread(rootId)
  await refreshOverview()
}

async function handleUnresolve(rootId: number): Promise<void> {
  await unresolveThread(rootId)
  await refreshOverview()
}

async function handleReviewActivity(): Promise<void> {
  await reloadReviewActivity()
  await refreshOverview()
}

async function refreshPRView(): Promise<void> {
  refreshing.value = true
  try {
    await loadPR()
    overview.value = await GetPROverview(showResolvedReviews.value)
    await diffReviewRef.value?.reload()
  } catch (err) {
    toast.error('Could not refresh PR data: ' + String(err))
  } finally {
    refreshing.value = false
  }
}

// ── Per-PR local path ─────────────────────────────────────────────────────
const localPath = ref('')

async function loadLocalPath(): Promise<void> {
  try {
    localPath.value = await GetPRLocalPath(props.prItem.owner, props.prItem.repo, props.prItem.number)
  } catch {
    // Non-fatal: path simply stays empty
  }
}

async function saveLocalPath(path: string): Promise<void> {
  localPath.value = path
  try {
    await SetPRLocalPath(props.prItem.owner, props.prItem.repo, props.prItem.number, path)
  } catch {
    // Non-fatal: value was set in-memory but not persisted
  }
}

async function browseForPath(): Promise<void> {
  try {
    const selected = await OpenFolderPicker(
      'Select local repository folder',
      localPath.value || '',
    )
    if (selected) {
      await saveLocalPath(selected)
    }
  } catch (err) {
    toast.error('Could not open folder picker: ' + String(err))
  }
}

onMounted(() => {
  void loadPR().then(async () => {
    overview.value = await GetPROverview(showResolvedReviews.value)
  }).catch(() => {})
  void loadCommands()
  void loadLocalPath()
})

watch(showResolvedReviews, async (value) => {
  if (loading.value || !prSummary.value) return
  try {
    overview.value = await GetPROverview(value)
  } catch (err) {
    toast.error('Could not update overview: ' + String(err))
  }
})

watch(prSummary, async (summary) => {
  if (!summary) return
  try {
    overview.value = await GetPROverview(showResolvedReviews.value)
  } catch {
    // Non-fatal; handled by refresh/load error paths.
  }
})

watch(
  prSummary,
  (summary) => {
    emit('update-pr-meta', {
      owner: props.prItem.owner,
      repo: props.prItem.repo,
      number: props.prItem.number,
      title: summary?.title ?? props.prItem.title,
      state: summary?.state ?? props.prItem.state,
      is_draft: summary?.is_draft ?? props.prItem.is_draft,
      html_url: summary?.html_url ?? props.prItem.html_url,
    })
  },
  { immediate: true },
)
</script>

<template>
  <div class="flex flex-col h-full" @keydown="handleKeydown" tabindex="-1">
    <!-- ── Top bar ─────────────────────────────────────────────────────────── -->
    <header class="flex items-center gap-3 px-4 py-2.5 border-b border-border shrink-0">
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
      <div v-else-if="prView === 'overview'" class="flex items-center gap-2 shrink-0">
        <Switch
          :model-value="showResolvedReviews"
          aria-label="Show reviews with resolved conversations"
          @update:model-value="showResolvedReviews = !showResolvedReviews"
        />
        <span class="text-xs text-muted-foreground select-none">Show resolved reviews</span>
      </div>
      <div v-else class="flex items-center gap-2 shrink-0">
        <Switch
          v-model="showDiffThreads"
          aria-label="Show comments in diff"
        />
        <span class="text-xs text-muted-foreground select-none">Comments</span>
      </div>

      <!-- Spacer -->
      <div class="flex-1" />

      <!-- Comment counts (conversation only) -->
      <div v-if="prSummary && prView === 'conversation'" class="flex items-center gap-2 shrink-0 text-xs text-muted-foreground">
        <span>{{ prSummary.unresolved_count }} unresolved</span>
        <span class="text-border">·</span>
        <span>{{ prSummary.comment_count }} total</span>
      </div>
      <div v-if="overview && prView === 'overview'" class="flex items-center gap-2 shrink-0 text-xs text-muted-foreground">
        <span>{{ overview.general_comments.length }} comments</span>
        <span class="text-border">·</span>
        <span>{{ overview.reviews.length }} reviews</span>
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

      <!-- Run history toggle (only shown when commands are configured) -->
      <Button
        v-if="prView === 'conversation' && commands.length > 0"
        variant="ghost"
        size="icon"
        :aria-label="runPanelOpen ? 'Close run history' : 'Open run history'"
        :class="runPanelOpen ? 'text-foreground' : 'text-muted-foreground'"
        @click="runPanelOpen = !runPanelOpen"
      >
        <span class="relative">
          <Terminal class="h-4 w-4" />
          <span
            v-if="pendingRunCount > 0"
            class="absolute -top-1 -right-1 h-2 w-2 rounded-full bg-primary"
            aria-hidden="true"
          />
        </span>
      </Button>

      <!-- Local repo path (only shown when commands are configured) -->
      <DropdownMenu v-if="prView === 'conversation' && commands.length > 0">
        <DropdownMenuTrigger as-child>
          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7 text-muted-foreground hover:text-foreground shrink-0"
            aria-label="Open local repository path menu"
          >
            <FolderOpen class="h-3.5 w-3.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-80 p-2">
          <div class="flex items-center gap-2">
            <Input
              :value="localPath"
              placeholder="Local repo path..."
              class="h-8 text-xs font-mono"
              aria-label="Local repository path"
              @change="(e: Event) => saveLocalPath((e.target as HTMLInputElement).value)"
            />
            <Button
              variant="outline"
              size="sm"
              class="h-8 shrink-0"
              aria-label="Browse for local repository folder"
              @click="browseForPath()"
            >
              Browse
            </Button>
          </div>
        </DropdownMenuContent>
      </DropdownMenu>

      <!-- Prev/Next (both views) -->
      <Button
        variant="ghost"
        size="icon"
        :disabled="loading || refreshing"
        aria-label="Refresh pull request data"
        @click="refreshPRView()"
      >
        <RefreshCw :class="['h-4 w-4', (loading || refreshing) && 'animate-spin']" />
      </Button>
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
            :commands="commands"
            :default-command-name="defaultCommandName"
            @select="handleSelect"
            @ran="handleRan"
          />
        </div>

        <!-- Right: detail + navigation -->
        <div class="flex flex-col flex-1 min-h-0 overflow-hidden">
          <CommentDetailPanel
            :thread="currentThread"
            :is-at-end="isAtEnd && queue.length > 0"
            :commands="commands"
            :default-command-name="defaultCommandName"
            class="flex-1 overflow-hidden"
            @resolve="handleResolve"
            @unresolve="handleUnresolve"
            @reply-sent="handleReplySent"
            @suggestion-committed="handleSuggestionCommitted"
            @ran="handleRan"
          />
          <!-- Run history panel (pinned to bottom of right panel) -->
          <RunPanel
            :open="runPanelOpen"
            :current-thread-root-id="currentThread?.root_id"
            @close="runPanelOpen = false"
          />
        </div>
      </div>

      <PROverviewPanel
        v-if="prView === 'overview'"
        :overview="overview"
        class="flex-1 min-h-0 overflow-hidden"
      />

      <!-- Files changed view -->
      <DiffReviewView
        v-show="prView === 'files'"
        ref="diffReviewRef"
        :threads="threads"
        :show-threads="showDiffThreads"
        :is-own-pr="isOwnPR"
        :head-sha="prSummary?.head_sha ?? ''"
        class="flex-1 min-h-0 overflow-hidden"
        @resolve="handleResolve"
        @unresolve="handleUnresolve"
        @reply-sent="handleReplySent"
        @threads-changed="handleReviewActivity"
      />
    </template>
  </div>
</template>
