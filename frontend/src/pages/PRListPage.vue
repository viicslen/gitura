<template>
  <div class="h-full overflow-y-auto flex gap-6 items-start p-6">
    <!-- ── Filter sidebar ───────────────────────────────────────────────── -->
    <!-- The last group (Clear all) stays out of the cascade: a rise starts below its place, which would make the sidebar scrollable. -->
    <aside
      class="w-64 shrink-0 space-y-6 sticky top-0 self-start max-h-screen overflow-y-auto overflow-x-hidden"
      :class="arriving && '*:not-last:wow-rise'"
      aria-label="PR filters"
    >
      <!-- Involvement types -->
      <section class="[--i:1]">
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
            <CountUp :value="involved.author" class="ml-auto text-xs text-muted-foreground" />
          </label>
          <label class="flex items-center gap-2 cursor-pointer select-none">
            <Checkbox
              :model-value="filters.includeAssignee.value"
              :disabled="filters.isOnlyActiveInvolvement('assignee')"
              aria-label="Include PRs assigned to me"
              @update:model-value="filters.toggleAssignee()"
            />
            <span class="text-sm">Assignee</span>
            <CountUp :value="involved.assignee" class="ml-auto text-xs text-muted-foreground" />
          </label>
          <label class="flex items-center gap-2 cursor-pointer select-none">
            <Checkbox
              :model-value="filters.includeReviewer.value"
              :disabled="filters.isOnlyActiveInvolvement('reviewer')"
              aria-label="Include PRs where I am a requested reviewer"
              @update:model-value="filters.toggleReviewer()"
            />
            <span class="text-sm">Reviewer</span>
            <CountUp :value="involved.reviewer" class="ml-auto text-xs text-muted-foreground" />
          </label>
        </div>
      </section>

      <Separator class="[--i:2]" />

      <!-- Repository filter (searchable from loaded results) -->
      <section class="[--i:3]">
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
      <section class="[--i:4]">
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
      <section class="[--i:5]">
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
      <section class="[--i:6]">
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

      <Separator class="[--i:7]" />

      <!-- Include drafts toggle -->
      <section class="flex items-center justify-between [--i:8]">
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
        :class="filters.hasActiveFilters.value ? 'duration-base' : 'invisible opacity-0 -translate-y-1 duration-fast'"
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
        <p class="text-sm text-muted-foreground" aria-live="polite">
          <Transition enter-active-class="animate-in fade-in-0 animation-duration-base">
            <span v-if="rawResult">
              <!-- The rolling digits are for the eye; a screen reader gets the final count once. -->
              <span class="sr-only">{{ visibleItems.length }} {{ countNoun }}</span>
              <span aria-hidden="true">
                <CountUp :value="visibleItems.length" class="text-base font-semibold text-foreground" />
                {{ countNoun }}
              </span>
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
            class="group/refresh"
            :disabled="loading"
            aria-label="Refresh pull request list"
            @click="refresh()"
          >
            <RefreshCw :class="[refreshIcon, spinning && 'animate-spin']" @animationiteration="onSpinTurn" />
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
        <div v-if="errorMsg" class="grid grid-rows-[1fr]" :class="shake && 'wow-shake'" role="alert" @animationend.self="shake = false">
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
                  class="group/refresh"
                  :disabled="loading"
                  @click="refresh()"
                >
                  <RefreshCw :class="[refreshIcon, 'mr-2', spinning && 'animate-spin']" @animationiteration="onSpinTurn" />
                  Refresh pull requests
                </Button>
              </div>
            </div>
          </div>
        </div>
      </Transition>

      <div class="group/list relative" @pointermove.passive="trackPointer">
        <!-- A refresh keeps the current content, dimmed once the wait is real -->
        <div class="transition-opacity" :class="loading && rawResult && 'opacity-60 delay-200'">
          <Transition
            :css="rowMotion !== 'cut'"
            enter-active-class="animate-in fade-in-0 zoom-in-95 animation-duration-slower ease-spring"
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
              <div v-for="n in 6" :key="n" class="flex items-start gap-3 px-4 py-3" :style="{ '--i': n }">
                <Skeleton :class="[wave, 'h-4 w-4 mt-0.5 shrink-0 rounded-full']" />
                <div class="flex-1">
                  <Skeleton :class="[wave, 'h-4 w-3/4 my-0.5']" />
                  <Skeleton :class="[wave, 'h-3 w-1/2 mt-2 mb-0.5']" />
                </div>
              </div>
            </div>

            <!-- Empty: no PRs at all, or none matching the active filters -->
            <div
              v-else-if="rawResult && visibleItems.length === 0"
              :key="filters.hasActiveFilters.value ? 'no-match' : 'empty'"
              class="relative flex flex-col items-center justify-center gap-3 overflow-hidden rounded-lg py-20 text-center"
            >
              <div aria-hidden="true" class="wow-aurora opacity-60" />
              <div class="relative" :class="arriving && 'wow-focus-in'">
                <div class="pr-float grid size-20 place-items-center rounded-full border border-border bg-card/70 shadow-lg shadow-wow-1/20">
                  <component :is="filters.hasActiveFilters.value ? Filter : GitPullRequest" class="size-9 text-wow-1" />
                </div>
              </div>
              <template v-if="filters.hasActiveFilters.value">
                <p class="relative text-sm text-muted-foreground">No pull requests match the current filters.</p>
                <Button variant="ghost" size="sm" class="relative" @click="filters.clearAllFilters()">
                  Clear filters
                </Button>
              </template>
              <template v-else>
                <p class="relative text-sm text-muted-foreground">No open pull requests found.</p>
                <Button
                  size="lg"
                  variant="outline"
                  class="group/refresh relative"
                  :disabled="loading"
                  @click="refresh()"
                >
                  <RefreshCw :class="[refreshIcon, 'mr-2', spinning && 'animate-spin']" @animationiteration="onSpinTurn" />
                  Refresh pull requests
                </Button>
              </template>
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
                v-for="(pr, i) in visibleItems"
                :key="pr.html_url"
                :data-pr-key="prKey(pr)"
                class="group/row relative hover:z-10"
                :class="[arriving && i < 15 && 'wow-rise', changedRows.has(pr.html_url) && 'z-10 wow-ring']"
                :style="{ '--i': i }"
                @animationend.self="$event.animationName === 'wow-ring' && changedRows.delete(pr.html_url)"
              >
                <span
                  aria-hidden="true"
                  class="pointer-events-none absolute inset-x-1 inset-y-0.5 rounded-md bg-wow-1/5 opacity-0 shadow-lg shadow-wow-1/25 ring-1 ring-wow-1/40 scale-[0.97] transition-[opacity,scale] duration-fast group-hover/row:scale-100 group-hover/row:opacity-100 group-hover/row:duration-slower group-hover/row:ease-spring group-active/row:scale-[0.98]"
                />
                <button
                  class="relative w-full text-left flex items-start gap-3 px-4 py-3 transition-[scale,translate] duration-fast hover:translate-x-[calc(var(--px,0)*1rem)] hover:scale-[1.012] hover:duration-slower hover:ease-spring active:scale-[0.985] active:duration-faster focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring cursor-pointer"
                  @click="openPR(pr)"
                >
                  <component
                    :is="pr.is_draft ? GitPullRequestDraft : GitPullRequest"
                    class="h-4 w-4 mt-0.5 shrink-0 transition-[scale,rotate] duration-fast group-hover/row:scale-125 group-hover/row:-rotate-12 group-hover/row:duration-slower group-hover/row:ease-spring"
                    :class="pr.is_draft ? 'text-muted-foreground' : 'text-emerald-600 dark:text-emerald-400'"
                    aria-hidden="true"
                  />
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2 min-w-0">
                      <span data-pr-title class="text-sm font-medium truncate" :title="pr.title">{{ pr.title }}</span>
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
        <div ref="spot" aria-hidden="true" class="pointer-events-none absolute inset-0 rounded-lg wow-spotlight group-hover/list:before:opacity-100" />
        <!-- One sweep per turn, again for as long as the reload is still running. -->
        <div
          v-if="sheen"
          :key="sheen"
          aria-hidden="true"
          class="pointer-events-none absolute inset-0 z-20 rounded-lg wow-sheen"
          @animationend="sheen = loading ? sheen + 1 : 0"
        />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { errorMessage } from '@/lib/utils'
