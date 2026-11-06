<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useElementSize, useEventListener, useResizeObserver } from '@vueuse/core'
import { Badge } from '@/components/ui/badge'
import DiffPane from './DiffPane.vue'
import DiffInlineThread from './DiffInlineThread.vue'
import InlineCommentForm from './InlineCommentForm.vue'
import DiffPendingComment from './DiffPendingComment.vue'
import type { model } from '@/wailsjs/go/models'
import { langFromPath } from '@/lib/lang'
import { anchorKey, buildSplitLayout, diffLineKeys, mapOffset, type BlockKind, type Edge, type LineSelection, type Side } from '@/lib/diffLayout'

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
  /** Saves a new comment as a draft or posts it now; resolves false on failure. */
  postComment: (comment: model.DraftCommentDTO, mode: 'draft' | 'immediate') => Promise<boolean>
}>()

const emit = defineEmits<{
  (e: 'resolve', rootId: number): void
  (e: 'unresolve', rootId: number): void
  (e: 'reply-sent', comment: model.CommentDTO): void
}>()

const language = computed(() => langFromPath(props.file.filename))
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

const indexedDrafts = computed(() => props.drafts.map((draft, i) => ({ draft, key: `draft:${i}` })))
const placedDrafts = computed(() =>
  props.draftsStale ? [] : indexedDrafts.value.filter((d) => lineKeys.value.has(draftKey(d.draft))),
)
const unplacedDrafts = computed(() => indexedDrafts.value.filter((d) => !placedDrafts.value.includes(d)))

// ── Comment form ───────────────────────────────────────────────────────────

interface FormTarget {
  side: Side
  hunk: number
  line: number
  startLine?: number
}
const form = ref<FormTarget | null>(null)
const formBody = ref('')
// While a comment posts, the form's target is pinned so a failed post can't be retried on another line.
const posting = ref(false)
const drag = ref<{ side: Side; hunk: number; anchor: number; current: number } | null>(null)
watch(() => props.file.filename, () => {
  form.value = null
  drag.value = null
})
watch(form, (f) => {
  if (!f) formBody.value = ''
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
  if (posting.value) return
  if (shift && extendForm(side, hunk, line)) return
  drag.value = { side, hunk, anchor: line, current: line }
}

function onHover(side: Side, hunk: number, line: number): void {
  if (drag.value && drag.value.side === side && drag.value.hunk === hunk) drag.value.current = line
}

function onActivate(side: Side, hunk: number, line: number, shift: boolean): void {
  if (posting.value) return
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

async function submitComment(body: string, submitMode: 'draft' | 'immediate'): Promise<boolean> {
  const t = form.value
  if (!t) return false
  const comment = {
    path: props.file.filename,
    body,
    line: t.line,
    side: t.side,
    start_line: t.startLine,
    start_side: t.startLine ? t.side : undefined,
  } as model.DraftCommentDTO
  posting.value = true
  try {
    const ok = await props.postComment(comment, submitMode)
    if (ok && form.value === t) form.value = null
    return ok
  } finally {
    posting.value = false
  }
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
const inlineKeys = computed(() => new Set(inlineByKey.value.keys()))

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
}

useResizeObserver(() => [contentEl('LEFT'), contentEl('RIGHT')], measure)
watch([layout, inlineKeys, mode], () => nextTick(measure))

const pos = (m: PaneMetrics, e: Edge) => (e.at === 'top' ? m.top : m.bottom)[e.item] ?? 0

const syncPoints = computed<[number, number][]>(() => {
  const m = metrics.value
  if (!m) return []
  return [
    [0, 0],
    ...layout.value.sync.map((p): [number, number] => [pos(m.LEFT, p.left), pos(m.RIGHT, p.right)]),
    [m.LEFT.height, m.RIGHT.height],
  ]
})


function onScroll(side: Side): void {
  const el = scrollEls[side].value
  if (!el) return
  scrollTop[side] = el.scrollTop
  if (side !== master.value) return
  const other: Side = side === 'LEFT' ? 'RIGHT' : 'LEFT'
  const target = scrollEls[other].value
  if (!target || !syncPoints.value.length) return
  // The anchor slides from the viewport's top to its bottom as the pane scrolls, so both panes reach their ends together.
  const max = el.scrollHeight - el.clientHeight
  const anchor = el.scrollTop + el.clientHeight * (max > 0 ? el.scrollTop / max : 0)
  const points = side === 'LEFT' ? syncPoints.value : syncPoints.value.map(([l, r]): [number, number] => [r, l])
  const next = mapOffset(points, anchor) - (anchor - el.scrollTop)
  target.scrollTop = Math.max(0, Math.min(next, target.scrollHeight - target.clientHeight))
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
    const [l0, l1] = b.left.map((e) => pos(m.LEFT, e) - scrollTop.LEFT)
    const [r0, r1] = b.right.map((e) => pos(m.RIGHT, e) - scrollTop.RIGHT)
    if (Math.max(l1, r1) < 0 || Math.min(l0, r0) > h) continue
    out.push({
      d: `M0,${l0} C${mid},${l0} ${mid},${r0} ${w},${r0} L${w},${r1} C${mid},${r1} ${mid},${l1} 0,${l1} Z`,
      cls: SHAPE[b.kind],
    })
  }
  return out
})

