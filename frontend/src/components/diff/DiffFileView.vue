<script setup lang="ts">
import { computed, h, nextTick, onBeforeUnmount, onMounted, reactive, ref, shallowRef, watch } from 'vue'
import { useElementSize, useEventListener, useResizeObserver } from '@vueuse/core'
import { Badge } from '@/components/ui/badge'
import { FileSymlink, Plus } from '@lucide/vue'
import { describePathChange } from '@/lib/pathChange'
import DiffPane from './DiffPane.vue'
import DiffInlineThread from './DiffInlineThread.vue'
import InlineCommentForm from './InlineCommentForm.vue'
import DiffDraftComment from './DiffDraftComment.vue'
import GlideList from '@/components/motion/GlideList.vue'
import { ROW_GLIDE } from '@/components/motion/motion'
import type * as model from '@/bindings/gitura/internal/model'
import { langFromPath } from '@/lib/highlight'
import { anchorKey, buildSplitLayout, diffLineKeys, syncScrollTop, type BlockKind, type CommentTarget, type Edge, type LineSelection, type Side, type UnsentComments } from '@/lib/diffLayout'

const props = defineProps<{
  file: model.PRFileDTO
  diff: model.ParsedDiffDTO | null
  commentable?: boolean
  /** Review threads on this file. */
  threads: model.CommentThreadDTO[]
  /** Unsubmitted review comments on this file. */
  drafts: model.DraftCommentDTO[]
  /** Drafts were written against an older head, so their line numbers don't match this diff. */
  draftsStale?: boolean
  showThreads: boolean
  wrapLines: boolean
  /** Holds half-written comments per file so switching files doesn't lose them. */
  unsentComments: UnsentComments
  /** Saves a new comment as a draft or posts it now; resolves false on failure. */
  postComment: (comment: model.DraftCommentDTO, mode: 'draft' | 'immediate') => Promise<boolean>
}>()

const emit = defineEmits<{
  (e: 'resolve', rootId: number): void
  (e: 'unresolve', rootId: number): void
  (e: 'reply-sent', comment: model.ReviewCommentDTO): void
}>()

