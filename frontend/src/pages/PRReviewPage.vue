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
import BusyLabel from '@/components/motion/BusyLabel.vue'
import CountUp from '@/components/motion/CountUp.vue'
import { confetti } from '@/components/motion/wow'
import { collapse, collapseX, collapsible, collapsibleX } from '@/components/motion/motion'
import { useCommentThreads } from '@/composables/useCommentThreads'
import { useRuns } from '@/composables/useRuns'
import { useAuth } from '@/composables/useAuth'
import type { PRRef } from '@/types/pr'
import type * as model from '@/bindings/gitura/internal/model'
import { useToggles } from '@/composables/useToggles'
import { GetCommands, GetDefaultCommandName, GetPRLocalPath, SetPRLocalPath, OpenFolderPicker, GetPROverview, MergePR, ClosePR } from '@/bindings/gitura/internal/app/app'
import { toast } from 'vue-sonner'
import { useReviewTabs, type ReviewTab } from '@/composables/useReviewTabs'
import { useViewShortcuts } from '@/composables/useViewShortcuts'
import { useKeybindings, keyLabel } from '@/composables/useKeybindings'
import type { UnsentComments } from '@/lib/diffLayout'

const { reviewTab, tabBadges } = useReviewTabs()
// A tab slides in from the side its button sits on, and the one it replaces drops back. The Files tab can hold a diff
// of thousands of lines, so going to or from it only the arriving panel moves and the other is gone at once.
const TAB_ORDER: ReviewTab[] = ['overview', 'conversation', 'files']
const tabMotion = ref('tab-next')
watch(reviewTab, (to, from) => {
  const side = TAB_ORDER.indexOf(to) > TAB_ORDER.indexOf(from) ? 'next' : 'prev'
  tabMotion.value = to === 'files' || from === 'files' ? `tab-lite-${side}` : `tab-${side}`
}, { flush: 'sync' })

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

const resolvingId = ref<number | null>(null)
const conversationPane = ref<HTMLElement | null>(null)
const unresolvedCount = computed(() => threads.value.filter((t) => !t.resolved).length)

