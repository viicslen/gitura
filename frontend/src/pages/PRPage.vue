<template>
  <div class="flex gap-6 items-start p-6">
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
            v-if="filters.repo.value"
            class="ml-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
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
          <ul
            v-if="repoDropdownOpen && filteredRepoOptions.length > 0"
            class="absolute z-10 mt-1 w-full rounded-md border border-border bg-popover shadow-md max-h-48 overflow-auto text-sm"
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
        </div>
      </section>

      <!-- Organisation filter (searchable from loaded results) -->
      <section>
        <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          Organisation
          <button
            v-if="filters.org.value"
            class="ml-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
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
          <ul
            v-if="orgDropdownOpen && filteredOrgOptions.length > 0"
            class="absolute z-10 mt-1 w-full rounded-md border border-border bg-popover shadow-md max-h-48 overflow-auto text-sm"
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
        </div>
      </section>

      <!-- Author filter -->
      <section>
        <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          PR Author
          <button
            v-if="filters.author.value"
            class="ml-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
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
        />
      </section>

      <!-- Updated after filter -->
      <section>
        <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          Updated after
          <button
            v-if="filters.updatedAfter.value"
            class="ml-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
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
      <div v-if="filters.hasActiveFilters.value">
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
    <main class="flex-1 min-w-0" aria-label="Pull request list" aria-live="polite">
      <!-- Toolbar: count + refresh -->
      <div class="flex items-center justify-between mb-4">
        <p class="text-sm text-muted-foreground">
          <template v-if="!loading && !errorMsg">
            {{ visibleItems.length }} pull requests
            <span v-if="rawResult?.incomplete_results" class="ml-1 text-yellow-500">(incomplete)</span>
          </template>
        </p>
        <Button
          variant="ghost"
          size="sm"
          :disabled="loading"
          aria-label="Refresh pull request list"
          @click="fetchPRs()"
        >
          <RefreshCw :class="['h-4 w-4', loading && 'animate-spin']" />
        </Button>
      </div>

      <!-- Rate limit banner -->
      <div
        v-if="errorMsg && rateLimitReset"
        class="mb-4 rounded-md border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 text-sm text-yellow-600 dark:text-yellow-400"
        role="alert"
      >
        GitHub rate limit reached. Resets at {{ formatReset(rateLimitReset) }}.
      </div>

      <!-- General error banner -->
      <div
        v-else-if="errorMsg"
        class="mb-4 rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive"
        role="alert"
      >
        {{ errorMsg }}
      </div>

      <!-- Skeleton loading state -->
      <template v-if="loading">
        <div class="space-y-2" aria-label="Loading pull requests">
          <div
            v-for="n in 6"
            :key="n"
            class="rounded-lg border border-border bg-card p-4"
          >
            <div class="flex items-start justify-between gap-4">
              <div class="flex-1 space-y-2">
                <Skeleton class="h-4 w-3/4" />
                <Skeleton class="h-3 w-1/2" />
              </div>
              <Skeleton class="h-5 w-14 shrink-0" />
            </div>
          </div>
        </div>
      </template>

      <!-- Empty: no PRs at all (no filters active) -->
      <template v-else-if="visibleItems.length === 0 && !filters.hasActiveFilters.value && !errorMsg">
        <div class="flex flex-col items-center justify-center py-20 text-center gap-3">
          <GitPullRequest class="h-10 w-10 text-muted-foreground" />
          <p class="text-sm text-muted-foreground">No open pull requests found.</p>
        </div>
      </template>

      <!-- Empty: filters active but no results -->
      <template v-else-if="visibleItems.length === 0 && filters.hasActiveFilters.value && !errorMsg">
        <div class="flex flex-col items-center justify-center py-20 text-center gap-3">
          <Filter class="h-10 w-10 text-muted-foreground" />
          <p class="text-sm text-muted-foreground">No pull requests match the current filters.</p>
          <Button variant="ghost" size="sm" @click="filters.clearAllFilters()">
            Clear filters
          </Button>
        </div>
      </template>

      <!-- PR rows -->
      <template v-else-if="visibleItems.length > 0">
        <ul class="space-y-2" role="list">
          <li
            v-for="pr in visibleItems"
            :key="pr.html_url"
            role="listitem"
          >
            <button
              class="w-full text-left rounded-lg border border-border bg-card px-4 py-3 hover:bg-accent/50 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer"
              :aria-label="`${pr.owner}/${pr.repo} #${pr.number}: ${pr.title}`"
              @click="openPR(pr)"
            >
              <div class="flex items-start justify-between gap-4">
                <div class="flex-1 min-w-0">
                  <!-- Title row -->
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="text-sm font-medium truncate">{{ pr.title }}</span>
                    <Badge v-if="pr.is_draft" variant="secondary" class="shrink-0 text-xs">
                      Draft
                    </Badge>
                  </div>
                  <!-- Meta row -->
                  <div class="mt-1 flex items-center gap-2 text-xs text-muted-foreground flex-wrap">
                    <span class="font-mono">{{ pr.owner }}/{{ pr.repo }} #{{ pr.number }}</span>
                    <span>·</span>
                    <span>{{ pr.author_login }}</span>
                    <span>·</span>
                    <span>updated {{ formatAge(pr.updated_at) }}</span>
                  </div>
                </div>
              </div>
            </button>
          </li>
        </ul>
      </template>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { RefreshCw, GitPullRequest, Filter } from 'lucide-vue-next'
