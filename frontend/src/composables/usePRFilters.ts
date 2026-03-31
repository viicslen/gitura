import { ref, computed, readonly, watch } from 'vue'
import type { model } from '../wailsjs/go/models'

// ── Module-level singleton state ─────────────────────────────────────────────
// Declared outside the exported function so all component mounts share the same
// reactive object — filter state persists across route navigations for the
// lifetime of the JS session (FR-012).

const includeAuthor = ref(true)
const includeAssignee = ref(true)
const includeReviewer = ref(true)
const includeDrafts = ref(false)
const repo = ref('')
const org = ref('')
const author = ref('')
const updatedAfter = ref('')

// Tracks watch stop-handles so that re-mounting PRPage.vue does not accumulate
// duplicate watchers (each call to usePRFilters tears down prior watches first).
let stopWatches: (() => void)[] = []

// ── Derived state ─────────────────────────────────────────────────────────────

/** True when any filter deviates from its default value. */
const hasActiveFilters = computed(
  () =>
    !includeAuthor.value ||
    !includeAssignee.value ||
    !includeReviewer.value ||
    includeDrafts.value ||
    repo.value.trim() !== '' ||
    org.value.trim() !== '' ||
    author.value.trim() !== '' ||
    updatedAfter.value.trim() !== '',
)

// ── Constraint helper ─────────────────────────────────────────────────────────

/**
 * Returns true when the given involvement type is the only one currently active.
 * Use this to disable the corresponding checkbox in the UI (FR-001a).
 */
function isOnlyActiveInvolvement(type: 'author' | 'assignee' | 'reviewer'): boolean {
  const active = [includeAuthor.value, includeAssignee.value, includeReviewer.value]
  const trueCount = active.filter(Boolean).length
  if (trueCount !== 1) return false
  if (type === 'author') return includeAuthor.value
  if (type === 'assignee') return includeAssignee.value
  return includeReviewer.value
}

// ── Actions ───────────────────────────────────────────────────────────────────

/**
 * Toggle author involvement. Refused when it is the sole active type (FR-001a).
 */
function toggleAuthor(): void {
  if (isOnlyActiveInvolvement('author')) return
  includeAuthor.value = !includeAuthor.value
}

/**
 * Toggle assignee involvement. Refused when it is the sole active type.
 */
function toggleAssignee(): void {
  if (isOnlyActiveInvolvement('assignee')) return
  includeAssignee.value = !includeAssignee.value
}

/**
 * Toggle reviewer involvement. Refused when it is the sole active type.
 */
function toggleReviewer(): void {
  if (isOnlyActiveInvolvement('reviewer')) return
  includeReviewer.value = !includeReviewer.value
}

/**
 * Toggle draft inclusion. Triggers a new API query (FR-015).
 */
function toggleDrafts(): void {
  includeDrafts.value = !includeDrafts.value
}

function clearRepo(): void {
  repo.value = ''
}
function clearOrg(): void {
  org.value = ''
}
function clearAuthor(): void {
  author.value = ''
}
function clearUpdatedAfter(): void {
  updatedAfter.value = ''
}

/**
 * Reset all filters to their defaults in a single reactive flush (FR-008).
 */
function clearAllFilters(): void {
  includeAuthor.value = true
  includeAssignee.value = true
  includeReviewer.value = true
  includeDrafts.value = false
  repo.value = ''
  org.value = ''
  author.value = ''
  updatedAfter.value = ''
}

/**
 * Snapshot the current filter state as the Go backend DTO.
 * Only include_drafts is used server-side; all other fields are applied
 * client-side by applyFilters() (FR-013 local filtering architecture).
 */
function toFilters(): model.PRListFilters {
  return {
    include_author: includeAuthor.value,
    include_assignee: includeAssignee.value,
    include_reviewer: includeReviewer.value,
    include_drafts: includeDrafts.value,
    repo: '',
    org: '',
    author: '',
    updated_after: '',
  }
}