import { ref, computed, nextTick, onMounted, onActivated, onDeactivated, watch } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useLocalStorage, useTimeoutFn } from '@vueuse/core'
import { toast } from 'vue-sonner'
import { RefreshCw, GitPullRequest, GitPullRequestDraft, Filter, FolderGit2 } from '@lucide/vue'
import { Separator } from '@/components/ui/separator'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import CountUp from '@/components/motion/CountUp.vue'
import { reducedMotion } from '@/components/motion/wow'
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

// ── Showpiece moments ─────────────────────────────────────────────────────────
// Each is a flag that lasts as long as its animation: a keyframe class left on an element would replay every time KeepAlive re-attaches the page.

/** The entrance: rows and sidebar groups cascade in. */
const arriving = ref(false)
const { start: endArrival } = useTimeoutFn(() => { arriving.value = false }, 1400, { immediate: false })
/** Counts the sweeps of the current reload; 0 when none is running. */
const sheen = ref(0)
const shake = ref(false)
let attached = true

// Not v-pointer on the list: a custom property set there is inherited, so every pointer move would restyle every row
// (70ms a move at 160 rows). The light is a leaf, and only the hovered row learns where the pointer is.
const spot = ref<HTMLElement | null>(null)
function trackPointer(e: PointerEvent): void {
  if (!spot.value || reducedMotion()) return
  const box = spot.value.getBoundingClientRect()
  spot.value.style.setProperty('--mx', `${e.clientX - box.x}px`)
  spot.value.style.setProperty('--my', `${e.clientY - box.y}px`)
  if (e.target instanceof Element) e.target.closest<HTMLElement>('li')?.style.setProperty('--px', String((e.clientX - box.x) / box.width - 0.5))
}