import { Separator } from '@/components/ui/separator'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import { usePRFilters } from '@/composables/usePRFilters'
import * as App from '../wailsjs/go/main/App'
import type { model } from '../wailsjs/go/models'
import type { ReviewLoadInput } from '@/types/review'

const emit = defineEmits<{
  (e: 'open-review', item: ReviewLoadInput): void
}>()

// ── State ─────────────────────────────────────────────────────────────────────

const loading = ref(false)
const errorMsg = ref<string | null>(null)
const rateLimitReset = ref<string | null>(null)

/** Raw unfiltered result from the backend (involves: query). */
const rawResult = ref<model.PRListResult | null>(null)

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
  return filters.applyFilters(rawResult.value.items)
})

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
  filters.repo.value = fullName
  repoSearch.value = fullName
  repoDropdownOpen.value = false
}

function onRepoBlur(): void {
  setTimeout(() => {
    repoDropdownOpen.value = false
    if (repoSearch.value !== filters.repo.value) {
      filters.repo.value = repoSearch.value.trim()
    }
  }, 150)
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
  filters.org.value = name
  orgSearch.value = name
  orgDropdownOpen.value = false
}

function onOrgBlur(): void {
  setTimeout(() => {
    orgDropdownOpen.value = false
    if (orgSearch.value !== filters.org.value) {
      filters.org.value = orgSearch.value.trim()
    }
  }, 150)
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
async function fetchPRs(): Promise<void> {
  const thisId = filters.requestId.value

  loading.value = true
  errorMsg.value = null
  rateLimitReset.value = null

  try {
    // Only include_drafts is meaningful server-side; other filter fields are ignored.
    const res = await App.ListOpenPRs(filters.toFilters())

    console.debug('[fetchPRs] thisId', thisId, 'requestId', filters.requestId.value, 'items', res?.items?.length)
    if (res?.items?.length) {
      const sample = res.items[0]
      console.debug('[fetchPRs] sample item', JSON.stringify({ title: sample.title, is_author: sample.is_author, is_assignee: sample.is_assignee, is_reviewer: sample.is_reviewer }))
    }
    console.debug('[fetchPRs] filter state', JSON.stringify({ includeAuthor: filters.includeAuthor.value, includeAssignee: filters.includeAssignee.value, includeReviewer: filters.includeReviewer.value }))

    if (thisId !== filters.requestId.value) return

    rawResult.value = res

    if (res.error) {
      errorMsg.value = res.error
    }
    if (res.rate_limit_reset) {
      rateLimitReset.value = res.rate_limit_reset
    }
  } catch (e) {
    if (thisId !== filters.requestId.value) return
    errorMsg.value = String(e)
  } finally {
    if (thisId === filters.requestId.value) {
      loading.value = false
    }
  }
}

// ── Navigation ────────────────────────────────────────────────────────────────

function openPR(pr: model.PRListItem): void {
  emit('open-review', {
    owner: pr.owner,
    repo: pr.repo,
    number: pr.number,
    title: pr.title,
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
</script>
