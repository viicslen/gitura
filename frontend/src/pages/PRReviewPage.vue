<script setup lang="ts">
import { errorMessage } from '@/lib/utils'
import { onMounted, reactive, ref, computed, watch, watchEffect, provide } from 'vue'
import { RefreshCw, ChevronLeft, ChevronRight, Terminal, FolderOpen, ChevronUp, ChevronDown, GitMerge } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import CommentSummaryList from '@/components/comment/CommentSummaryList.vue'
import CommentDetailPanel from '@/components/comment/CommentDetailPanel.vue'
import RunPanel from '@/components/run/RunPanel.vue'
import DiffReviewView from '@/components/diff/DiffReviewView.vue'
import PROverviewPanel from '@/components/pr/PROverviewPanel.vue'
import { useCommentThreads } from '@/composables/useCommentThreads'
import { useRuns } from '@/composables/useRuns'
import { useAuth } from '@/composables/useAuth'
import type { PRRef } from '@/types/pr'
import type * as model from '@/bindings/gitura/internal/model'
import { useToggles } from '@/composables/useToggles'
import { GetCommands, GetDefaultCommandName, GetPRLocalPath, SetPRLocalPath, OpenFolderPicker, GetPROverview, MergePR, ClosePR } from '@/bindings/gitura/internal/app/app'
import { toast } from 'vue-sonner'
import { useReviewTabs } from '@/composables/useReviewTabs'
import { useViewShortcuts } from '@/composables/useViewShortcuts'
import { useKeybindings, keyLabel } from '@/composables/useKeybindings'
import type { UnsentComments } from '@/lib/diffLayout'

const { reviewTab, tabBadges } = useReviewTabs()

const props = defineProps<{
  prItem: PRRef
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

reviewTab.value = 'overview'

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
const { toggles } = useToggles()
const showDiffThreads = ref(toggles.value.show_comments)

const refreshing = ref(false)
const overview = ref<model.PROverviewDTO | null>(null)
const showResolvedReviews = ref(toggles.value.show_resolved_reviews)

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
} = useCommentThreads(props.prItem)

const { authState } = useAuth()
provide('viewerLogin', computed(() => authState.value.login))
const isOwnPR = computed(() => !!prSummary.value?.author_login && prSummary.value.author_login === authState.value.login)

function handleSelect(index: number): void {
  currentIndex.value = index
}

const { bindings } = useKeybindings()
useViewShortcuts(() => reviewTab.value === 'conversation', () => ({ [bindings.value.nextThread]: goNext, [bindings.value.prevThread]: goPrev }))

watchEffect(() => {
  tabBadges.overview = overview.value?.reviews.filter((r) => r.has_threads && !r.all_threads_resolved).length ?? 0
  tabBadges.conversation = threads.value.filter((t) => !t.resolved).length
})

// Lives here, not in DiffReviewView, because a failed refresh unmounts the Files view.
const unsentComments: UnsentComments = reactive(new Map())

