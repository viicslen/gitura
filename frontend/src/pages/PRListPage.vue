<template>
  <div class="h-full overflow-y-auto flex gap-6 items-start p-6">
    <!-- ── Filter sidebar ───────────────────────────────────────────────── -->
    <aside class="w-64 shrink-0 space-y-6 sticky top-0 self-start max-h-screen overflow-y-auto" aria-label="PR filters">
      <!-- Involvement types -->
      <section>
        <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          Involvement
        </h3>
        <div class="space-y-2">
          <label class="flex items-center gap-2 cursor-pointer select-none">
            <Checkbox
              :model-value="filters.includeAuthor.value"
              :disabled="filters.isOnlyActiveInvolvement('author')"
              aria-label="Include PRs I authored"
              @update:model-value="filters.toggleAuthor()"
            />
            <span class="text-sm">Author</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer select-none">
            <Checkbox
              :model-value="filters.includeAssignee.value"
              :disabled="filters.isOnlyActiveInvolvement('assignee')"
              aria-label="Include PRs assigned to me"
              @update:model-value="filters.toggleAssignee()"
            />
            <span class="text-sm">Assignee</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer select-none">
            <Checkbox
              :model-value="filters.includeReviewer.value"
              :disabled="filters.isOnlyActiveInvolvement('reviewer')"
              aria-label="Include PRs where I am a requested reviewer"
              @update:model-value="filters.toggleReviewer()"
            />
            <span class="text-sm">Reviewer</span>
          </label>
        </div>
      </section>

      <Separator />

      <!-- Repository filter (searchable from loaded results) -->
      <section>
        <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          Repository
          <button
            class="ml-1 text-xs text-muted-foreground hover:text-foreground transition-[color,opacity,visibility]"
            :class="filters.repo.value ? 'duration-fast' : 'invisible opacity-0 duration-faster'"
            aria-label="Clear repository filter"
            @click="filters.clearRepo()"
          >
            ✕
          </button>
        </h3>
        <div class="relative">
          <Input
            v-model="repoSearch"
            placeholder="Search repos…"
            aria-label="Filter by repository"
            class="h-8 text-sm"
            @focus="repoDropdownOpen = true"
            @blur="onRepoBlur"
          />
          <Transition
            enter-active-class="animate-in fade-in-0 zoom-in-95 animation-duration-fast ease-out"
            leave-active-class="animate-out fade-out-0 zoom-out-95 animation-duration-faster ease-out"
          >
            <ul
              v-if="repoDropdownOpen && filteredRepoOptions.length > 0"
              class="absolute z-10 mt-1 w-full origin-top rounded-md border border-border bg-popover shadow-md max-h-48 overflow-auto text-sm"
              role="listbox"
              aria-label="Repository options"
            >
              <li
                v-for="opt in filteredRepoOptions"
                :key="opt"
                class="cursor-pointer px-3 py-1.5 hover:bg-accent"
                role="option"
                :aria-selected="filters.repo.value === opt"
                @mousedown.prevent="selectRepo(opt)"
              >
                {{ opt }}
              </li>
            </ul>
          </Transition>
        </div>
      </section>

      <!-- Organisation filter (searchable from loaded results) -->
      <section>
        <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          Organisation
          <button
            class="ml-1 text-xs text-muted-foreground hover:text-foreground transition-[color,opacity,visibility]"
            :class="filters.org.value ? 'duration-fast' : 'invisible opacity-0 duration-faster'"
            aria-label="Clear organisation filter"
            @click="filters.clearOrg()"
          >
            ✕
          </button>
        </h3>
        <div class="relative">
          <Input
            v-model="orgSearch"
            placeholder="Search orgs…"
            aria-label="Filter by organisation"
            class="h-8 text-sm"
            @focus="orgDropdownOpen = true"
            @blur="onOrgBlur"
          />
          <Transition
            enter-active-class="animate-in fade-in-0 zoom-in-95 animation-duration-fast ease-out"
            leave-active-class="animate-out fade-out-0 zoom-out-95 animation-duration-faster ease-out"
          >
            <ul
              v-if="orgDropdownOpen && filteredOrgOptions.length > 0"
              class="absolute z-10 mt-1 w-full origin-top rounded-md border border-border bg-popover shadow-md max-h-48 overflow-auto text-sm"
              role="listbox"
              aria-label="Organisation options"
            >
              <li
                v-for="opt in filteredOrgOptions"
                :key="opt"
                class="cursor-pointer px-3 py-1.5 hover:bg-accent"
                role="option"
                :aria-selected="filters.org.value === opt"
                @mousedown.prevent="selectOrg(opt)"
              >
                {{ opt }}
              </li>
            </ul>
          </Transition>
        </div>
      </section>

      <!-- Author filter -->
      <section>
        <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          PR Author
          <button
            class="ml-1 text-xs text-muted-foreground hover:text-foreground transition-[color,opacity,visibility]"
            :class="filters.author.value ? 'duration-fast' : 'invisible opacity-0 duration-faster'"
            aria-label="Clear author filter"
            @click="filters.clearAuthor()"
          >
            ✕
          </button>
        </h3>
        <Input
          v-model="filters.author.value"
          placeholder="github-login"
          aria-label="Filter by PR author"
          class="h-8 text-sm"
          @input="rowMotionOnce('cut')"
        />
      </section>

      <!-- Updated after filter -->
      <section>
        <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          Updated after
          <button
            class="ml-1 text-xs text-muted-foreground hover:text-foreground transition-[color,opacity,visibility]"
            :class="filters.updatedAfter.value ? 'duration-fast' : 'invisible opacity-0 duration-faster'"
            aria-label="Clear date filter"
            @click="filters.clearUpdatedAfter()"
          >
            ✕
          </button>
        </h3>
        <Input
          v-model="filters.updatedAfter.value"
          type="date"
          aria-label="Filter by updated date"
          class="h-8 text-sm"
          @input="rowMotionOnce('cut')"
        />
      </section>

      <Separator />

      <!-- Include drafts toggle -->
      <section class="flex items-center justify-between">
        <span class="text-sm">Include drafts</span>
        <Switch
          :model-value="filters.includeDrafts.value"
          aria-label="Include draft pull requests"
          @update:model-value="filters.toggleDrafts()"
        />
      </section>

      <!-- Clear all -->
      <div
        class="transition-[opacity,translate,visibility]"
        :class="filters.hasActiveFilters.value ? 'duration-base' : 'invisible opacity-0 translate-y-1 duration-fast'"
      >
        <Button
          variant="ghost"
          size="sm"
          class="w-full text-muted-foreground"
          @click="filters.clearAllFilters()"
        >
          Clear all filters
        </Button>
      </div>
    </aside>

    <!-- ── PR list ──────────────────────────────────────────────────────── -->
    <main class="flex-1 min-w-0" aria-label="Pull request list">
      <!-- Toolbar: count + refresh -->
      <div class="flex items-center justify-between mb-4">
        <p class="text-sm text-muted-foreground tabular-nums" aria-live="polite">
          <Transition enter-active-class="animate-in fade-in-0 animation-duration-base">
            <span v-if="rawResult">
              {{ visibleItems.length }} {{ visibleItems.length === 1 ? 'pull request' : 'pull requests' }}
              <span v-if="rawResult.incomplete_results" class="ml-1 text-yellow-500">(incomplete)</span>
            </span>
          </Transition>
        </p>
        <div class="flex items-center gap-1">
          <Button variant="outline" size="sm" class="gap-1.5" @click="openLocalReview()">
            <FolderGit2 class="h-4 w-4" />
            Review local changes
          </Button>
          <Button
            variant="ghost"
            size="sm"
            :disabled="loading"
            aria-label="Refresh pull request list"
            @click="fetchPRs()"
          >
            <RefreshCw :class="['h-4 w-4', spinning && 'animate-spin']" @animationiteration="onSpinTurn" />
          </Button>
        </div>
      </div>

      <!-- Error state: opens and closes by height so the content below glides -->
      <Transition
        enter-active-class="transition-[grid-template-rows,opacity] duration-slow"
        enter-from-class="grid-rows-[0fr]! opacity-0"
        leave-active-class="transition-[grid-template-rows,opacity] duration-base"
        leave-to-class="grid-rows-[0fr]! opacity-0"
      >
        <div v-if="errorMsg" class="grid grid-rows-[1fr]" role="alert">
          <div class="min-h-0 overflow-hidden">
            <div class="mb-4 rounded-lg border border-destructive/30 bg-destructive/10 p-5">
              <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div class="space-y-1">
                  <p class="text-sm font-semibold text-destructive">Could not load pull requests</p>
                  <div class="grid *:col-start-1 *:row-start-1">
                    <Transition
                      enter-active-class="animate-in fade-in-0 animation-duration-base"
                      leave-active-class="animate-out fade-out-0 animation-duration-fast"
                    >
                      <p
                        :key="errorText"
                        class="text-sm"
                        :class="rateLimitReset ? 'text-yellow-700 dark:text-yellow-300' : 'text-muted-foreground'"
                      >
                        {{ errorText }}
                      </p>
                    </Transition>
                  </div>
                </div>
                <Button
                  size="lg"
                  :disabled="loading"
                  @click="fetchPRs()"
                >
                  <RefreshCw :class="['mr-2 h-4 w-4', spinning && 'animate-spin']" @animationiteration="onSpinTurn" />
                  Refresh pull requests
                </Button>
              </div>
            </div>
          </div>
        </div>
      </Transition>

      <!-- A refresh keeps the current content, dimmed once the wait is real -->
      <div class="relative transition-opacity" :class="loading && rawResult && 'opacity-60 delay-200'">
        <Transition
          :css="rowMotion !== 'cut'"
          enter-active-class="animate-in fade-in-0 animation-duration-base"
          leave-active-class="absolute inset-x-0 top-0 animate-none! transition-opacity duration-fast delay-0!"
          leave-to-class="opacity-0!"
          @before-leave="freezeOpacity"
        >
          <!-- Skeleton: first load only, and only once the wait is real -->
          <div
            v-if="!rawResult && loading"
            key="skeleton"
            class="rounded-lg border border-border bg-card divide-y divide-border animate-in fade-in-0 animation-duration-base delay-200 fill-mode-backwards"
            aria-label="Loading pull requests"
          >
            <div v-for="n in 6" :key="n" class="flex items-start gap-3 px-4 py-3">
              <Skeleton class="h-4 w-4 mt-0.5 shrink-0 rounded-full" />
              <div class="flex-1">
                <Skeleton class="h-4 w-3/4 my-0.5" />
                <Skeleton class="h-3 w-1/2 mt-2 mb-0.5" />
              </div>
            </div>
          </div>

          <!-- Empty: no PRs at all (no filters active) -->
          <div
            v-else-if="rawResult && visibleItems.length === 0 && !filters.hasActiveFilters.value"
            key="empty"
            class="flex flex-col items-center justify-center py-20 text-center gap-3"
          >
            <GitPullRequest class="h-10 w-10 text-muted-foreground" />
            <p class="text-sm text-muted-foreground">No open pull requests found.</p>
            <Button
              size="lg"
              variant="outline"
              :disabled="loading"
              @click="fetchPRs()"
            >
              <RefreshCw :class="['mr-2 h-4 w-4', spinning && 'animate-spin']" @animationiteration="onSpinTurn" />
              Refresh pull requests
            </Button>
          </div>

          <!-- Empty: filters active but no results -->
          <div
            v-else-if="rawResult && visibleItems.length === 0"
            key="no-match"
            class="flex flex-col items-center justify-center py-20 text-center gap-3"
          >
            <Filter class="h-10 w-10 text-muted-foreground" />
            <p class="text-sm text-muted-foreground">No pull requests match the current filters.</p>
            <Button variant="ghost" size="sm" @click="filters.clearAllFilters()">
              Clear filters
            </Button>
          </div>

          <!-- PR rows -->
          <TransitionGroup
            v-else-if="visibleItems.length > 0"
            key="rows"
            ref="rowsEl"
            tag="ul"
            v-bind="rowMotionProps"
            class="relative rounded-lg border border-border bg-card divide-y divide-border overflow-hidden transition-[height] ease-in-out"
            role="list"
            @transitionend.self="rowsHeightDone"
            @animationend.self="rowsFadeDone"
          >
            <li
              v-for="pr in visibleItems"
              :key="pr.html_url"
              :class="changedRows.has(pr.html_url) && 'pr-row-flash'"
              @animationend.self="changedRows.delete(pr.html_url)"
            >
              <button
                class="w-full text-left flex items-start gap-3 px-4 py-3 hover:bg-accent/50 transition-colors duration-faster hover:duration-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring cursor-pointer"
                @click="openPR(pr)"
              >
                <GitPullRequestDraft v-if="pr.is_draft" class="h-4 w-4 mt-0.5 shrink-0 text-muted-foreground" aria-hidden="true" />
                <GitPullRequest v-else class="h-4 w-4 mt-0.5 shrink-0 text-emerald-600 dark:text-emerald-400" aria-hidden="true" />
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2 min-w-0">
                    <span class="text-sm font-medium truncate" :title="pr.title">{{ pr.title }}</span>
                    <Badge v-if="pr.is_draft" variant="secondary" class="shrink-0 text-xs">
                      Draft
                    </Badge>
                  </div>
                  <div class="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-xs text-muted-foreground min-w-0">
                    <span class="truncate max-w-full" :title="`${pr.owner}/${pr.repo} #${pr.number}`">{{ pr.owner }}/{{ pr.repo }} #{{ pr.number }}</span>
                    <span aria-hidden="true">·</span>
                    <span class="truncate max-w-full">{{ pr.author_login }}</span>
                    <span aria-hidden="true">·</span>
                    <time :datetime="pr.updated_at" :title="new Date(pr.updated_at).toLocaleString()">
                      updated {{ formatAge(pr.updated_at) }}
                    </time>
                  </div>
                </div>
              </button>
            </li>
          </TransitionGroup>
        </Transition>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { errorMessage } from '@/lib/utils'
