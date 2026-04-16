<script setup lang="ts">
import { ref, computed } from 'vue'
import DiffLine from './DiffLine.vue'
import InlineCommentForm from './InlineCommentForm.vue'
import type { model } from '@/wailsjs/go/models'

const props = defineProps<{
  hunk: model.DiffHunkDTO
  /** Path of the file this hunk belongs to (for comment targeting). */
  path: string
  /** Whether comment affordance (+) buttons are shown. */
  commentable?: boolean
  /** Index of this hunk within the file's hunk list (used to anchor the compose box). */
  hunkIndex: number
  /** File-level status from GitHub API ('added' | 'removed' | 'modified' | 'renamed'). */
  fileStatus?: string
  /** hljs language id derived from the file extension (e.g. "typescript"). */
  language?: string
}>()

interface CommentTarget {
  path: string
  line: number
  side: 'RIGHT' | 'LEFT'
  startLine?: number
  hunkIndex: number
}

const emit = defineEmits<{
  (e: 'comment-submit', body: string, mode: 'draft' | 'immediate', target: CommentTarget): void
}>()

/** Row index after which the inline form should be rendered. null = closed. */
const activeCommentRowIdx = ref<number | null>(null)
const activeCommentStartRowIdx = ref<number | null>(null)
const activeCommentTarget = ref<CommentTarget | null>(null)

function openCommentAt(rowIdx: number, target: CommentTarget, startRowIdx?: number): void {
  activeCommentRowIdx.value = rowIdx
  activeCommentStartRowIdx.value = startRowIdx ?? rowIdx
  activeCommentTarget.value = target
}

function isCommentSelected(idx: number): boolean {
  if (activeCommentRowIdx.value === null || activeCommentStartRowIdx.value === null) return false
  const lo = Math.min(activeCommentStartRowIdx.value, activeCommentRowIdx.value)
  const hi = Math.max(activeCommentStartRowIdx.value, activeCommentRowIdx.value)
  return idx >= lo && idx <= hi
}

function handleCommentSubmit(body: string, mode: 'draft' | 'immediate'): void {
  if (!activeCommentTarget.value) return
  emit('comment-submit', body, mode, activeCommentTarget.value)
  activeCommentRowIdx.value = null
  activeCommentTarget.value = null
}

function handleCommentCancel(): void {
  activeCommentRowIdx.value = null
  activeCommentTarget.value = null
}

const expanded = ref(false)

// ── Drag-range selection state ─────────────────────────────────────────────

const dragStartIdx = ref<number | null>(null)
const dragEndIdx = ref<number | null>(null)
const isDragging = ref(false)

function dragMin(): number {
  if (dragStartIdx.value === null || dragEndIdx.value === null) return -1
  return Math.min(dragStartIdx.value, dragEndIdx.value)
}
function dragMax(): number {
  if (dragStartIdx.value === null || dragEndIdx.value === null) return -1
  return Math.max(dragStartIdx.value, dragEndIdx.value)
}

function isInDragRange(idx: number): boolean {
  if (!isDragging.value && dragStartIdx.value === null) return false
  return idx >= dragMin() && idx <= dragMax()
}

function handleDragStart(idx: number): void {
  dragStartIdx.value = idx
  dragEndIdx.value = idx
  isDragging.value = true
}

function handleDragEnter(idx: number): void {
  if (!isDragging.value) return
  dragEndIdx.value = idx
}

function handleDragEnd(idx: number): void {
  if (!isDragging.value) return
  dragEndIdx.value = idx
  isDragging.value = false

  // Resolve the line numbers from the row range
  const min = dragMin()
  const max = dragMax()
  const startRow = rows.value[min]
  const endRow = rows.value[max]

  const endLine = endRow?.rightLineNumber ?? endRow?.newLine?.new_no ?? null
  if (endLine == null) {
    resetDrag()
    return
  }

  const startLine = min !== max ? (startRow?.rightLineNumber ?? startRow?.newLine?.new_no ?? undefined) : undefined

  openCommentAt(max, {
    path: props.path,
    line: endLine,
    side: 'RIGHT',
    startLine: startLine !== endLine ? startLine : undefined,
    hunkIndex: props.hunkIndex,
  }, min)

  resetDrag()
}

function resetDrag(): void {
  dragStartIdx.value = null
  dragEndIdx.value = null
  isDragging.value = false
}

// Clear drag on mouseleave from the table
function handleTableMouseleave(): void {
  if (isDragging.value) resetDrag()
}

