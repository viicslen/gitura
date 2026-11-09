<script setup lang="ts">
import { computed } from 'vue'
import { highlightLine, lineLanguages } from '@/lib/highlight'
import { anchorKey, type BlockKind, type LineSelection, type PaneItem, type Side } from '@/lib/diffLayout'

const props = defineProps<{
  side: Side
  items: PaneItem[]
  /** Kind of each change block, indexed by a line's `block`. */
  blockKinds: BlockKind[]
  /** A split pane sits next to the connector gutter; a single pane is a whole added or removed file. */
  split: boolean
  language: string
  commentable?: boolean
  selection?: LineSelection | null
  /** Anchor keys that have inline rows (threads, drafts, comment form) under their line. */
  inlineKeys: Set<string>
  /** Wrap long lines; otherwise the pane scrolls sideways. */
  wrap: boolean
}>()

const emit = defineEmits<{
  /** Mouse press on a line number; starts a drag, or extends the open range with Shift. */
  (e: 'press', hunk: number, line: number, shift: boolean): void
  (e: 'hover', hunk: number, line: number): void
  /** Keyboard or assistive-tech activation of a line number. */
  (e: 'activate', hunk: number, line: number, shift: boolean): void
}>()

defineSlots<{ inline(props: { no: number }): unknown }>()

// Lines between two gaps are one contiguous slice of the file, which mixed-language files need.
const html = computed(() => {
  const out = props.items.map(() => '')
  const carry = {}
  let run: number[] = []
  const flush = () => {
    const contents = run.map((i) => (props.items[i] as Extract<PaneItem, { type: 'line' }>).content)
    const langs = lineLanguages(contents, props.language, carry)
    run.forEach((i, k) => (out[i] = highlightLine(contents[k], langs[k])))
    run = []
  }
  props.items.forEach((it, i) => {
    if (it.type === 'gap') flush()
    else if (it.type === 'line') run.push(i)
  })
  flush()
  return out
})

const LINE_BG: Record<BlockKind, string> = {
  added: 'bg-emerald-500/15',
  deleted: 'bg-rose-500/15',
  modified: 'bg-sky-500/15',
}
// The selection bar sits on the line-number side of the row.
const SELECTED_BAR = {
  left: 'shadow-[inset_3px_0_0_var(--color-primary)]',
  right: 'shadow-[inset_-3px_0_0_var(--color-primary)]',
}
// Sticky line numbers need an opaque cell; the tint is repeated underneath the number.
const STICKY_TINT: Record<BlockKind, string> = {
  added: 'before:bg-emerald-500/15',
  deleted: 'before:bg-rose-500/15',
  modified: 'before:bg-sky-500/15',
}
const MARK_BG: Record<BlockKind, string> = {
  added: 'bg-emerald-500/70',
  deleted: 'bg-rose-500/70',
  modified: 'bg-sky-500/70',
}

const singleKind = computed<BlockKind>(() => (props.side === 'LEFT' ? 'deleted' : 'added'))
const kindOf = (block: number): BlockKind | null =>
  !props.split ? singleKind.value : block >= 0 ? props.blockKinds[block] : null

function changeLabel(block: number): string {
  if (!props.split) return props.side === 'LEFT' ? ' (removed)' : ' (added)'
  return block >= 0 ? ` (${props.blockKinds[block]})` : ''
}

function isSelected(no: number): boolean {
  const s = props.selection
  return !!s && s.side === props.side && no >= s.from && no <= s.to
}

// Line numbers sit against the connector gutter on both sides.
const numbersRight = computed(() => props.split && props.side === 'LEFT')
</script>

<template>
  <div class="relative font-mono text-xs leading-5" :class="wrap ? '' : 'w-max min-w-full'">
    <template v-for="(item, i) in items" :key="item.key">
      <div
        v-if="item.type === 'gap'"
        class="select-none border-y border-border/60 bg-muted/50 py-0.5 text-center text-[0.6875rem] text-muted-foreground"
        :class="wrap ? '' : 'sticky left-0 w-[100cqw]'"
      >
        ⋯ {{ item.hidden }} unchanged line{{ item.hidden === 1 ? '' : 's' }}
      </div>
      <div v-else-if="item.type === 'mark'" aria-hidden="true" class="h-px" :class="MARK_BG[blockKinds[item.block]]" />
      <div v-else>
        <div
          class="group/line flex"
          :class="[
            kindOf(item.block) ? LINE_BG[kindOf(item.block)!] : '',
            numbersRight ? 'flex-row-reverse' : '',
            isSelected(item.no) ? [SELECTED_BAR[numbersRight ? 'right' : 'left'], 'bg-primary/15!'] : '',
          ]"
        >
          <button
            type="button"
            class="w-12 shrink-0 self-start select-none px-2 text-[0.6875rem] text-muted-foreground/80 tabular-nums"
            :class="[
              numbersRight ? 'text-left' : 'text-right',
              wrap ? '' : [
                'sticky z-[1] bg-background before:absolute before:inset-0 before:-z-10 before:content-[\'\']',
                numbersRight ? 'right-0' : 'left-0',
                isSelected(item.no)
                  ? ['before:bg-primary/15', SELECTED_BAR[numbersRight ? 'right' : 'left']]
                  : kindOf(item.block) ? STICKY_TINT[kindOf(item.block)!] : '',
              ],
              commentable ? 'cursor-pointer hover:bg-primary hover:text-primary-foreground' : 'cursor-default',
            ]"
            :tabindex="commentable ? 0 : -1"
            :aria-label="commentable ? `Comment on ${side === 'LEFT' ? 'old' : 'new'} line ${item.no}${changeLabel(item.block)}` : undefined"
            :title="commentable ? 'Click or drag to comment; Shift+click extends the range' : undefined"
            @mousedown.left="commentable && ($event.preventDefault(), emit('press', item.hunk, item.no, $event.shiftKey))"
            @mouseenter="commentable && emit('hover', item.hunk, item.no)"
            @click="commentable && $event.detail === 0 && emit('activate', item.hunk, item.no, $event.shiftKey)"
          >
            {{ item.no }}
          </button>
          <code class="min-w-0 flex-1 px-2" :class="wrap ? 'whitespace-pre-wrap break-words' : 'whitespace-pre'" v-html="html[i]" />
        </div>
        <div v-if="inlineKeys.has(anchorKey(side, item.no))" class="font-sans" :class="wrap ? '' : 'sticky left-0 w-[100cqw]'">
          <slot name="inline" :no="item.no" />
        </div>
      </div>
    </template>
  </div>
</template>