import { ref, computed, nextTick, onMounted, onActivated, watch } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useLocalStorage } from '@vueuse/core'
import { toast } from 'vue-sonner'
import { RefreshCw, GitPullRequest, GitPullRequestDraft, Filter, FolderGit2 } from '@lucide/vue'
import { Separator } from '@/components/ui/separator'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import { usePRFilters } from '@/composables/usePRFilters'
import * as App from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'
import type { PRRef } from '@/types/pr'

const props = defineProps<{
  /** Latest PR data seen in the review view, keyed `owner/repo#number`; overrides search results until search catches up. */
  prUpdates?: Map<string, PRRef>
}>()

const emit = defineEmits<{
  (e: 'open-review', item: PRRef): void
  (e: 'open-local', path: string): void
}>()

const lastLocalDir = useLocalStorage('gitura.lastLocalDir', '')

async function openLocalReview(): Promise<void> {
  try {
    const path = await App.OpenFolderPicker('Select a local repository', lastLocalDir.value)
    if (!path) return
    lastLocalDir.value = path
    emit('open-local', path)
  } catch (err) {
    toast.error('Could not open folder picker: ' + errorMessage(err))
  }
}

// ── State ─────────────────────────────────────────────────────────────────────

// Starts true so the first paint is the loading state, not the empty one.
const loading = ref(true)
const errorMsg = ref<string | null>(null)
const rateLimitReset = ref<string | null>(null)