// ── Side-by-side row pairing ───────────────────────────────────────────────

interface SideBySideRow {
  oldLine: model.DiffLineDTO | null
  newLine: model.DiffLineDTO | null
  rightLineNumber?: number
}

const rows = computed<SideBySideRow[]>(() => {
  const lines = props.hunk.lines
  const result: SideBySideRow[] = []
  let i = 0

  while (i < lines.length) {
    const line = lines[i]

    if (line.type === 'context') {
      result.push({ oldLine: line, newLine: line, rightLineNumber: line.new_no ?? undefined })
      i++
    } else if (line.type === 'delete') {
      const deletes: model.DiffLineDTO[] = []
      while (i < lines.length && lines[i].type === 'delete') {
        deletes.push(lines[i])
        i++
      }
      const adds: model.DiffLineDTO[] = []
      while (i < lines.length && lines[i].type === 'add') {
        adds.push(lines[i])
        i++
      }
      const maxLen = Math.max(deletes.length, adds.length)
      for (let j = 0; j < maxLen; j++) {
        const dl = deletes[j] ?? null
        const al = adds[j] ?? null
        result.push({ oldLine: dl, newLine: al, rightLineNumber: al?.new_no ?? undefined })
      }
    } else if (line.type === 'add') {
      result.push({ oldLine: null, newLine: line, rightLineNumber: line.new_no ?? undefined })
      i++
    } else {
      i++
    }
  }

  return result
})

/** For added/removed files one side is always empty — use a single-column layout. */
const isSingleSide = computed(() => props.fileStatus === 'added' || props.fileStatus === 'removed')

function handleOpenComment(payload: { line: number; side: 'RIGHT' | 'LEFT'; rowIndex: number }): void {
  openCommentAt(payload.rowIndex, { path: props.path, hunkIndex: props.hunkIndex, line: payload.line, side: payload.side })
}
</script>

<template>
  <div class="overflow-x-auto" @mouseleave="handleTableMouseleave">
    <!-- Hunk header / collapse bar -->
    <div
      class="flex items-center gap-2 px-3 py-1 bg-muted/60 text-xs text-muted-foreground font-mono
             border-y border-border cursor-pointer select-none hover:bg-muted transition-colors"
      :aria-expanded="expanded"
      :aria-label="expanded ? 'Collapse hunk context' : `Show hunk: ${hunk.header}`"
      role="button"
      tabindex="0"
      @click="expanded = !expanded"
      @keydown.enter.prevent="expanded = !expanded"
      @keydown.space.prevent="expanded = !expanded"
    >
      <span class="text-primary/70">@@</span>
      <span class="flex-1 truncate">{{ hunk.header }}</span>
      <span class="shrink-0 text-[10px] text-muted-foreground/70">
        {{ expanded ? '▲ collapse' : '▼ expand' }}
      </span>
    </div>

    <!-- Diff table -->
    <table
      v-if="rows.length > 0"
      class="w-full border-collapse text-xs font-mono table-fixed"
      :class="isDragging ? 'select-none' : ''"
      aria-label="Diff hunk"
    >
      <colgroup>
        <col class="w-10" />
        <col />
        <template v-if="!isSingleSide">
          <col class="w-10" />
          <col />
        </template>
      </colgroup>
      <tbody>
        <template v-for="(row, idx) in rows" :key="idx">
          <DiffLine
            :old-line="row.oldLine"
            :new-line="row.newLine"
            :right-line-number="row.rightLineNumber"
            :commentable="commentable"
            :row-index="idx"
            :in-drag-range="isInDragRange(idx)"
            :in-comment-range="isCommentSelected(idx)"
            :single-side="isSingleSide"
            :language="language"
            @open-comment="handleOpenComment"
            @drag-start="handleDragStart"
            @drag-enter="handleDragEnter"
            @drag-end="handleDragEnd"
          />
          <!-- Inline comment form rendered as a row immediately after the target line -->
          <tr v-if="activeCommentRowIdx === idx" :key="`form-${idx}`">
            <td :colspan="isSingleSide ? 2 : 4" class="p-0 border-b border-border">
              <div class="px-4 py-3 bg-background">
                <InlineCommentForm
                  :target="activeCommentTarget!"
                  @submit="handleCommentSubmit"
                  @cancel="handleCommentCancel"
                />
              </div>
            </td>
          </tr>
        </template>
      </tbody>
    </table>
  </div>
</template>