function handleReplySent(comment: model.ReviewCommentDTO): void {
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
  } catch (err) {
    toast.error('Could not load overview: ' + errorMessage(err))
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

provide('commentDeleted', handleReviewActivity)

async function refreshPRView(): Promise<void> {
  refreshing.value = true
  try {
    await loadPR()
    await diffReviewRef.value?.reload()
  } catch (err) {
    toast.error('Could not refresh PR data: ' + errorMessage(err))
  } finally {
    refreshing.value = false
  }
}

// ── Merge / close ─────────────────────────────────────────────────────────
type PRAction = 'merge' | 'squash' | 'rebase' | 'close'
const prActionLabels: Record<PRAction, string> = { merge: 'Merge', squash: 'Squash and merge', rebase: 'Rebase and merge', close: 'Close pull request' }
const confirmingAction = ref<PRAction | null>(null)
const actionRunning = ref(false)
const actionError = ref('')

function askAction(action: PRAction): void {
  actionError.value = ''
  confirmingAction.value = action
}

async function runAction(): Promise<void> {
  const action = confirmingAction.value
  if (!action) return
  actionRunning.value = true
  actionError.value = ''
  try {
    await (action === 'close' ? ClosePR() : MergePR(action))
    confirmingAction.value = null
    toast.success(action === 'close' ? 'Pull request closed' : 'Pull request merged')
    await refreshPRView()
    if (action !== 'close' && toggles.value.return_after_merge) emit('close-review')
  } catch (err) {
    actionError.value = errorMessage(err)
  } finally {
    actionRunning.value = false
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
    toast.error('Could not open folder picker: ' + errorMessage(err))
  }
}

onMounted(() => {
  void loadPR()
  void loadCommands()
  void loadLocalPath()
})

watch(showResolvedReviews, async (value) => {
  if (loading.value || !prSummary.value) return
  try {
    overview.value = await GetPROverview(value)
  } catch (err) {
    toast.error('Could not update overview: ' + errorMessage(err))
  }
})

watch(prSummary, async (summary) => {
  if (!summary) return
  try {
    overview.value = await GetPROverview(showResolvedReviews.value)
  } catch (err) {
    toast.error('Could not load overview: ' + errorMessage(err))
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
  <div class="flex flex-col h-full">
    <!-- Refresh affects the whole PR, so it lives in the title strip. -->
    <Teleport defer to="#title-strip-pr-actions">
      <Button
        variant="ghost"
        size="icon"
        class="h-7 w-7 text-muted-foreground hover:text-foreground"
        :disabled="loading || refreshing"
        :aria-label="refreshing ? 'Refreshing pull request' : 'Refresh pull request'"
        :title="refreshing ? 'Refreshing…' : 'Refresh pull request'"
        style="--wails-draggable: no-drag"
        @click="refreshPRView()"
      >
        <RefreshCw :class="['h-3.5 w-3.5', (loading || refreshing) && 'animate-spin']" />
      </Button>
    </Teleport>
    <Teleport defer to="#title-strip-actions">
      <DropdownMenu v-if="prSummary?.state === 'open'">
        <DropdownMenuTrigger as-child>
          <Button
            variant="outline"
            size="sm"
            class="h-7 ml-1 gap-1.5 px-2.5 text-xs border-emerald-500/40 bg-emerald-500/15 text-emerald-700 hover:bg-emerald-500/25 hover:text-emerald-800 dark:text-emerald-300 dark:hover:text-emerald-200"
            aria-label="Merge or close pull request"
            title="Merge or close pull request"
            style="--wails-draggable: no-drag"
          >
            <GitMerge class="h-3.5 w-3.5" />
            Merge
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem :disabled="prSummary.is_draft" @select="askAction('merge')">{{ prActionLabels.merge }}</DropdownMenuItem>
          <DropdownMenuItem :disabled="prSummary.is_draft" @select="askAction('squash')">{{ prActionLabels.squash }}</DropdownMenuItem>
          <DropdownMenuItem :disabled="prSummary.is_draft" @select="askAction('rebase')">{{ prActionLabels.rebase }}</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem class="text-destructive" @select="askAction('close')">{{ prActionLabels.close }}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </Teleport>

    <Dialog :open="!!confirmingAction" @update:open="(open: boolean) => { if (!open && !actionRunning) confirmingAction = null }">
      <DialogContent class="max-w-sm">
        <DialogHeader>
          <DialogTitle>{{ confirmingAction && prActionLabels[confirmingAction] }}?</DialogTitle>
        </DialogHeader>
        <p class="text-sm text-muted-foreground">
          {{ confirmingAction === 'close'
            ? `Close #${prSummary?.number} without merging.`
            : `${confirmingAction && prActionLabels[confirmingAction]} ${prSummary?.head_branch} into ${prSummary?.base_branch}. Fails if new commits were pushed since this PR was loaded.` }}
        </p>
        <p v-if="actionError" class="text-sm text-destructive">{{ actionError }}</p>
        <DialogFooter>
          <Button variant="ghost" :disabled="actionRunning" @click="confirmingAction = null">Cancel</Button>
          <Button :variant="confirmingAction === 'close' ? 'destructive' : 'default'" :disabled="actionRunning" @click="runAction()">
            {{ actionRunning ? 'Working…' : confirmingAction && prActionLabels[confirmingAction] }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- ── Loading state ───────────────────────────────────────────────────── -->
    <div v-if="loading && !prSummary" class="flex-1 flex flex-col items-center justify-center gap-3 text-muted-foreground">
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
      <div v-show="reviewTab === 'conversation'" class="flex flex-1 min-h-0 overflow-hidden">
        <!-- Left: thread list with its own filter -->
        <div class="w-72 shrink-0 border-r border-border flex flex-col min-h-0">
          <div class="h-10 shrink-0 border-b border-border px-3 flex items-center gap-2">
            <span class="text-xs font-medium">Threads</span>
            <div class="ml-auto flex shrink-0 whitespace-nowrap rounded-md bg-muted p-0.5 text-xs" role="group" aria-label="Thread filter">
              <button
                class="rounded px-2 py-0.5"
                :class="!showResolved ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground'"
                :aria-pressed="!showResolved"
                @click="showResolved && toggleShowResolved()"
              >Unresolved <span class="tabular-nums opacity-60">{{ threads.filter((t) => !t.resolved).length }}</span></button>
              <button
                class="rounded px-2 py-0.5"
                :class="showResolved ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground'"
                :aria-pressed="showResolved"
                @click="!showResolved && toggleShowResolved()"
              >All <span class="tabular-nums opacity-60">{{ threads.length }}</span></button>
            </div>
          </div>
          <div
            v-if="!showResolved && queue.length === 0 && threads.length > 0"
            class="flex-1 flex flex-col items-center justify-center gap-2 p-4 text-center text-sm text-muted-foreground"
          >
            All {{ threads.length }} threads are resolved.
            <Button variant="outline" size="sm" class="h-7 text-xs" @click="toggleShowResolved()">Show all threads</Button>
          </div>
          <div v-else class="flex-1 min-h-0 overflow-y-auto">
            <CommentSummaryList
              :threads="queue"
              :review-numbers="overview?.review_numbers"
              :current-index="currentIndex"
              :show-resolved="showResolved"
              :commands="commands"
              :default-command-name="defaultCommandName"
              @select="handleSelect"
              @ran="handleRan"
            />
          </div>
        </div>

        <!-- Right: thread pager, detail, run tools -->
        <div class="flex flex-col flex-1 min-h-0 overflow-hidden">
          <div v-if="queue.length > 0" class="h-10 shrink-0 border-b border-border px-4 flex items-center gap-1 text-xs">
            <span class="text-muted-foreground tabular-nums">Thread {{ currentIndex + 1 }} of {{ queue.length }}</span>
            <span class="flex-1" />
            <span class="hidden lg:inline text-muted-foreground/70 mr-2"><kbd class="font-sans">{{ keyLabel(bindings.prevThread) }}</kbd> <kbd class="font-sans">{{ keyLabel(bindings.nextThread) }}</kbd> to move</span>
            <Button variant="ghost" size="sm" class="h-7 gap-1 text-xs" :disabled="!canGoBack" @click="goPrev()">
              <ChevronLeft class="h-3.5 w-3.5" /> Previous
            </Button>
            <Button variant="ghost" size="sm" class="h-7 gap-1 text-xs" :disabled="!canGoForward" @click="goNext()">
              Next <ChevronRight class="h-3.5 w-3.5" />
            </Button>
          </div>
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
          <!-- Command tools: where commands run and what they produced -->
          <div v-if="commands.length > 0" class="h-9 shrink-0 border-t border-border px-2 flex items-center gap-1 text-xs">
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="sm" class="h-7 gap-1.5 text-xs min-w-0 max-w-[60%]" aria-label="Local checkout for commands">
                  <FolderOpen class="h-3.5 w-3.5 shrink-0" />
                  <span class="text-muted-foreground shrink-0">Run in</span>
                  <span class="font-mono truncate" :class="!localPath && 'text-muted-foreground italic font-sans'">{{ localPath || 'not set' }}</span>
                  <ChevronDown class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" side="top" class="w-80 p-2">
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
            <Button
              variant="ghost"
              size="sm"
              class="ml-auto h-7 gap-1.5 text-xs"
              :class="runPanelOpen ? 'text-foreground' : 'text-muted-foreground'"
              :aria-expanded="runPanelOpen"
              @click="runPanelOpen = !runPanelOpen"
            >
              <Terminal class="h-3.5 w-3.5" />
              Run history
              <span v-if="pendingRunCount > 0" class="rounded-full bg-primary px-1.5 text-[0.625rem] text-primary-foreground tabular-nums">{{ pendingRunCount }}</span>
              <ChevronUp class="h-3.5 w-3.5 transition-transform" :class="runPanelOpen && 'rotate-180'" />
            </Button>
          </div>
        </div>
      </div>

      <PROverviewPanel
        v-show="reviewTab === 'overview'"
        v-model:show-resolved-reviews="showResolvedReviews"
        :overview="overview"
        class="flex-1 min-h-0 overflow-hidden"
      />

      <!-- Files changed view -->
      <DiffReviewView
        v-show="reviewTab === 'files'"
        ref="diffReviewRef"
        :threads="threads"
        v-model:show-threads="showDiffThreads"
        :is-own-pr="isOwnPR"
        :head-sha="prSummary?.head_sha ?? ''"
        :active="reviewTab === 'files'"
        :unsent-comments="unsentComments"
        class="flex-1 min-h-0 overflow-hidden"
        @resolve="handleResolve"
        @unresolve="handleUnresolve"
        @reply-sent="handleReplySent"
        @threads-changed="handleReviewActivity"
      />
    </template>
  </div>
</template>