/** Raw unfiltered result from the backend (involves: query). */
const rawResult = ref<model.PRListResult | null>(null)
const errorText = computed<string>(() => {
  if (rateLimitReset.value) return `GitHub rate limit reached. Resets at ${formatReset(rateLimitReset.value)}.`
  if (errorMsg.value?.toLowerCase().includes('github service unavailable')) return 'GitHub is temporarily unavailable. This is usually transient.'
  return errorMsg.value ?? ''
})

// The refresh icon finishes its turn after the answer arrives, so it never snaps back mid-rotation.
const spinning = ref(true)
watch(loading, (on) => { if (on) spinning.value = true })
function onSpinTurn(): void {
  if (!loading.value) spinning.value = false
}

// ── Row motion ────────────────────────────────────────────────────────────────

/** glide: rows fade and the rest move (the user's own change). settle: new rows fade in, nothing moves (background refresh). cut: no motion (typing). */
type RowMotion = 'glide' | 'settle' | 'cut'
const rowMotion = ref<RowMotion>('glide')
const rowMotionProps = computed(() => {
  if (rowMotion.value === 'cut' || tooManyRows()) return { css: false }
  if (rowMotion.value === 'settle') {
    return { enterActiveClass: 'transition-opacity', enterFromClass: 'opacity-0', leaveActiveClass: 'hidden' }
  }
  return {
    // Spelled differently from the enter and leave classes: Vue strips the move classes when the move ends.
    moveClass: 'transition-[transform,opacity] ease-in-out',
    enterActiveClass: 'transition-[opacity,transform]',
    enterFromClass: 'opacity-0',
    leaveActiveClass: 'absolute inset-x-0 transition-[opacity,transform] duration-fast',
    leaveToClass: 'opacity-0',
  }
})