async function handleResolve(rootId: number): Promise<void> {
  // Set for the one render in which the row leaves the list: resolveThread marks the thread before its first await.
  resolvingId.value = rootId
  await resolveThread(rootId)
  resolvingId.value = null
  if (unresolvedCount.value === 0) confetti(reviewTab.value === 'conversation' ? conversationPane.value : null)
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
    if (await loadPR()) await diffReviewRef.value?.reload()
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
// Separate from confirmingAction so the dialog keeps its text while it animates out.
const confirmOpen = ref(false)
const actionRunning = ref(false)
const actionError = ref('')
const actionFailures = ref(0)
const mergeButton = ref<HTMLElement | null>(null)
const mergedStamp = ref(false)
// Longer than the merged-stamp animation below, which ends invisible: a busy reload can start it late.
const STAMP_MS = 2600

function askAction(action: PRAction): void {
  actionError.value = ''
  confirmingAction.value = action
  confirmOpen.value = true
}

async function runAction(): Promise<void> {
  const action = confirmingAction.value
  if (!action) return
  actionRunning.value = true
  try {
    await (action === 'close' ? ClosePR() : MergePR(action))
    confirmOpen.value = false
    if (action !== 'close') {
      confetti(mergeButton.value, 160)
      mergedStamp.value = true
      setTimeout(() => { mergedStamp.value = false }, STAMP_MS)
    }
    toast.success(action === 'close' ? 'Pull request closed' : 'Pull request merged')
    await refreshPRView()
    if (action !== 'close' && toggles.value.return_after_merge) emit('close-review')
  } catch (err) {
    actionError.value = errorMessage(err)
    actionFailures.value++
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
  <div class="relative flex flex-col h-full">
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
      <!-- In the strip from the start, not once the PR has loaded: the list only holds open PRs, so an unknown state counts
           as open. It closes up once the PR is merged or closed. -->
      <Transition v-bind="collapseX('-ml-0.5!')">
        <span v-if="(prSummary?.state ?? prItem.state ?? 'open') === 'open'" class="ml-1" :class="collapsibleX">
          <span ref="mergeButton" class="min-w-0">
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button
                  variant="outline"
                  size="sm"
                  class="h-7 gap-1.5 px-2.5 text-xs border-emerald-500/40 bg-emerald-500/15 text-emerald-700 hover:bg-emerald-500/25 hover:text-emerald-800 dark:text-emerald-300 dark:hover:text-emerald-200"
                  aria-label="Merge or close pull request"
                  title="Merge or close pull request"
                  style="--wails-draggable: no-drag"
                >
                  <GitMerge class="h-3.5 w-3.5" />
                  Merge
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem :disabled="!prSummary || prSummary.is_draft" @select="askAction('merge')">{{ prActionLabels.merge }}</DropdownMenuItem>
                <DropdownMenuItem :disabled="!prSummary || prSummary.is_draft" @select="askAction('squash')">{{ prActionLabels.squash }}</DropdownMenuItem>
                <DropdownMenuItem :disabled="!prSummary || prSummary.is_draft" @select="askAction('rebase')">{{ prActionLabels.rebase }}</DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem class="text-destructive" :disabled="!prSummary" @select="askAction('close')">{{ prActionLabels.close }}</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </span>
        </span>
      </Transition>
    </Teleport>

    <Dialog :open="confirmOpen" @update:open="(open: boolean) => { if (!open && !actionRunning) confirmOpen = false }">
      <DialogContent class="max-w-sm">
        <DialogHeader>
          <DialogTitle>{{ confirmingAction && prActionLabels[confirmingAction] }}?</DialogTitle>
        </DialogHeader>
        <div>
          <p class="text-sm text-muted-foreground">
            {{ confirmingAction === 'close'
              ? `Close #${prSummary?.number} without merging.`
              : `${confirmingAction && prActionLabels[confirmingAction]} ${prSummary?.head_branch} into ${prSummary?.base_branch}. Fails if new commits were pushed since this PR was loaded.` }}
          </p>
          <Transition v-bind="collapse">
            <div v-if="actionError" :class="collapsible">
              <div class="min-h-0"><p :key="actionFailures" class="pt-2 text-sm text-destructive wow-shake">{{ actionError }}</p></div>
            </div>
          </Transition>
        </div>
        <DialogFooter>
          <Button variant="ghost" :disabled="actionRunning" @click="confirmOpen = false">Cancel</Button>
          <Button :variant="confirmingAction === 'close' ? 'destructive' : 'default'" class="relative" :class="actionRunning && 'wow-glow'" :disabled="actionRunning" @click="runAction()">
            <BusyLabel :busy="actionRunning">{{ confirmingAction && prActionLabels[confirmingAction] }}</BusyLabel>
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Loader, error and content share one cell, so the loader fades out over what replaces it. -->
    <div class="grid min-h-0 flex-1 grid-cols-1 grid-rows-1">
      <Transition leave-active-class="transition-opacity duration-fast pointer-events-none" leave-to-class="opacity-0">
        <!-- Clipped: a panel sliding in overshoots its place for a few frames. -->
        <div v-if="prSummary" class="grid min-h-0 min-w-0 grid-cols-1 grid-rows-1 overflow-hidden [grid-area:1/1]">
          <Transition :name="tabMotion">
            <div v-show="reviewTab === 'conversation'" class="flex min-h-0 min-w-0 overflow-hidden [grid-area:1/1]">
              <!-- Left: thread list with its own filter -->
              <div class="w-72 shrink-0 border-r border-border flex flex-col min-h-0">
                <div class="h-10 shrink-0 border-b border-border px-3 flex items-center gap-2">
                  <span class="min-w-0 truncate text-xs font-medium">Threads</span>
                  <!-- Equal columns, so the thumb can slide between them with a transform alone. -->
                  <div class="relative ml-auto grid shrink-0 grid-cols-2 whitespace-nowrap rounded-md bg-muted p-0.5 text-xs" role="group" aria-label="Thread filter">
                    <span
                      aria-hidden="true"
                      class="absolute inset-y-0.5 left-0.5 w-[calc(50%-0.125rem)] rounded bg-background shadow-sm transition-[translate] duration-fast ease-in-out"
                      :class="showResolved && 'translate-x-full'"
                    />
                    <button
                      class="relative rounded px-2 py-0.5 transition-colors duration-fast"
                      :class="!showResolved ? 'text-foreground' : 'text-muted-foreground hover:text-foreground'"
                      :aria-pressed="!showResolved"
                      @click="showResolved && toggleShowResolved()"
                    >Unresolved <CountUp :value="unresolvedCount" class="opacity-60" /></button>
                    <button
                      class="relative rounded px-2 py-0.5 transition-colors duration-fast"
                      :class="showResolved ? 'text-foreground' : 'text-muted-foreground hover:text-foreground'"
                      :aria-pressed="showResolved"
                      @click="!showResolved && toggleShowResolved()"
                    >All <CountUp :value="threads.length" class="opacity-60" /></button>
                  </div>
                </div>
                <div class="flex-1 min-h-0 overflow-y-auto">
                  <CommentSummaryList
                    :threads="queue"
                    :total="threads.length"
                    :review-numbers="overview?.review_numbers"
                    :current-index="currentIndex"
                    :show-resolved="showResolved"
                    :commands="commands"
                    :default-command-name="defaultCommandName"
                    :resolving-id="resolvingId"
                    @select="handleSelect"
                    @show-all="toggleShowResolved()"
                    @ran="handleRan"
                  />
                </div>
              </div>

              <!-- Right: thread pager, detail, run tools -->
              <div ref="conversationPane" class="flex flex-col flex-1 min-w-0 min-h-0 overflow-hidden">
                <!-- Stays when no thread is listed, so the detail below never jumps. -->
                <div class="h-10 shrink-0 border-b border-border px-4 flex items-center gap-1 text-xs">
                  <span class="text-muted-foreground tabular-nums">
                    <template v-if="queue.length > 0">Thread {{ currentIndex + 1 }} of <CountUp :value="queue.length" /></template>
                    <template v-else>No threads</template>
                  </span>
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
                  :index="currentIndex"
                  :is-at-end="isAtEnd && queue.length > 0"
                  :all-resolved="threads.length > 0 && unresolvedCount === 0"
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
                    <!-- Opens sideways so the label glides aside instead of jumping when a run starts or ends. -->
                    <Transition v-bind="collapseX('-ml-1.5!')">
                      <span v-if="pendingRunCount > 0" :class="collapsibleX">
                        <span class="min-w-0">
                          <span class="rounded-full bg-primary px-1.5 text-[0.625rem] text-primary-foreground tabular-nums">{{ pendingRunCount }}</span>
                        </span>
                      </span>
                    </Transition>
                    <ChevronUp class="h-3.5 w-3.5 transition-transform" :class="runPanelOpen && 'rotate-180'" />
                  </Button>
                </div>
              </div>
            </div>
          </Transition>

          <Transition :name="tabMotion">
            <PROverviewPanel
              v-show="reviewTab === 'overview'"
              v-model:show-resolved-reviews="showResolvedReviews"
              :overview="overview"
              class="min-h-0 min-w-0 [grid-area:1/1]"
            />
          </Transition>

          <Transition :name="tabMotion">
            <DiffReviewView
              v-show="reviewTab === 'files'"
              ref="diffReviewRef"
              :threads="threads"
              v-model:show-threads="showDiffThreads"
              :is-own-pr="isOwnPR"
              :head-sha="prSummary?.head_sha ?? ''"
              :active="reviewTab === 'files'"
              :unsent-comments="unsentComments"
              class="min-h-0 min-w-0 overflow-hidden [grid-area:1/1]"
              @resolve="handleResolve"
              @unresolve="handleUnresolve"
              @reply-sent="handleReplySent"
              @threads-changed="handleReviewActivity"
            />
          </Transition>
        </div>

        <div v-else-if="error" class="[grid-area:1/1] flex flex-col items-center justify-center gap-3 p-6 animate-in fade-in-0 animation-duration-base">
          <p class="text-sm text-destructive text-center wow-shake">{{ error }}</p>
          <Button variant="outline" size="sm" :disabled="loading" @click="loadPR()">
            <BusyLabel :busy="loading">Retry</BusyLabel>
          </Button>
        </div>

        <div v-else class="relative overflow-hidden [grid-area:1/1] grid place-items-center">
          <!-- Delayed, so a fast load goes straight to the content. -->
          <div aria-hidden="true" class="absolute inset-0 animate-in fade-in-0 animation-duration-show delay-200 fill-mode-backwards"><div class="wow-aurora opacity-50" /></div>
          <div class="relative flex flex-col items-center gap-3 text-muted-foreground animate-in fade-in-0 animation-duration-base delay-200 fill-mode-backwards">
            <RefreshCw class="h-6 w-6 animate-spin" />
            <div class="text-sm">Loading review…</div>
            <div class="text-xs tabular-nums" :class="{ invisible: loadProgress.loaded === 0 }">
              <CountUp :value="loadProgress.loaded" /> thread{{ loadProgress.loaded !== 1 ? 's' : '' }} loaded
              <span v-if="loadProgress.total > 0"> of {{ loadProgress.total }}</span>
            </div>
          </div>
        </div>
      </Transition>
    </div>

    <div v-if="mergedStamp" aria-hidden="true" class="pointer-events-none absolute inset-0 z-50 grid place-items-center wow-sheen">
      <div class="merged-stamp relative flex items-center gap-4 rounded-2xl border-4 border-wow-1 bg-background/80 px-10 py-5 text-5xl font-black tracking-widest text-wow-1 uppercase wow-ring">
        <GitMerge class="size-12" />
        Merged
      </div>
    </div>
  </div>
</template>

<style scoped>
.tab-next-enter-active,
.tab-prev-enter-active,
.tab-lite-next-enter-active,
.tab-lite-prev-enter-active {
  transition: opacity var(--transition-duration-base) var(--ease-out), transform var(--transition-duration-slower) var(--ease-spring);
}
.tab-next-enter-from {
  opacity: 0;
  transform: perspective(75rem) translateX(5rem) rotateY(-14deg) scale(0.94);
}
.tab-prev-enter-from {
  opacity: 0;
  transform: perspective(75rem) translateX(-5rem) rotateY(14deg) scale(0.94);
}
.tab-lite-next-enter-from {
  opacity: 0;
  transform: translateX(3rem);
}
.tab-lite-prev-enter-from {
  opacity: 0;
  transform: translateX(-3rem);
}
.tab-next-leave-active,
.tab-prev-leave-active {
  pointer-events: none;
  transition: opacity var(--transition-duration-fast) var(--ease-out), transform var(--transition-duration-fast) var(--ease-out);
}
.tab-next-leave-to {
  opacity: 0;
  transform: perspective(75rem) translateX(-3rem) translateZ(-10rem);
}
.tab-prev-leave-to {
  opacity: 0;
  transform: perspective(75rem) translateX(3rem) translateZ(-10rem);
}
.tab-lite-next-leave-active,
.tab-lite-prev-leave-active {
  display: none;
}

/* Slams down from in front of the screen, holds, and lifts off; its ring bursts on impact. */
.merged-stamp {
  animation: merged-stamp 1900ms var(--ease-out) both;
}
.merged-stamp::after {
  animation-delay: 300ms;
}
@keyframes merged-stamp {
  0% { opacity: 0; transform: scale(4) rotate(-20deg); }
  16% { opacity: 1; transform: scale(0.9) rotate(-7deg); }
  22% { transform: scale(1.05) rotate(-7deg); }
  28%, 80% { opacity: 1; transform: scale(1) rotate(-7deg); }
  100% { opacity: 0; transform: scale(1.1) rotate(-7deg); }
}
</style>
