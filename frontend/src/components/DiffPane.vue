<script setup lang="ts">
import { computed } from 'vue'
import { highlightLine } from '@/lib/highlight'
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
}>()

const emit = defineEmits<{
  /** Mouse press on a line number; starts a drag, or extends the open range with Shift. */
  (e: 'press', hunk: number, line: number, shift: boolean): void
  (e: 'hover', hunk: number, line: number): void
  /** Keyboard or assistive-tech activation of a line number. */
  (e: 'activate', hunk: number, line: number, shift: boolean): void
}>()

defineSlots<{ inline(props: { no: number }): unknown }>()

const html = computed(() => props.items.map((it) => (it.type === 'line' ? highlightLine(it.content, props.language) : '')))

const LINE_BG: Record<BlockKind, string> = {
  added: 'bg-emerald-500/15',
  deleted: 'bg-rose-500/15',
  modified: 'bg-sky-500/15',
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
  <div class="relative font-mono text-xs leading-5">
    <template v-for="(item, i) in items" :key="item.key">
      <div
        v-if="item.type === 'gap'"
        class="select-none border-y border-border/60 bg-muted/50 py-0.5 text-center text-[11px] text-muted-foreground"
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
            isSelected(item.no) ? 'shadow-[inset_3px_0_0_var(--color-primary)] bg-primary/15!' : '',
          ]"
        >
          <button
            type="button"
            class="w-12 shrink-0 select-none px-2 text-[11px] text-muted-foreground/80 tabular-nums"
            :class="[
              numbersRight ? 'text-left' : 'text-right',
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
          <code class="min-w-0 flex-1 whitespace-pre-wrap break-words px-2" v-html="html[i]" />
        </div>
        <div v-if="inlineKeys.has(anchorKey(side, item.no))" class="font-sans">
          <slot name="inline" :no="item.no" />
        </div>
      </div>
    </template>
  </div>
</template>