// Moving rows one by one drops a frame somewhere between 100 and 200 rows; a longer list changes in one frame and fades in as a block.
const maxGlidingRows = 100
let renderedRows = 0
function tooManyRows(): boolean {
  return Math.max(renderedRows, visibleItems.value.length) > maxGlidingRows
}
const blockFade = ['animate-in', 'fade-in-0', 'animation-duration-base']

/** Applies `mode` to the list change made in this tick only. */
function rowMotionOnce(mode: RowMotion): void {
  rowMotion.value = mode
  void nextTick(() => { rowMotion.value = 'glide' })
}

const rowsEl = ref<ComponentPublicInstance | null>(null)

/** Rows whose title, draft state or update time changed in the last refresh; they flash once. */
const changedRows = ref(new Set<string>())

function freezeOpacity(el: Element): void {
  (el as HTMLElement).style.opacity = getComputedStyle(el).opacity
}

function rowsHeightDone(e: TransitionEvent): void {
  if (e.propertyName === 'height') (e.currentTarget as HTMLElement).style.height = ''
}

function rowsFadeDone(e: AnimationEvent): void {
  (e.currentTarget as HTMLElement).classList.remove(...blockFade)
}

// ── Filters composable ────────────────────────────────────────────────────────

// fetchPRs is referenced before its definition; hoisting is fine in JS modules.
const filters = usePRFilters(fetchPRs)

