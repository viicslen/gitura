<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useEventListener } from '@vueuse/core'
import { Badge } from '@/components/ui/badge'
import DiffSplitSegment from './DiffSplitSegment.vue'
import DiffInlineThread from './DiffInlineThread.vue'
import InlineCommentForm from './InlineCommentForm.vue'
import DiffPendingComment from './DiffPendingComment.vue'
import type { model } from '@/wailsjs/go/models'
import { langFromPath } from '@/lib/lang'
import { anchorKey, buildDiffRows, diffLineKeys, type DiffSegment, type LineSelection, type Side } from '@/lib/diffLayout'

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
const hunks = computed(() => props.diff?.hunks ?? [])
const lineKeys = computed(() => diffLineKeys(hunks.value))

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
// While a comment posts, the form's target is pinned so a failed post can't be retried on another line.
const posting = ref(false)
const drag = ref<{ side: Side; hunk: number; anchor: number; current: number } | null>(null)
watch(() => props.file.filename, () => {
  form.value = null
  drag.value = null
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

function onPress(hunk: number, side: Side, line: number, shift: boolean): void {
  if (posting.value) return
  if (shift && extendForm(side, hunk, line)) return
  drag.value = { side, hunk, anchor: line, current: line }
}

function onHover(hunk: number, side: Side, line: number): void {
  if (drag.value && drag.value.side === side && drag.value.hunk === hunk) drag.value.current = line
}

function onActivate(hunk: number, side: Side, line: number, shift: boolean): void {
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

// ── Layout ─────────────────────────────────────────────────────────────────

const formKey = computed(() => (form.value ? anchorKey(form.value.side, form.value.line) : ''))

const rows = computed(() => {
  const breakAt = new Set<string>([
    ...placedThreads.value.map(threadKey),
    ...placedDrafts.value.map((d) => draftKey(d.draft)),
  ])
  if (formKey.value) breakAt.add(formKey.value)
  return buildDiffRows(hunks.value, breakAt)
})

// Inline rows are siblings of segments with keys independent of the layout, so re-splitting
// (opening the form, new threads) moves them instead of remounting and losing typed text.
type Item =
  | { type: 'gap'; key: string; hidden: number }
  | { type: 'segment'; key: string; segment: DiffSegment }
  | { type: 'thread'; key: string; thread: model.CommentThreadDTO }
  | { type: 'draft'; key: string; draft: model.DraftCommentDTO }
  | { type: 'form'; key: string }

const items = computed<Item[]>(() => {
  const out: Item[] = []
  for (const row of rows.value) {
    if (row.type === 'gap') {
      out.push(row)
      continue
    }
    out.push({ type: 'segment', key: `seg:${row.segment.key}`, segment: row.segment })
    for (const key of row.segment.anchors) {
      for (const t of placedThreads.value) {
        if (threadKey(t) === key) out.push({ type: 'thread', key: `thread:${t.root_id}`, thread: t })
      }
      for (const d of placedDrafts.value) {
        if (draftKey(d.draft) === key) out.push({ type: 'draft', key: d.key, draft: d.draft })
      }
      if (key === formKey.value) out.push({ type: 'form', key: 'form' })
    }
  }
  return out
})

/** Only segments containing selected lines get a selection, so a drag re-renders just those. */
function selectionFor(segment: DiffSegment): LineSelection | null {
  const s = selection.value
  if (!s) return null
  const lines = s.side === 'LEFT' ? segment.left : segment.right
  return lines.some((l) => l.no >= s.from && l.no <= s.to) ? s : null
}

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

    <div v-else class="flex-1 overflow-auto">
      <div
        v-if="mode === 'split'"
        class="sticky top-0 z-10 grid grid-cols-[minmax(0,1fr)_2.5rem_minmax(0,1fr)] border-b border-border bg-background/95 font-mono text-[11px] text-muted-foreground backdrop-blur"
      >
        <span class="truncate px-3 py-1">Base · {{ file.previous_filename || file.filename }}</span>
        <span />
        <span class="truncate px-3 py-1">Head · {{ file.filename }}</span>
      </div>

      <div v-if="unplacedThreads.length || unplacedDrafts.length" class="border-b border-border bg-muted/20 py-1">
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

      <template v-for="item in items" :key="item.key">
        <div
          v-if="item.type === 'gap'"
          class="select-none border-y border-border/60 bg-muted/50 py-0.5 text-center font-mono text-[11px] text-muted-foreground"
        >
          ⋯ {{ item.hidden }} unchanged line{{ item.hidden === 1 ? '' : 's' }}
        </div>
        <DiffSplitSegment
          v-else-if="item.type === 'segment'"
          :segment="item.segment"
          :mode="mode"
          :language="language"
          :commentable="commentable"
          :selection="selectionFor(item.segment)"
          @press="(side, line, shift) => onPress(item.segment.hunk, side, line, shift)"
          @hover="(side, line) => onHover(item.segment.hunk, side, line)"
          @activate="(side, line, shift) => onActivate(item.segment.hunk, side, line, shift)"
        />
        <DiffInlineThread
          v-else-if="item.type === 'thread'"
          :thread="item.thread"
          :language="language"
          @resolve="emit('resolve', $event)"
          @unresolve="emit('unresolve', $event)"
          @reply-sent="emit('reply-sent', $event)"
        />
        <DiffPendingComment v-else-if="item.type === 'draft'" :draft="item.draft" />
        <div v-else-if="item.type === 'form' && form" class="px-3 py-2 font-sans">
          <InlineCommentForm
            :target="{ path: file.filename, side: form.side, line: form.line, startLine: form.startLine }"
            :post="submitComment"
            @cancel="form = null"
          />
        </div>
      </template>
    </div>
  </div>
</template>
