import { computed, readonly, ref, type WritableComputedRef } from 'vue'
import { toast } from 'vue-sonner'
import { GetToggles, SetToggles } from '@/bindings/gitura/internal/app/app'
import { errorMessage } from '@/lib/utils'

export const TOGGLE_SECTIONS = {
  review: 'Review',
  diff: 'Files',
  prList: 'Pull request list',
  commands: 'Commands',
} as const

interface ToggleDef {
  id: string
  section: keyof typeof TOGGLE_SECTIONS
  label: string
  hint: string
  default: boolean
  /** Browser-storage key this toggle used before settings.toml; read once, then removed. */
  legacyKey?: string
  /** Only has an effect while this other toggle is on. */
  requires?: string
}

export const TOGGLES = [
  { id: 'return_after_merge', section: 'review', label: 'Leave the review after merging', hint: 'Go back to the pull request list. A --focus window closes instead.', default: false },
  { id: 'show_resolved_reviews', section: 'review', label: 'Include fully resolved reviews', hint: 'Starting value of the switch in Overview.', default: false },
  { id: 'wrap_lines', section: 'diff', label: 'Wrap lines', hint: 'Wrap long lines instead of scrolling sideways.', default: true, legacyKey: 'gitura.diffWrapLines' },
  { id: 'show_comments', section: 'diff', label: 'Show comments', hint: 'Starting value of Show comments in the View menu.', default: true },
  { id: 'hide_viewed', section: 'diff', label: 'Hide viewed files', hint: 'Leave files you marked viewed out of the file list.', default: false, legacyKey: 'gitura.diffHideViewed' },
  { id: 'hide_tests', section: 'diff', label: 'Hide test files', hint: 'Leave test files out of the file list.', default: false, legacyKey: 'gitura.diffHideTests' },
  { id: 'auto_view_tests', section: 'diff', label: 'Mark hidden test files viewed', hint: 'Marks them viewed on GitHub when the files load.', default: false, legacyKey: 'gitura.diffAutoViewTests', requires: 'hide_tests' },
  { id: 'pr_list_author', section: 'prList', label: 'Authored by me', hint: 'Pull requests you opened. On by default in the list.', default: true },
  { id: 'pr_list_assignee', section: 'prList', label: 'Assigned to me', hint: 'Pull requests assigned to you. On by default in the list.', default: true },
  { id: 'pr_list_reviewer', section: 'prList', label: 'Review requested', hint: 'Pull requests waiting on your review. On by default in the list.', default: true },
  { id: 'pr_list_drafts', section: 'prList', label: 'Include drafts', hint: 'List draft pull requests too. On by default in the list.', default: false },
  { id: 'pr_list_tree', section: 'prList', label: 'Tree layout', hint: 'Nest each pull request under the one whose branch it is based on. Same as the List / Tree switch above the list.', default: false },
  { id: 'remove_draft_after_run', section: 'commands', label: 'Remove comments you run', hint: 'A pending comment sent to a command is taken out of your review, and one you are writing is discarded.', default: false },
] as const satisfies readonly ToggleDef[]

export type ToggleId = (typeof TOGGLES)[number]['id']
type Toggles = Record<ToggleId, boolean>

/** The PR list needs at least one of these on. */
export const INVOLVEMENT_TOGGLES = ['pr_list_author', 'pr_list_assignee', 'pr_list_reviewer'] as const satisfies readonly ToggleId[]

const defaults = Object.fromEntries(TOGGLES.map((t) => [t.id, t.default])) as Toggles
const state = ref<Toggles>({ ...defaults })

function readLegacy(key: string): boolean | undefined {
  try {
    const raw = localStorage.getItem(key)
    localStorage.removeItem(key)
    return raw === 'true' ? true : raw === 'false' ? false : undefined
  } catch {
    return undefined
  }
}

/** Reads the saved toggles; call before mounting so starting values are known. Unknown ids are dropped. */
export async function loadToggles(): Promise<void> {
  try {
    const saved = (await GetToggles()) ?? {}
    const next = { ...defaults }
    let migrated = false
    for (const t of TOGGLES as readonly ToggleDef[]) {
      const id = t.id as ToggleId
      if (typeof saved[id] === 'boolean') next[id] = saved[id]
      const legacy = t.legacyKey ? readLegacy(t.legacyKey) : undefined
      if (legacy !== undefined && !(id in saved)) {
        next[id] = legacy
        migrated = true
      }
    }
    state.value = stored = next
    if (migrated) await SetToggles(next)
  } catch {
    // Keep defaults rather than block startup on a settings read failure.
  }
}

// Saves run one at a time: each writes the whole set, so overlapping writes could land out of order.
let saving: Promise<unknown> = Promise.resolve()
/** The set last known to be in settings.toml; a failed save falls back to it. */
let stored = state.value

export async function setToggle(id: ToggleId, value: boolean): Promise<void> {
  state.value = { ...state.value, [id]: value }
  const next = state.value
  const save = saving.then(() => SetToggles(next))
  saving = save.catch(() => {})
  try {
    await save
    stored = next
  } catch (e: unknown) {
    // A later change still queued decides the state once its own save answers.
    if (state.value === next) state.value = stored
    toast.error('Could not save setting: ' + errorMessage(e))
  }
}

/** A ref for v-model that saves every change to settings.toml. */
export function toggleRef(id: ToggleId): WritableComputedRef<boolean> {
  return computed({ get: () => state.value[id], set: (v) => void setToggle(id, v) })
}

/** Why a toggle can't be changed right now, or '' when it can. */
export function toggleLock(id: ToggleId): string {
  const def: ToggleDef | undefined = TOGGLES.find((t) => t.id === id)
  const required = TOGGLES.find((t) => t.id === def?.requires)
  if (required && !state.value[required.id]) return `Needs ${required.label}.`
  const involvement: readonly ToggleId[] = INVOLVEMENT_TOGGLES
  if (involvement.includes(id) && state.value[id] && involvement.filter((i) => state.value[i]).length === 1) return 'At least one must stay on.'
  return ''
}

export function useToggles() {
  return { toggles: readonly(state) }
}
