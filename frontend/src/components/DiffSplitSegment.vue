<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import { highlightLine } from '@/lib/highlight'
import type { BlockKind, DiffSegment, LineSelection, PaneLine, Side } from '@/lib/diffLayout'

const props = defineProps<{
  segment: DiffSegment
  /** 'split' renders both panes with connectors; 'left'/'right' render that pane alone. */
  mode: 'split' | 'left' | 'right'
  language: string
  commentable?: boolean
  selection?: LineSelection | null
}>()

const emit = defineEmits<{
  /** Mouse press on a line number; starts a drag, or extends the open range with Shift. */
  (e: 'press', side: Side, line: number, shift: boolean): void
  (e: 'hover', side: Side, line: number): void
  /** Keyboard or assistive-tech activation of a line number. */
  (e: 'activate', side: Side, line: number, shift: boolean): void
}>()

function highlight(lines: PaneLine[]): string[] {
  return lines.map((l) => highlightLine(l.content, props.language))
}

const panes = computed(() => {
  const both = [
    { side: 'LEFT' as const, lines: props.segment.left, html: highlight(props.segment.left) },
    { side: 'RIGHT' as const, lines: props.segment.right, html: highlight(props.segment.right) },
  ]
  if (props.mode === 'left') return [both[0]]
  if (props.mode === 'right') return [both[1]]
  return both
})

const LINE_BG: Record<BlockKind, string> = {
  added: 'bg-emerald-500/15',
  deleted: 'bg-rose-500/15',
  modified: 'bg-sky-500/15',
}
const SHAPE: Record<BlockKind, string> = {
  added: 'fill-emerald-500/20 stroke-emerald-500/70',
  deleted: 'fill-rose-500/20 stroke-rose-500/70',
  modified: 'fill-sky-500/20 stroke-sky-500/70',
}

function changeLabel(line: PaneLine): string {
  if (props.mode !== 'split') return props.mode === 'left' ? ' (removed)' : ' (added)'
  return line.block >= 0 ? ` (${props.segment.blocks[line.block].kind})` : ''
}

function lineBg(line: PaneLine): string {
  if (props.mode === 'left') return LINE_BG.deleted
  if (props.mode === 'right') return LINE_BG.added
  return line.block >= 0 ? LINE_BG[props.segment.blocks[line.block].kind] : ''
}

function isSelected(side: Side, no: number): boolean {
  const s = props.selection
  return !!s && s.side === side && no >= s.from && no <= s.to
}

// ── Connector geometry ─────────────────────────────────────────────────────
// Lines wrap, so block edges are measured from the DOM rather than derived from line counts.

const rootEl = ref<HTMLElement | null>(null)
const paneEls = { LEFT: ref<HTMLElement | null>(null), RIGHT: ref<HTMLElement | null>(null) }

interface Geometry {
  width: number
  height: number
  gutterL: number
  gutterR: number
  left: number[]
  right: number[]
}
const geo = ref<Geometry | null>(null)

function edges(pane: HTMLElement): number[] {
  const rows = Array.from(pane.children) as HTMLElement[]
  const ys = rows.map((r) => r.offsetTop)
  const last = rows[rows.length - 1]
  ys.push(last ? last.offsetTop + last.offsetHeight : 0)
  return ys
}

function measure(): void {
  const [l, r] = [paneEls.LEFT.value, paneEls.RIGHT.value]
  if (props.mode !== 'split' || !rootEl.value || !l || !r) return
  geo.value = {
    width: rootEl.value.clientWidth,
    height: rootEl.value.clientHeight,
    gutterL: l.offsetLeft + l.offsetWidth,
    gutterR: r.offsetLeft,
    left: edges(l),
    right: edges(r),
  }
}

useResizeObserver([rootEl, paneEls.LEFT, paneEls.RIGHT], measure)
watch(() => props.segment, () => nextTick(measure))

const shapes = computed(() => {
  const g = geo.value
  if (!g) return []
  const mid = (g.gutterL + g.gutterR) / 2
  return props.segment.blocks.map((b) => {
    const [l0, l1] = [g.left[b.left[0]], g.left[b.left[1]]]
    const [r0, r1] = [g.right[b.right[0]], g.right[b.right[1]]]
    const { gutterL: a, gutterR: z } = g
    const d = `M${a},${l0} C${mid},${l0} ${mid},${r0} ${z},${r0} L${z},${r1} C${mid},${r1} ${mid},${l1} ${a},${l1} Z`
    // A side with no lines gets a marker line across its pane at the insertion point.
    const marker = b.kind === 'added' ? `M0,${l0} H${a}` : b.kind === 'deleted' ? `M${z},${r0} H${g.width}` : ''
    return { d, marker, cls: SHAPE[b.kind] }
  })
})
</script>

<template>
  <div
    ref="rootEl"
    class="relative font-mono text-xs leading-5"
    :class="mode === 'split' ? 'grid grid-cols-[minmax(0,1fr)_2.5rem_minmax(0,1fr)]' : ''"
  >
    <template v-for="(pane, pi) in panes" :key="pane.side">
      <div v-if="pi === 1" aria-hidden="true" class="bg-muted/30" />
      <div :ref="(el) => (paneEls[pane.side].value = el as HTMLElement | null)" :class="mode === 'split' && pane.side === 'LEFT' ? 'border-r border-border/60' : ''">
        <div
          v-for="(line, li) in pane.lines"
          :key="li"
          class="group/line flex"
          :class="[
            lineBg(line),
            mode === 'split' && pane.side === 'LEFT' ? 'flex-row-reverse' : '',
            isSelected(pane.side, line.no) ? 'shadow-[inset_3px_0_0_var(--color-primary)] bg-primary/15!' : '',
          ]"
        >
          <button
            type="button"
            class="w-12 shrink-0 select-none px-2 text-[11px] text-muted-foreground/80 tabular-nums"
            :class="[
              mode === 'split' && pane.side === 'LEFT' ? 'text-left' : 'text-right',
              commentable ? 'cursor-pointer hover:bg-primary hover:text-primary-foreground' : 'cursor-default',
            ]"
            :tabindex="commentable ? 0 : -1"
            :aria-label="commentable ? `Comment on ${pane.side === 'LEFT' ? 'old' : 'new'} line ${line.no}${changeLabel(line)}` : undefined"
            :title="commentable ? 'Click or drag to comment; Shift+click extends the range' : undefined"
            @mousedown.left="commentable && ($event.preventDefault(), emit('press', pane.side, line.no, $event.shiftKey))"
            @mouseenter="commentable && emit('hover', pane.side, line.no)"
            @click="commentable && $event.detail === 0 && emit('activate', pane.side, line.no, $event.shiftKey)"
          >
            {{ line.no }}
          </button>
          <code class="min-w-0 flex-1 whitespace-pre-wrap break-words px-2" v-html="pane.html[li]" />
        </div>
      </div>
    </template>

    <svg
      v-if="mode === 'split' && geo"
      class="pointer-events-none absolute inset-0 overflow-visible"
      :width="geo.width"
      :height="geo.height"
      aria-hidden="true"
    >
      <template v-for="(s, i) in shapes" :key="i">
        <path :d="s.d" :class="s.cls" stroke-width="1" />
        <path v-if="s.marker" :d="s.marker" :class="s.cls" stroke-width="1.5" />
      </template>
    </svg>
  </div>
</template>