/**
 * Apply all active client-side filters to a raw list of PR items.
 * Returns a new array containing only items that match all active criteria.
 */
function applyFilters(items: model.PRListItem[]): model.PRListItem[] {
  return items.filter((pr) => {
    // Involvement: at least one active type must match (OR logic among types).
    const involvementMatch =
      (includeAuthor.value && pr.is_author) ||
      (includeAssignee.value && pr.is_assignee) ||
      (includeReviewer.value && pr.is_reviewer)
    if (!involvementMatch) return false

    // Repo: AND logic with other filters.
    if (repo.value.trim()) {
      const full = `${pr.owner}/${pr.repo}`
      if (!full.toLowerCase().includes(repo.value.trim().toLowerCase())) return false
    }

    // Org: match owner portion of the repo.
    if (org.value.trim()) {
      if (!pr.owner.toLowerCase().includes(org.value.trim().toLowerCase())) return false
    }

    // Author login.
    if (author.value.trim()) {
      if (!pr.author_login.toLowerCase().includes(author.value.trim().toLowerCase())) return false
    }

    // Updated after date.
    if (updatedAfter.value.trim()) {
      const threshold = new Date(updatedAfter.value.trim()).getTime()
      if (!isNaN(threshold) && new Date(pr.updated_at).getTime() < threshold) return false
    }

    return true
  })
}

// ── Composable export ─────────────────────────────────────────────────────────

/**
 * usePRFilters manages the PR list filter state and triggers a caller-supplied
 * fetch callback whenever any filter changes.
 *
 * State is module-level (session singleton). Calling this function multiple times
 * (e.g. on component re-mount) re-registers watchers while preserving the existing
 * filter values — the prior fetch callback is cleanly replaced (FR-012).
 *
 * The returned `requestId` ref increments on every triggered fetch. The caller
 * MUST capture the value at the start of each async fetch and discard the
 * response if the ref has advanced beyond that value by the time the response
 * arrives (FR-013 — stale response discarding, CHK010/CHK011).
 *
 * @param onFilterChange - Async function to call when filters change (e.g. fetches PRs).
 */
export function usePRFilters(onFilterChange: () => void | Promise<void>) {
  // ── Request-ID counter ─────────────────────────────────────────────────────
  // Incremented every time a filter-driven fetch is triggered. The caller reads
  // this before each async call and compares on response to detect staleness.
  // Module-level so it persists across re-mounts (FR-012 / CHK010 / CHK011).
  const requestId = ref(0)

  /** Increment the request counter and then invoke onFilterChange. */
  function triggerFetch(): void {
    requestId.value += 1
    void onFilterChange()
  }

  // Tear down any watches created by a previous mount so we don't accumulate
  // duplicate watchers when PRPage.vue is re-mounted.
  stopWatches.forEach((stop) => stop())
  stopWatches = []

  // Only includeDrafts requires a new server-side request — all other filters
  // are applied client-side by applyFilters() and must NOT trigger a fetch.
  stopWatches.push(
    watch(includeDrafts, () => triggerFetch()),
  )

  return {
    // Reactive state (readonly to prevent external direct mutation)
    includeAuthor: readonly(includeAuthor),
    includeAssignee: readonly(includeAssignee),
    includeReviewer: readonly(includeReviewer),
    includeDrafts: readonly(includeDrafts),
    repo, // writable — bound to <Input v-model>
    org, // writable
    author, // writable
    updatedAfter, // writable

    // Derived
    hasActiveFilters,

    // Request-ID counter for stale-response detection (CHK010/CHK011)
    requestId: readonly(requestId),

    // Actions
    toggleAuthor,
    toggleAssignee,
    toggleReviewer,
    toggleDrafts,
    clearRepo,
    clearOrg,
    clearAuthor,
    clearUpdatedAfter,
    clearAllFilters,
    toFilters,
    applyFilters,

    // Constraint helper (used to disable the last active involvement checkbox)
    isOnlyActiveInvolvement,
  }
}
