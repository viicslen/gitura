<script setup lang="ts">
import { computed } from 'vue'
import hljs from 'highlight.js/lib/common'
import type { model } from '@/wailsjs/go/models'

const props = defineProps<{
  /** Old-side line (null for pure-add lines). */
  oldLine: model.DiffLineDTO | null
  /** New-side line (null for pure-delete lines). */
  newLine: model.DiffLineDTO | null
  /** 1-based line number for the right side (for comment targeting). */
  rightLineNumber?: number
  /** Whether comment affordance is enabled. */
  commentable?: boolean
  /** Row index within the hunk (for drag tracking). */
  rowIndex?: number
  /** Whether this row is within an active drag selection. */
  inDragRange?: boolean
  /** Whether this row is part of the active comment selection (form is open). */
  inCommentRange?: boolean
  /** When true, render a single column instead of split (for added/removed files). */
  singleSide?: boolean
  /** hljs language id for syntax highlighting (e.g. "typescript"). */
  language?: string
}>()

const emit = defineEmits<{
  (e: 'open-comment', payload: { line: number; side: 'RIGHT' | 'LEFT'; rowIndex: number }): void
  (e: 'drag-start', rowIndex: number): void
  (e: 'drag-enter', rowIndex: number): void
  (e: 'drag-end', rowIndex: number): void
}>()

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function highlight(content: string): string {
  const lang = props.language && hljs.getLanguage(props.language) ? props.language : 'plaintext'
  try {
    return hljs.highlight(content, { language: lang }).value
  } catch {
    return escapeHtml(content)
  }
}

const oldHighlighted = computed(() => props.oldLine ? highlight(props.oldLine.content) : '')
const newHighlighted = computed(() => props.newLine ? highlight(props.newLine.content) : '')

function lineClass(type: string): string {
  switch (type) {
    case 'add':
      return 'bg-[#f0fff4] dark:bg-[#1b4721]'
    case 'delete':
      return 'bg-[#ffeef0] dark:bg-[#78191b]'
    default:
      return ''
  }
}

function lineNumClass(type: string): string {
  switch (type) {
    case 'add':
      return 'text-green-600 dark:text-green-400 select-none'
    case 'delete':
      return 'text-red-500 dark:text-red-400 select-none'
    default:
      return 'text-muted-foreground select-none'
  }
}

function prefixClass(type: string): string {
  switch (type) {
    case 'add':
      return 'text-green-600 dark:text-green-400'
    case 'delete':
      return 'text-red-500 dark:text-red-400'
    default:
      return 'text-muted-foreground'
  }
}

function prefix(type: string): string {
  if (type === 'add') return '+'
  if (type === 'delete') return '-'
  return ' '
}

function handleMousedown(event: MouseEvent): void {
  if (!props.commentable || props.rowIndex === undefined) return
  // Only start drag on left-click on the content cells (not the comment button)
  if ((event.target as HTMLElement).closest('button')) return
  emit('drag-start', props.rowIndex)
}

function handleMouseenter(): void {
  if (!props.commentable || props.rowIndex === undefined) return
  emit('drag-enter', props.rowIndex)
}

function handleMouseup(): void {
  if (!props.commentable || props.rowIndex === undefined) return
  emit('drag-end', props.rowIndex)
}
</script>