// ── Derived: client-side filtered items ──────────────────────────────────────

/**
 * Applies all active client-side filters (involvement toggles, repo, org,
 * author, date) to the raw result. Recomputes instantly on any filter change.
 */
const visibleItems = computed<model.PRListItem[]>(() => {
  if (!rawResult.value?.items) return []
  const updates = props.prUpdates
  const items = !updates?.size ? rawResult.value.items : rawResult.value.items.flatMap((pr) => {
    const update = updates.get(prKey(pr))
    if (!update) return [pr]
    if (isClosed(update)) return []
    return [{ ...pr, title: update.title, is_draft: update.is_draft ?? pr.is_draft }]
  })
  return filters.applyFilters(items)
})

// The card's bottom edge follows its rows: animate the list from its old height to the new one.
watch(visibleItems, (items) => {
  const ul = rowsEl.value?.$el as HTMLElement | undefined
  const block = tooManyRows()
  void nextTick(() => { renderedRows = items.length })
  if (!ul || rowMotion.value !== 'glide') return
  if (block) {
    ul.classList.remove(...blockFade)
    void nextTick(() => ul.classList.add(...blockFade))
    return
  }
  const from = ul.offsetHeight
  void nextTick(() => {
    if (!ul.isConnected) return
    ul.style.height = ''
    const to = ul.offsetHeight
    if (to === from) return
    ul.style.height = `${from}px`
    void ul.offsetHeight
    ul.style.height = `${to}px`
  })
}, { flush: 'pre' })

// ── Derived: repo options from raw result ─────────────────────────────────────

/** Distinct sorted repo names derived from the raw (unfiltered) result. */
const repoOptions = computed<string[]>(() => {
  if (!rawResult.value?.items) return []
  const seen = new Set<string>()
  rawResult.value.items.forEach((pr) => seen.add(`${pr.owner}/${pr.repo}`))
  return Array.from(seen).sort()
})

// ── Repo combobox state ───────────────────────────────────────────────────────

const repoSearch = ref('')
const repoDropdownOpen = ref(false)

const filteredRepoOptions = computed(() => {
  const q = repoSearch.value.toLowerCase()
  if (!q) return repoOptions.value
  return repoOptions.value.filter((r) => r.toLowerCase().includes(q))
})

function selectRepo(fullName: string): void {
  filters.pickRepo(fullName)
  repoSearch.value = fullName
  repoDropdownOpen.value = false
}

function onRepoBlur(): void {
  repoDropdownOpen.value = false
  const known = findOption(repoOptions.value, repoSearch.value)
  if (known) selectRepo(known)
  else if (repoSearch.value !== filters.repo.value) filters.repo.value = repoSearch.value.trim()
}

/** Typed text naming a loaded option counts as picking it, so exact matching is keyboard-reachable. */
function findOption(options: string[], typed: string): string | undefined {
  const q = typed.trim().toLowerCase()
  return q ? options.find((o) => o.toLowerCase() === q) : undefined
}

// Keep repoSearch in sync when filters.repo is cleared externally.
watch(filters.repo, (val) => {
  if (val === '') repoSearch.value = ''
})

// ── Derived: org options from raw result ──────────────────────────────────────

/** Distinct sorted org names (owner portion) derived from the raw (unfiltered) result. */
const orgOptions = computed<string[]>(() => {
  if (!rawResult.value?.items) return []
  const seen = new Set<string>()
  rawResult.value.items.forEach((pr) => seen.add(pr.owner))
  return Array.from(seen).sort()
})

// ── Org combobox state ────────────────────────────────────────────────────────