const language = computed(() => langFromPath(props.file.filename))
const pathChange = computed(() =>
  props.file.previous_filename ? describePathChange(props.file.previous_filename, props.file.filename) : null,
)
const pathChangeTitle = computed(() => {
  const kind = props.file.status === 'copied' ? 'copied' : pathChange.value?.kind ?? ''
  return kind.charAt(0).toUpperCase() + kind.slice(1)
})
// Drops the joining slash next to an empty changed part (e.g. a file moved into a new folder).
const tail = (middle: string, suffix: string) => (middle ? suffix : suffix.replace(/^\//, ''))
const mode = computed(() =>
  props.file.status === 'added' ? 'right' : props.file.status === 'removed' ? 'left' : 'split',
)
const sides = computed<Side[]>(() => (mode.value === 'split' ? ['LEFT', 'RIGHT'] : mode.value === 'left' ? ['LEFT'] : ['RIGHT']))
const hunks = computed(() => props.diff?.hunks ?? [])
const lineKeys = computed(() => diffLineKeys(hunks.value))
const layout = computed(() => buildSplitLayout(hunks.value))
const blockKinds = computed<BlockKind[]>(() => layout.value.blocks.map((b) => b.kind))

const threadKey = (t: model.CommentThreadDTO) => anchorKey(t.side === 'LEFT' ? 'LEFT' : 'RIGHT', t.line)
const draftKey = (d: model.DraftCommentDTO) => anchorKey(d.side === 'LEFT' ? 'LEFT' : 'RIGHT', d.line)

const visibleThreads = computed(() => (props.showThreads ? props.threads : []))
const placedThreads = computed(() =>
  visibleThreads.value.filter((t) => !t.outdated && lineKeys.value.has(threadKey(t))),
)
const unplacedThreads = computed(() => visibleThreads.value.filter((t) => !placedThreads.value.includes(t)))

// Keyed by line, not by position in the review: removing one draft must not re-key the others.
const indexedDrafts = computed(() => {
  const perLine = new Map<string, number>()
  return props.drafts.map((draft) => {
    const at = draftKey(draft)
    const n = perLine.get(at) ?? 0
    perLine.set(at, n + 1)
    return { draft, key: `draft:${at}:${n}` }
  })
})
const placedDrafts = computed(() =>
  props.draftsStale ? [] : indexedDrafts.value.filter((d) => lineKeys.value.has(draftKey(d.draft))),
)
const unplacedDrafts = computed(() => indexedDrafts.value.filter((d) => !placedDrafts.value.includes(d)))

// ── Comment form ───────────────────────────────────────────────────────────

type FormTarget = CommentTarget
const form = ref<FormTarget | null>(null)
const formDraft = reactive({ body: '' })

// A comment that is sending stays in the shared map marked `sending`, so every view of the file
// shows it locked instead of offering it again or letting a competing draft replace it.
const sending = computed(() => !!props.unsentComments.get(props.file.filename)?.sending)
function stashForm(path: string): void {
  if (props.unsentComments.get(path)?.sending) return
  if (form.value && formDraft.body.trim()) props.unsentComments.set(path, { target: form.value, body: formDraft.body })
  else props.unsentComments.delete(path)
}
// TODO: a restored comment whose line is gone after new commits stays hidden until the file is left.
function restoreForm(path: string): void {
  const saved = props.unsentComments.get(path)
  form.value = saved?.target ?? null
  formDraft.body = saved?.body ?? ''
}
restoreForm(props.file.filename)
onBeforeUnmount(() => stashForm(props.file.filename))
const drag = ref<{ side: Side; hunk: number; anchor: number; current: number } | null>(null)
watch(() => props.file.filename, (path, previous) => {
  stashForm(previous)
  restoreForm(path)
  drag.value = null
  opened = rowKeys()
})
watch(form, (f) => {
  if (!f) formDraft.body = ''
})
// A send started from an earlier view of this file succeeded: drop the locked copy shown here.
watch(() => props.unsentComments.get(props.file.filename), (entry, previous) => {
  if (previous?.sending && !entry) form.value = null
})

function rangeTarget(side: Side, hunk: number, a: number, b: number): FormTarget {
  const [lo, hi] = a <= b ? [a, b] : [b, a]
  return { side, hunk, line: hi, startLine: lo !== hi ? lo : undefined }
}

/** With Shift, grows the open comment range when the line is on the same side and hunk. */
function extendForm(side: Side, hunk: number, line: number): boolean {
  const f = form.value
  if (!f || f.side !== side || f.hunk !== hunk) return false
  form.value = rangeTarget(side, hunk, Math.min(f.startLine ?? f.line, line), Math.max(f.line, line))
  return true
}

function onPress(side: Side, hunk: number, line: number, shift: boolean): void {
  if (sending.value) return
  if (shift && extendForm(side, hunk, line)) return
  drag.value = { side, hunk, anchor: line, current: line }
}

function onHover(side: Side, hunk: number, line: number): void {
  if (drag.value && drag.value.side === side && drag.value.hunk === hunk) drag.value.current = line
}

function onActivate(side: Side, hunk: number, line: number, shift: boolean): void {
  if (sending.value) return
  if (shift && extendForm(side, hunk, line)) return
  form.value = { side, hunk, line }
}

useEventListener(window, 'mouseup', () => {
  const d = drag.value
  if (!d) return
  drag.value = null
  form.value = rangeTarget(d.side, d.hunk, d.anchor, d.current)
})

const selection = computed<LineSelection | null>(() => {
  const d = drag.value
  if (d) return { side: d.side, from: Math.min(d.anchor, d.current), to: Math.max(d.anchor, d.current) }
  const f = form.value
  return f ? { side: f.side, from: f.startLine ?? f.line, to: f.line } : null
})

// The form stays up until the posted comment comes back as a thread (3s at most), so the two swap in one move.
const unmounted = ref(false)
onBeforeUnmount(() => (unmounted.value = true))
function threadLands(at: string): Promise<void> {
  const count = () => placedThreads.value.filter((t) => threadKey(t) === at).length
  const before = count()
  return new Promise((resolve) => {
    const finish = () => {
      stop()
      clearTimeout(timer)
      resolve()
    }
    const stop = watch(() => unmounted.value || count() > before, (landed) => landed && finish())
    const timer = setTimeout(finish, 3000)
  })
}

async function submitComment(body: string, submitMode: 'draft' | 'immediate'): Promise<boolean> {
  const t = form.value
  if (!t || sending.value) return false
  const path = props.file.filename
  const comment = {
    path,
    body,
    line: t.line,
    side: t.side,
    start_line: t.startLine,
    start_side: t.startLine ? t.side : undefined,
  } as model.DraftCommentDTO
  props.unsentComments.set(path, { target: t, body: formDraft.body, sending: true })
  const entry = props.unsentComments.get(path)
  let ok = false
  try {
    ok = await props.postComment(comment, submitMode)
    if (ok && submitMode === 'immediate' && props.showThreads && !unmounted.value) await threadLands(anchorKey(t.side, t.line))
  } finally {
    if (entry && props.unsentComments.get(path) === entry) {
      if (ok) props.unsentComments.delete(path)
      else entry.sending = false
    }
  }
  if (ok && form.value === t) form.value = null
  return ok
}

// ── Inline rows ────────────────────────────────────────────────────────────

type Inline =
  | { type: 'thread'; key: string; thread: model.CommentThreadDTO }
  | { type: 'draft'; key: string; draft: model.DraftCommentDTO }
  | { type: 'form'; key: string }

const inlineByKey = computed(() => {
  const m = new Map<string, Inline[]>()
  const add = (key: string, row: Inline) => {
    const rows = m.get(key)
    if (rows) rows.push(row)
    else m.set(key, [row])
  }
  for (const t of placedThreads.value) add(threadKey(t), { type: 'thread', key: `thread:${t.root_id}`, thread: t })
  for (const d of placedDrafts.value) add(draftKey(d.draft), { type: 'draft', key: d.key, draft: d.draft })
  if (form.value) add(anchorKey(form.value.side, form.value.line), { type: 'form', key: 'form' })
  return m
})
// A line keeps its inline container once it had a row, so the last row has somewhere to animate out of.
const usedAnchors = new Set<string>()
const inlineKeys = computed(() => {
  for (const key of inlineByKey.value.keys()) usedAnchors.add(key)
  return new Set(usedAnchors)
})
// Rows the file opens with don't animate, later ones do. Not a ref: flipping it must not re-render the panes.
let settled = false
onMounted(() => { settled = true })
// Threads and drafts that arrive after the file opened wear the glow border for one turn. Not reactive, like `settled`.
const rowKeys = () => new Set([...props.threads.map((t) => `thread:${t.root_id}`), ...indexedDrafts.value.map((d) => d.key)])
let opened = rowKeys()
const freshGlow = (key: string) => ({ 'wow-glow fresh-glow': !opened.has(key) })
const NO_ROWS: Inline[] = []
const rewrapped = ref(0)
watch(() => props.wrapLines, () => rewrapped.value++)
const rowKey = (row: Inline) => row.key

// ── Synchronized scrolling ─────────────────────────────────────────────────
// Each pane scrolls on its own; the one the user drives maps its position through the
// sync points, so through a change block the side with fewer lines moves slower.

const scrollEls = { LEFT: ref<HTMLElement | null>(null), RIGHT: ref<HTMLElement | null>(null) }
const gutterEl = ref<HTMLElement | null>(null)
const setScrollEl = (side: Side, el: unknown) => (scrollEls[side].value = el as HTMLElement | null)
const setGutterEl = (el: unknown) => (gutterEl.value = el as HTMLElement | null)
const { width: gutterW, height: gutterH } = useElementSize(gutterEl)
const master = ref<Side>('RIGHT')
const scrollTop = reactive<Record<Side, number>>({ LEFT: 0, RIGHT: 0 })
const scrollLeft: Record<Side, number> = { LEFT: 0, RIGHT: 0 }
const dirName = computed(() => props.file.filename.split('/').slice(0, -1).join('/'))
const baseName = computed(() => props.file.filename.split('/').pop() ?? props.file.filename)

interface PaneMetrics {
  top: number[]
  bottom: number[]
  height: number
}
const metrics = ref<Record<Side, PaneMetrics> | null>(null)

const contentEl = (side: Side) => (scrollEls[side].value?.firstElementChild as HTMLElement | null) ?? null

function measurePane(side: Side): PaneMetrics | null {
  const content = contentEl(side)
  if (!content) return null
  const items = side === 'LEFT' ? layout.value.left : layout.value.right
  const top: number[] = []
  const bottom: number[] = []
  Array.from(content.children).forEach((child, i) => {
    const el = child as HTMLElement
    const type = items[i]?.type
    top.push(el.offsetTop)
    bottom.push(
      type === 'line' ? el.offsetTop + (el.firstElementChild as HTMLElement).offsetHeight
        : type === 'mark' ? el.offsetTop
          : el.offsetTop + el.offsetHeight,
    )
  })
  return { top, bottom, height: content.offsetHeight }
}

function measure(): void {
  const l = mode.value === 'split' ? measurePane('LEFT') : null
  const r = mode.value === 'split' ? measurePane('RIGHT') : null
  metrics.value = l && r ? { LEFT: l, RIGHT: r } : null
  shifts.value = NO_SHIFT
}

// ── Gliding rows ───────────────────────────────────────────────────────────
// A full measure on every frame of a glide drops frames; one height read per gliding row moves everything below it instead.

/** Rows in mid-glide: the pane item each sits in, and its height at the last measure. */
const glides = new Map<HTMLElement, { side: Side; item: number; from: number }>()
const NO_SHIFT: Record<Side, [afterItem: number, dy: number][]> = { LEFT: [], RIGHT: [] }
/** How far each gliding row has pushed the items after it since the last measure. */
const shifts = shallowRef(NO_SHIFT)
let glideFrame = 0
// A glide's start event arrives a frame late, so its starting height is read together with a fresh measure.
let rebase = false

function followGlides(): void {
  // A row removed mid-glide sends no end event.
  for (const el of glides.keys()) if (!el.isConnected || !el.getAnimations().length) glides.delete(el)
  glideFrame = glides.size ? requestAnimationFrame(followGlides) : 0
  if (rebase || !glides.size) {
    measure()
    for (const [el, glide] of glides) glide.from = el.offsetHeight
    rebase = false
    return
  }
  const next: typeof NO_SHIFT = { LEFT: [], RIGHT: [] }
  for (const [el, glide] of glides) next[glide.side].push([glide.item, el.offsetHeight - glide.from])
  shifts.value = next
}

function onGlide(side: Side, event: TransitionEvent): void {
  const content = contentEl(side)
  const el = event.target as HTMLElement
  if (event.propertyName !== 'grid-template-rows' || !content?.contains(el)) return
  if (event.type !== 'transitionrun') {
    glides.delete(el)
    return
  }
  let item = el
  while (item.parentElement && item.parentElement !== content) item = item.parentElement
  glides.set(el, { side, item: Array.prototype.indexOf.call(content.children, item), from: 0 })
  rebase = true
  glideFrame ||= requestAnimationFrame(followGlides)
}
onBeforeUnmount(() => cancelAnimationFrame(glideFrame))

useResizeObserver(() => [contentEl('LEFT'), contentEl('RIGHT')], () => glides.size || measure())
watch([layout, inlineKeys, mode], () => nextTick(() => glides.size || measure()))

const shiftAt = (side: Side, item: number) =>
  shifts.value[side].reduce((sum, [after, dy]) => (after < item ? sum + dy : sum), 0)
const pos = (side: Side, m: PaneMetrics, e: Edge) =>
  ((e.at === 'top' ? m.top : m.bottom)[e.item] ?? 0) + shiftAt(side, e.item)

const syncPoints = computed<[number, number][]>(() => {
  const m = metrics.value
  if (!m) return []
  return [
    [0, 0],
    ...layout.value.sync.map((p): [number, number] => [pos('LEFT', m.LEFT, p.left), pos('RIGHT', m.RIGHT, p.right)]),
    [m.LEFT.height + shiftAt('LEFT', Infinity), m.RIGHT.height + shiftAt('RIGHT', Infinity)],
  ]
})

function onScroll(side: Side): void {
  const el = scrollEls[side].value
  if (!el) return
  scrollTop[side] = el.scrollTop
  // Only follow sideways moves: a shorter pane clamps scrollLeft, and copying it on every
  // vertical scroll would yank the other pane back.
  const movedSideways = el.scrollLeft !== scrollLeft[side]
  scrollLeft[side] = el.scrollLeft
  if (side !== master.value) return
  const other: Side = side === 'LEFT' ? 'RIGHT' : 'LEFT'
  const target = scrollEls[other].value
  if (target && movedSideways) {
    target.scrollLeft = el.scrollLeft
    scrollLeft[other] = target.scrollLeft
  }
  if (!target || !syncPoints.value.length) return
  const points = side === 'LEFT' ? syncPoints.value : syncPoints.value.map(([l, r]): [number, number] => [r, l])
  target.scrollTop = syncScrollTop(
    points,
    el.scrollTop,
    el.scrollHeight - el.clientHeight,
    target.scrollHeight - target.clientHeight,
    el.clientHeight,
  )
  scrollTop[other] = target.scrollTop
}

watch(syncPoints, () => onScroll(master.value))
watch(() => props.file.filename, () => {
  master.value = 'RIGHT'
  for (const side of ['LEFT', 'RIGHT'] as const) {
    const el = scrollEls[side].value
    if (el) el.scrollTop = 0
    scrollTop[side] = 0
  }
})

const SHAPE: Record<BlockKind, string> = {
  added: 'fill-emerald-500/20 stroke-emerald-500/70',
  deleted: 'fill-rose-500/20 stroke-rose-500/70',
  modified: 'fill-sky-500/20 stroke-sky-500/70',
}

const connectors = computed(() => {
  const m = metrics.value
  if (!m) return []
  const w = gutterW.value
  const h = gutterH.value
  const mid = w / 2
  const out: { d: string; cls: string }[] = []
  for (const b of layout.value.blocks) {
    const [l0, l1] = b.left.map((e) => pos('LEFT', m.LEFT, e) - scrollTop.LEFT)
    const [r0, r1] = b.right.map((e) => pos('RIGHT', m.RIGHT, e) - scrollTop.RIGHT)
    if (Math.max(l1, r1) < 0 || Math.min(l0, r0) > h) continue
    out.push({
      d: `M0,${l0} C${mid},${l0} ${mid},${r0} ${w},${r0} L${w},${r1} C${mid},${r1} ${mid},${l1} 0,${l1} Z`,
      cls: SHAPE[b.kind],
    })
  }
  return out
})

// Its own render scope: the connectors follow every scroll and resize, and must not re-render the panes with them.
const Connectors = () => h('svg', { class: 'absolute inset-0 overflow-visible', width: gutterW.value, height: gutterH.value },
  connectors.value.map((c, i) => h('path', { key: i, d: c.d, class: c.cls, 'stroke-width': 1 })))

function statusLabel(status: string): string {
  switch (status) {
    case 'added': return 'Added'
    case 'removed': return 'Deleted'
    case 'renamed': return 'Renamed'
    case 'copied': return 'Copied'
    default: return 'Modified'
  }
}

function statusVariant(status: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'added': return 'default'
    case 'removed': return 'destructive'
    case 'renamed':
    case 'copied': return 'secondary'
    default: return 'outline'
  }
}
</script>

<template>
  <div class="flex min-h-0 flex-col overflow-hidden">
    <div class="relative flex shrink-0 items-center gap-3 border-b border-border bg-muted/30 px-4 py-2 wow-sheen">
      <Badge :variant="statusVariant(file.status)" class="text-[0.625rem]">
        {{ statusLabel(file.status) }}
      </Badge>
      <span class="flex min-w-0 flex-1 font-mono text-xs font-medium" :title="file.filename">
        <span v-if="dirName" class="min-w-0 truncate text-muted-foreground">{{ dirName }}/</span>
        <span class="max-w-full shrink-0 truncate">{{ baseName }}</span>
      </span>
      <span v-if="!file.is_binary && file.changes > 0" class="flex shrink-0 gap-2 text-xs">
        <span class="text-green-600 dark:text-green-400">+{{ file.additions }}</span>
        <span class="text-red-500 dark:text-red-400">-{{ file.deletions }}</span>
      </span>
      <slot name="header-actions" />
    </div>

    <div v-if="file.is_binary" class="flex flex-1 items-center justify-center p-8 text-sm text-muted-foreground">
      Binary file — diff not available
    </div>

    <div v-else-if="pathChange && diff && hunks.length === 0" class="flex flex-1 items-center justify-center p-6">
      <div class="w-full max-w-2xl rounded-lg border border-border bg-muted/30 p-5">
        <div class="flex items-center gap-3">
          <span class="flex size-9 shrink-0 items-center justify-center rounded-full bg-sky-500/15 text-sky-600 dark:text-sky-400">
            <FileSymlink class="size-4.5" />
          </span>
          <div>
            <p class="text-sm font-semibold">{{ pathChangeTitle }} without changes</p>
            <p class="text-xs text-muted-foreground">The content is identical; only the path changed.</p>
          </div>
        </div>
        <dl class="mt-4 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5 font-mono text-xs">
          <dt class="text-muted-foreground">from</dt>
          <dd class="break-all text-muted-foreground">
            {{ pathChange.prefix }}<mark v-if="pathChange.from" class="rounded-sm bg-rose-500/20 px-0.5 text-rose-700 dark:text-rose-300">{{ pathChange.from }}</mark>{{ tail(pathChange.from, pathChange.suffix) }}
          </dd>
          <dt class="text-muted-foreground">to</dt>
          <dd class="break-all">
            {{ pathChange.prefix }}<mark v-if="pathChange.to" class="rounded-sm bg-emerald-500/20 px-0.5 text-emerald-700 dark:text-emerald-300">{{ pathChange.to }}</mark>{{ tail(pathChange.to, pathChange.suffix) }}
          </dd>
        </dl>
      </div>
    </div>

    <div v-else-if="!diff || hunks.length === 0" class="flex flex-1 items-center justify-center text-sm text-muted-foreground">
      {{ diff ? 'No textual changes' : 'No diff available' }}
    </div>

    <template v-else>
      <p v-if="pathChange" class="shrink-0 truncate border-b border-border bg-sky-500/5 px-4 py-1.5 font-mono text-xs text-muted-foreground" :title="`${file.previous_filename} → ${file.filename}`">
        <span class="font-sans">{{ pathChangeTitle }}</span> from
        {{ pathChange.prefix }}<mark v-if="pathChange.from" class="rounded-sm bg-rose-500/20 px-0.5 text-rose-700 dark:text-rose-300">{{ pathChange.from }}</mark>{{ tail(pathChange.from, pathChange.suffix) }}
      </p>
      <!-- The bottom line is a shadow, not a border, so the box can close to nothing. -->
      <Transition v-bind="ROW_GLIDE">
        <div v-if="unplacedThreads.length || unplacedDrafts.length" class="grid max-h-[40%] shrink-0 bg-muted/20 shadow-[inset_0_-1px_0_var(--color-border)]">
          <div class="min-h-0 overflow-y-auto">
            <div class="py-1">
              <p v-if="unplacedThreads.length" class="px-4 pt-1 text-xs text-muted-foreground">
                {{ unplacedThreads.length }} comment{{ unplacedThreads.length === 1 ? '' : 's' }} outside the visible diff
              </p>
              <GlideList v-slot="{ item: t }" :items="unplacedThreads" :item-key="(t) => t.root_id" :appear="settled">
                <div class="grid">
                  <div class="min-h-0">
                    <DiffInlineThread
                      :thread="t"
                      :language="language"
                      :class="freshGlow(`thread:${t.root_id}`)"
                      @resolve="emit('resolve', $event)"
                      @unresolve="emit('unresolve', $event)"
                      @reply-sent="emit('reply-sent', $event)"
                    />
                  </div>
                </div>
              </GlideList>
              <p v-if="unplacedDrafts.length" class="px-4 pt-1 text-xs text-muted-foreground">
                {{ draftsStale
                  ? 'Your pending comments were written against an earlier commit and will be submitted against it'
                  : 'Pending comments outside the visible diff' }}
              </p>
              <GlideList v-slot="{ item: d }" :items="unplacedDrafts" :item-key="(d) => d.key" :appear="settled">
                <div class="grid"><div class="min-h-0"><DiffDraftComment :draft="d.draft" :class="freshGlow(d.key)" /></div></div>
              </GlideList>
            </div>
          </div>
        </div>
      </Transition>

      <div
        class="relative grid min-h-0 flex-1"
        :class="mode === 'split'
          ? 'grid-cols-[minmax(0,1fr)_2.5rem_minmax(0,1fr)] grid-rows-[auto_minmax(0,1fr)]'
          : 'grid-cols-1 grid-rows-[minmax(0,1fr)]'"
      >
        <!-- Every line re-wraps at once, so the new layout fades in from behind a cover instead of being mounted twice. -->
        <div
          v-if="rewrapped"
          :key="rewrapped"
          class="pointer-events-none absolute inset-0 z-[2] bg-background animate-out fade-out-0"
          @animationend="rewrapped = 0"
        />
        <template v-if="mode === 'split'">
          <span class="truncate border-b border-border px-3 py-1 font-mono text-[0.6875rem] text-muted-foreground">
            Base
          </span>
          <span class="border-b border-border bg-muted/30" />
          <span class="truncate border-b border-border px-3 py-1 font-mono text-[0.6875rem] text-muted-foreground">
            Head
          </span>
        </template>

        <template v-for="(side, si) in sides" :key="side">
          <div
            v-if="si === 1"
            :ref="setGutterEl"
            aria-hidden="true"
            class="relative overflow-hidden bg-muted/30"
          >
            <Connectors />
          </div>
          <div
            :ref="(el) => setScrollEl(side, el)"
            class="@container min-h-0 overflow-y-auto"
            :class="[
              wrapLines ? 'overflow-x-hidden' : 'overflow-x-auto',
              mode === 'split' && side === 'LEFT' ? 'border-r border-border/60 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden' : '',
            ]"
            @scroll.passive="onScroll(side)"
            @transitionrun="onGlide(side, $event)"
            @transitionend="onGlide(side, $event)"
            @transitioncancel="onGlide(side, $event)"
            @wheel.passive="master = side"
            @touchstart.passive="master = side"
            @pointerdown="master = side"
            @keydown="master = side"
            @focusin="master = side"
          >
            <DiffPane
              :side="side"
              :items="side === 'LEFT' ? layout.left : layout.right"
              :block-kinds="blockKinds"
              :split="mode === 'split'"
              :language="language"
              :commentable="commentable"
              :selection="selection"
              :inline-keys="inlineKeys"
              :wrap="wrapLines"
              @press="(hunk, line, shift) => onPress(side, hunk, line, shift)"
              @hover="(hunk, line) => onHover(side, hunk, line)"
              @activate="(hunk, line, shift) => onActivate(side, hunk, line, shift)"
            >
              <template #inline="{ no }">
                <GlideList
                  v-slot="{ item: row }"
                  :items="inlineByKey.get(anchorKey(side, no)) ?? NO_ROWS"
                  :item-key="rowKey"
                  :appear="settled"
                >
                  <div class="grid">
                    <div class="min-h-0">
                      <DiffInlineThread
                        v-if="row.type === 'thread'"
                        :thread="row.thread"
                        :language="language"
                        :class="freshGlow(row.key)"
                        @resolve="emit('resolve', $event)"
                        @unresolve="emit('unresolve', $event)"
                        @reply-sent="emit('reply-sent', $event)"
                      />
                      <DiffDraftComment v-else-if="row.type === 'draft'" :draft="row.draft" :class="freshGlow(row.key)" />
                      <!-- The form springs out of the line number it was opened from; clipped, so its overshoot never widens the pane. -->
                      <div v-else-if="form" class="relative overflow-clip px-3 py-2">
                        <span
                          aria-hidden="true"
                          class="gutter-pop pointer-events-none absolute top-0 z-[2] flex size-5 items-center justify-center rounded-full bg-wow-1 text-background wow-ring"
                          :class="mode === 'split' && side === 'LEFT' ? 'right-3.5' : 'left-3.5'"
                        >
                          <Plus class="size-3.5" />
                        </span>
                        <div class="wow-pop" :class="mode === 'split' && side === 'LEFT' ? 'origin-top-right' : 'origin-top-left'">
                          <InlineCommentForm
                            :draft="formDraft"
                            :target="{ path: file.filename, side: form.side, line: form.line, startLine: form.startLine }"
                            :post="submitComment"
                            :sending="sending"
                            @cancel="form = null"
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </GlideList>
              </template>
            </DiffPane>
          </div>
        </template>
      </div>
    </template>
  </div>
</template>

<style scoped>
.gutter-pop {
  animation: gutter-pop var(--transition-duration-show) var(--ease-spring) both;
}
@keyframes gutter-pop {
  from { scale: 0; }
  60% { opacity: 1; }
  to { opacity: 0; }
}

/* wow-glow spins for as long as it is worn; a fresh comment takes one turn and fades. Inset, because a thread clips at its border. */
.fresh-glow::before {
  inset: 0;
  padding: 2px;
  animation: wow-spin 1.6s linear, fresh-glow 1.6s var(--ease-out) forwards;
}
@keyframes fresh-glow {
  60% { opacity: 1; }
  to { opacity: 0; }
}
</style>