<template>
  <tr
    class="group"
    :class="[
      inDragRange ? 'ring-1 ring-inset ring-primary/40 bg-primary/5' : '',
      inCommentRange ? 'bg-primary/10 ring-1 ring-inset ring-primary/50' : '',
    ]"
    @mousedown="handleMousedown"
    @mouseenter="handleMouseenter"
    @mouseup="handleMouseup"
  >
    <!-- ── SINGLE-SIDE view (added/removed files) ────────────────────────── -->
    <template v-if="singleSide">
      <!-- Use newLine for added files, oldLine for removed files -->
      <td
        class="w-10 px-1 text-right text-[10px] font-mono border-r border-border/50 shrink-0 align-top leading-5"
        :class="(newLine ?? oldLine) ? lineNumClass((newLine ?? oldLine)!.type) : 'bg-muted/30'"
      >
        {{ newLine?.new_no ?? oldLine?.old_no ?? '' }}
      </td>
      <td
        class="relative px-0 font-mono text-xs leading-5 align-top whitespace-pre-wrap break-all"
        :class="(newLine ?? oldLine) ? lineClass((newLine ?? oldLine)!.type) : 'bg-muted/30'"
      >
        <span v-if="newLine ?? oldLine">
          <span :class="prefixClass((newLine ?? oldLine)!.type)" class="select-none px-1">{{ prefix((newLine ?? oldLine)!.type) }}</span><!-- --><span v-html="newLine ? newHighlighted : oldHighlighted" />
        </span>

        <!-- Comment affordance -->
        <button
          v-if="commentable && newLine && rightLineNumber !== undefined"
          class="absolute right-1 top-0 hidden group-hover:flex items-center justify-center
                 w-4 h-4 mt-0.5 rounded-full bg-primary text-primary-foreground text-[10px] font-bold
                 hover:scale-110 transition-transform cursor-pointer border-0 p-0"
          :aria-label="`Add comment at line ${rightLineNumber}`"
          tabindex="0"
          @click.stop="emit('open-comment', { line: rightLineNumber!, side: 'RIGHT', rowIndex: rowIndex! })"
          @keydown.enter.prevent="emit('open-comment', { line: rightLineNumber!, side: 'RIGHT', rowIndex: rowIndex! })"
          @keydown.space.prevent="emit('open-comment', { line: rightLineNumber!, side: 'RIGHT', rowIndex: rowIndex! })"
        >
          +
        </button>
      </td>
    </template>

    <!-- ── SPLIT view (modified/renamed files) ──────────────────────────── -->
    <template v-else>
      <!-- OLD side -->
      <td
        class="w-10 px-1 text-right text-[10px] font-mono border-r border-border/50 shrink-0 align-top leading-5"
        :class="oldLine ? lineNumClass(oldLine.type) : 'bg-muted/30'"
      >
        {{ oldLine?.old_no ?? '' }}
      </td>
      <td
        class="px-0 font-mono text-xs leading-5 align-top whitespace-pre-wrap break-all"
        :class="oldLine ? lineClass(oldLine.type) : 'bg-muted/30'"
      >
        <span v-if="oldLine">
          <span :class="prefixClass(oldLine.type)" class="select-none px-1">{{ prefix(oldLine.type) }}</span><!-- --><span v-html="oldHighlighted" />
        </span>
      </td>

      <!-- NEW side -->
      <td
        class="w-10 px-1 text-right text-[10px] font-mono border-r border-border/50 shrink-0 align-top leading-5"
        :class="newLine ? lineNumClass(newLine.type) : 'bg-muted/30'"
      >
        {{ newLine?.new_no ?? '' }}
      </td>
      <td
        class="relative px-0 font-mono text-xs leading-5 align-top whitespace-pre-wrap break-all"
        :class="newLine ? lineClass(newLine.type) : 'bg-muted/30'"
      >
        <span v-if="newLine">
          <span :class="prefixClass(newLine.type)" class="select-none px-1">{{ prefix(newLine.type) }}</span><!-- --><span v-html="newHighlighted" />
        </span>

        <!-- Comment affordance (hidden until hover, or if drag-range active) -->
        <button
          v-if="commentable && newLine && rightLineNumber !== undefined"
          class="absolute right-1 top-0 hidden group-hover:flex items-center justify-center
                 w-4 h-4 mt-0.5 rounded-full bg-primary text-primary-foreground text-[10px] font-bold
                 hover:scale-110 transition-transform cursor-pointer border-0 p-0"
          :aria-label="`Add comment at line ${rightLineNumber}`"
          tabindex="0"
          @click.stop="emit('open-comment', { line: rightLineNumber!, side: 'RIGHT', rowIndex: rowIndex! })"
              @keydown.enter.prevent="emit('open-comment', { line: rightLineNumber!, side: 'RIGHT', rowIndex: rowIndex! })"
              @keydown.space.prevent="emit('open-comment', { line: rightLineNumber!, side: 'RIGHT', rowIndex: rowIndex! })"
        >
          +
        </button>
      </td>
    </template>
  </tr>
</template>