function statusLabel(status: string): string {
  switch (status) {
    case 'added': return 'Added'
    case 'removed': return 'Deleted'
    case 'renamed': return 'Renamed'
    default: return 'Modified'
  }
}

function statusVariant(status: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'added': return 'default'
    case 'removed': return 'destructive'
    case 'renamed': return 'secondary'
    default: return 'outline'
  }
}
</script>

<template>
  <div class="flex min-h-0 flex-col overflow-hidden">
    <div class="flex shrink-0 items-center gap-3 border-b border-border bg-muted/30 px-4 py-2">
      <Badge :variant="statusVariant(file.status)" class="text-[10px]">
        {{ statusLabel(file.status) }}
      </Badge>
      <span v-if="file.previous_filename" class="truncate font-mono text-xs text-muted-foreground">
        {{ file.previous_filename }}<span class="mx-1">→</span>
      </span>
      <span class="flex-1 truncate font-mono text-xs font-medium">{{ file.filename }}</span>
      <span v-if="!file.is_binary" class="flex shrink-0 gap-2 text-xs">
        <span class="text-green-600 dark:text-green-400">+{{ file.additions }}</span>
        <span class="text-red-500 dark:text-red-400">-{{ file.deletions }}</span>
      </span>
    </div>

    <div v-if="file.is_binary" class="flex flex-1 items-center justify-center p-8 text-sm text-muted-foreground">
      Binary file — diff not available
    </div>

    <div v-else-if="!diff || hunks.length === 0" class="flex flex-1 items-center justify-center text-sm text-muted-foreground">
      {{ diff ? 'No textual changes' : 'No diff available' }}
    </div>

    <template v-else>
      <div v-if="unplacedThreads.length || unplacedDrafts.length" class="max-h-[40%] shrink-0 overflow-y-auto border-b border-border bg-muted/20 py-1">
        <p v-if="unplacedThreads.length" class="px-4 pt-1 text-xs text-muted-foreground">
          {{ unplacedThreads.length }} comment{{ unplacedThreads.length === 1 ? '' : 's' }} outside the visible diff
        </p>
        <DiffInlineThread
          v-for="t in unplacedThreads"
          :key="t.root_id"
          :thread="t"
          :language="language"
          @resolve="emit('resolve', $event)"
          @unresolve="emit('unresolve', $event)"
          @reply-sent="emit('reply-sent', $event)"
        />
        <p v-if="unplacedDrafts.length" class="px-4 pt-1 text-xs text-muted-foreground">
          {{ draftsStale
            ? 'Your pending comments were written against an earlier commit and will be submitted against it'
            : 'Pending comments outside the visible diff' }}
        </p>
        <DiffPendingComment v-for="d in unplacedDrafts" :key="d.key" :draft="d.draft" />
      </div>

      <div
        class="grid min-h-0 flex-1"
        :class="mode === 'split'
          ? 'grid-cols-[minmax(0,1fr)_2.5rem_minmax(0,1fr)] grid-rows-[auto_minmax(0,1fr)]'
          : 'grid-cols-1 grid-rows-[minmax(0,1fr)]'"
      >
        <template v-if="mode === 'split'">
          <span class="truncate border-b border-border px-3 py-1 font-mono text-[11px] text-muted-foreground">
            Base · {{ file.previous_filename || file.filename }}
          </span>
          <span class="border-b border-border bg-muted/30" />
          <span class="truncate border-b border-border px-3 py-1 font-mono text-[11px] text-muted-foreground">
            Head · {{ file.filename }}
          </span>
        </template>

        <template v-for="(side, si) in sides" :key="side">
          <div
            v-if="si === 1"
            :ref="setGutterEl"
            aria-hidden="true"
            class="relative overflow-hidden bg-muted/30"
          >
            <svg class="absolute inset-0 overflow-visible" :width="gutterW" :height="gutterH">
              <path v-for="(c, i) in connectors" :key="i" :d="c.d" :class="c.cls" stroke-width="1" />
            </svg>
          </div>
          <div
            :ref="(el) => setScrollEl(side, el)"
            class="min-h-0 overflow-y-auto overflow-x-hidden"
            :class="mode === 'split' && side === 'LEFT' ? 'border-r border-border/60 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden' : ''"
            @scroll.passive="onScroll(side)"
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
              @press="(hunk, line, shift) => onPress(side, hunk, line, shift)"
              @hover="(hunk, line) => onHover(side, hunk, line)"
              @activate="(hunk, line, shift) => onActivate(side, hunk, line, shift)"
            >
              <template #inline="{ no }">
                <template v-for="row in inlineByKey.get(anchorKey(side, no)) ?? []" :key="row.key">
                  <DiffInlineThread
                    v-if="row.type === 'thread'"
                    :thread="row.thread"
                    :language="language"
                    @resolve="emit('resolve', $event)"
                    @unresolve="emit('unresolve', $event)"
                    @reply-sent="emit('reply-sent', $event)"
                  />
                  <DiffPendingComment v-else-if="row.type === 'draft'" :draft="row.draft" />
                  <div v-else-if="form" class="px-3 py-2">
                    <InlineCommentForm
                      v-model:body="formBody"
                      :target="{ path: file.filename, side: form.side, line: form.line, startLine: form.startLine }"
                      :post="submitComment"
                      @cancel="form = null"
                    />
                  </div>
                </template>
              </template>
            </DiffPane>
          </div>
        </template>
      </div>
    </template>
  </div>
</template>