function refresh(): void {
  void fetchPRs(false, true)
}

const refreshIcon = 'h-4 w-4 transition-[rotate,scale] duration-slower ease-spring group-hover/refresh:rotate-180 group-active/refresh:scale-75'
/** The skeleton's accent sweep runs down the rows as a wave. */
const wave = '[animation-delay:calc(var(--i)*120ms)]'

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
    moveClass: 'transition-[transform,opacity] duration-slower ease-spring',
    enterActiveClass: 'transition-[opacity,scale,translate] duration-(--transition-duration-slower) ease-(--ease-spring)',
    enterFromClass: 'opacity-0 scale-90 translate-y-4',
    // A leaving row is moved too (it drops out of flow); important, so its quick exit beats the move's spring.
    leaveActiveClass: 'absolute! inset-x-0 transition-[opacity,scale,transform]! duration-fast! ease-out!',
    leaveToClass: 'opacity-0 scale-95',
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

/** Rows that are new, or whose title, draft state or update time changed, in the last refresh; a ring bursts from each once. */
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

const countNoun = computed(() => (visibleItems.value.length === 1 ? 'pull request' : 'pull requests'))

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

/** How many loaded PRs each involvement brings. */
const involved = computed(() => {
  const items = rawResult.value?.items ?? []
  return {
    author: items.filter((pr) => pr.is_author).length,
    assignee: items.filter((pr) => pr.is_assignee).length,
    reviewer: items.filter((pr) => pr.is_reviewer).length,
  }
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
 * `entrance` replays the cascade the first answer gets (the refresh buttons).
 */
async function fetchPRs(background = false, entrance = false): Promise<void> {
  const thisId = filters.requestId.value

  loading.value = !background
  if (!background && rawResult.value && !reducedMotion()) sheen.value ||= 1

  try {
    // Only include_drafts is meaningful server-side; other filter fields are ignored.
    const res = await App.ListOpenPRs(filters.toFilters())

    if (thisId !== filters.requestId.value) return

    // A failed refresh carries no items; what is on screen stays.
    if (!res.error) {
      const before = new Map(rawResult.value?.items?.map((pr) => [pr.html_url, pr]))
      const changed = (before.size ? res.items ?? [] : []).filter((pr) => {
        const old = before.get(pr.html_url)
        return !old || old.title !== pr.title || old.is_draft !== pr.is_draft || old.updated_at !== pr.updated_at
      })
      // A list that changed wholesale has nothing to point at, and that many rings at once drop frames.
      changedRows.value = new Set(changed.length <= 15 ? changed.map((pr) => pr.html_url) : [])
      if (background) rowMotionOnce('settle')
      if (!background && (entrance || !rawResult.value) && attached && !reducedMotion()) {
        arriving.value = true
        endArrival()
      }
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
      shake.value = !!errorMsg.value && !reducedMotion()
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
  attached = true
})

onDeactivated(() => {
  attached = false
  arriving.value = false
  shake.value = false
  sheen.value = 0
  changedRows.value = new Set()
})
</script>

<style scoped>
.pr-float {
  animation: pr-float 5s ease-in-out infinite alternate;
}
@keyframes pr-float {
  from { translate: 0 0.25rem; rotate: -5deg; }
  to { translate: 0 -0.5rem; rotate: 5deg; }
}
</style>