const orgSearch = ref('')
const orgDropdownOpen = ref(false)

const filteredOrgOptions = computed(() => {
  const q = orgSearch.value.toLowerCase()
  if (!q) return orgOptions.value
  return orgOptions.value.filter((o) => o.toLowerCase().includes(q))
})

function selectOrg(name: string): void {
  filters.pickOrg(name)
  orgSearch.value = name
  orgDropdownOpen.value = false
}

function onOrgBlur(): void {
  orgDropdownOpen.value = false
  const known = findOption(orgOptions.value, orgSearch.value)
  if (known) selectOrg(known)
  else if (orgSearch.value !== filters.org.value) filters.org.value = orgSearch.value.trim()
}

// Keep orgSearch in sync when filters.org is cleared externally.
watch(filters.org, (val) => {
  if (val === '') orgSearch.value = ''
})

// ── Data fetching ─────────────────────────────────────────────────────────────

/**
 * Fetch all open PRs involving the user (single involves: query).
 * All filters except include_drafts are applied client-side after this call.
 * Uses requestId to discard stale responses from superseded requests (CHK010/CHK011).
 */
async function fetchPRs(background = false): Promise<void> {
  const thisId = filters.requestId.value

  loading.value = !background

  try {
    // Only include_drafts is meaningful server-side; other filter fields are ignored.
    const res = await App.ListOpenPRs(filters.toFilters())

    if (thisId !== filters.requestId.value) return

    // A failed refresh carries no items; what is on screen stays.
    if (!res.error) {
      const before = new Map(rawResult.value?.items?.map((pr) => [pr.html_url, pr]))
      changedRows.value = new Set(res.items?.filter((pr) => {
        const old = before.get(pr.html_url)
        return old && (old.title !== pr.title || old.is_draft !== pr.is_draft || old.updated_at !== pr.updated_at)
      }).map((pr) => pr.html_url))
      if (background) rowMotionOnce('settle')
      rawResult.value = res
      // GitHub's search index lags edits and merges; drop an update once search reflects it.
      const returned = new Map(res.items?.map((pr) => [prKey(pr), pr]))
      props.prUpdates?.forEach((update, key) => {
        const pr = returned.get(key)
        if (!pr || (!isClosed(update) && pr.title === update.title && pr.is_draft === (update.is_draft ?? pr.is_draft))) {
          props.prUpdates?.delete(key)
        }
      })
    }

    // The previous error stays on screen until the retry answers.
    errorMsg.value = res.error || null
    rateLimitReset.value = res.rate_limit_reset || null
  } catch (e) {
    if (thisId !== filters.requestId.value) return
    errorMsg.value = errorMessage(e)
    rateLimitReset.value = null
  } finally {
    if (thisId === filters.requestId.value) {
      loading.value = false
    }
  }
}

function prKey(pr: { owner: string; repo: string; number: number }): string {
  return `${pr.owner}/${pr.repo}#${pr.number}`
}

function isClosed(pr: PRRef): boolean {
  return !!pr.state && pr.state !== 'open'
}

// ── Navigation ────────────────────────────────────────────────────────────────

function openPR(pr: model.PRListItem): void {
  emit('open-review', {
    owner: pr.owner,
    repo: pr.repo,
    number: pr.number,
    title: pr.title,
    state: 'open',
    is_draft: pr.is_draft,
    html_url: pr.html_url,
  })
}

// ── Formatting helpers ────────────────────────────────────────────────────────

function formatAge(updatedAt: string): string {
  const date = new Date(updatedAt)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
  if (diffDays === 0) return 'today'
  if (diffDays === 1) return 'yesterday'
  if (diffDays < 30) return `${diffDays}d ago`
  const diffMonths = Math.floor(diffDays / 30)
  if (diffMonths < 12) return `${diffMonths}mo ago`
  return `${Math.floor(diffMonths / 12)}y ago`
}

function formatReset(reset: string): string {
  return new Date(reset).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

// ── Lifecycle ─────────────────────────────────────────────────────────────────

onMounted(() => {
  void fetchPRs()
})

// KeepAlive also fires onActivated on first mount, which onMounted already covers.
let activatedOnce = false
onActivated(() => {
  if (activatedOnce) void fetchPRs(true)
  activatedOnce = true
})
</script>

<style scoped>
.pr-row-flash {
  animation: pr-row-flash var(--transition-duration-slower) var(--ease-out);
}
@keyframes pr-row-flash {
  from { background-color: var(--color-accent); }
}
</style>
