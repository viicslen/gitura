<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import DiffHunk from './DiffHunk.vue'
import type { model } from '@/wailsjs/go/models'
import { langFromPath } from '@/lib/lang'

const props = defineProps<{
  file: model.PRFileDTO
  diff: model.ParsedDiffDTO | null
  commentable?: boolean
  /** Other reviewer threads to render inline. */
  otherThreads?: model.CommentThreadDTO[]
  showOtherThreads?: boolean
}>()

const emit = defineEmits<{
  (e: 'draft-comment', comment: model.DraftCommentDTO): void
  (e: 'immediate-comment', comment: model.DraftCommentDTO): void
}>()

interface CommentTarget {
  path: string
  line: number
  side: 'RIGHT' | 'LEFT'
  startLine?: number
  hunkIndex: number
}

function handleCommentSubmit(body: string, mode: 'draft' | 'immediate', target: CommentTarget): void {
  const comment: model.DraftCommentDTO = {
    path: target.path,
    line: target.line,
    side: target.side,
    start_line: target.startLine,
    start_side: target.startLine ? target.side : undefined,
    body,
  }
  if (mode === 'draft') {
    emit('draft-comment', comment)
  } else {
    emit('immediate-comment', comment)
  }
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

function threadsForLine(line: number): model.CommentThreadDTO[] {
  if (!props.showOtherThreads || !props.otherThreads) return []
  return props.otherThreads.filter((t) => t.line === line && t.path === props.file.filename)
}
</script>

<template>
  <div class="flex flex-col min-h-0 overflow-hidden">
    <!-- File header -->
    <div class="flex items-center gap-3 px-4 py-2 border-b border-border bg-muted/30 shrink-0">
      <Badge :variant="statusVariant(file.status)" class="text-[10px]">
        {{ statusLabel(file.status) }}
      </Badge>

      <!-- Rename display -->
      <span v-if="file.previous_filename" class="text-xs font-mono text-muted-foreground truncate">
        {{ file.previous_filename }}
        <span class="mx-1">→</span>
      </span>
      <span class="text-xs font-mono font-medium truncate flex-1">{{ file.filename }}</span>

      <!-- +/- stats -->
      <span v-if="!file.is_binary" class="shrink-0 flex gap-2 text-xs">
        <span class="text-green-600 dark:text-green-400">+{{ file.additions }}</span>
        <span class="text-red-500 dark:text-red-400">-{{ file.deletions }}</span>
      </span>
    </div>

    <!-- Binary placeholder -->
    <div
      v-if="file.is_binary"
      class="flex-1 flex items-center justify-center text-sm text-muted-foreground p-8"
    >
      Binary file — diff not available
    </div>

    <!-- Diff hunks -->
    <div v-else-if="diff" class="overflow-auto flex-1">
      <template v-for="(hunk, hi) in diff.hunks" :key="hi">
        <DiffHunk
          :hunk="hunk"
          :path="file.filename"
          :commentable="commentable"
          :hunk-index="hi"
          :file-status="file.status"
          :language="langFromPath(file.filename)"
          @comment-submit="handleCommentSubmit"
        />

        <!-- Other reviewer threads inline -->
        <template v-if="showOtherThreads">
          <div
            v-for="thread in hunk.lines
              .filter((l) => l.new_no !== 0 && threadsForLine(l.new_no).length > 0)
              .flatMap((l) => threadsForLine(l.new_no))"
            :key="thread.root_id"
            class="border-b border-border px-4 py-2"
            :class="thread.outdated ? 'bg-muted/10 opacity-70' : 'bg-muted/20'"
          >
            <div class="flex items-center gap-2 mb-1">
              <span class="text-xs font-medium">{{ thread.comments[0]?.author_login ?? 'Unknown' }}</span>
              <Badge
                v-if="thread.outdated"
                variant="secondary"
                class="text-[10px] px-1.5 py-0"
              >Outdated</Badge>
            </div>
            <p class="text-xs text-muted-foreground whitespace-pre-wrap">
              {{ thread.comments[0]?.body ?? '' }}
            </p>
            <span v-if="thread.comments.length > 1" class="text-[10px] text-muted-foreground mt-1 block">
              +{{ thread.comments.length - 1 }} more
            </span>
          </div>
        </template>
      </template>
    </div>

    <!-- Empty diff state (no hunks) -->
    <div
      v-else-if="!diff"
      class="flex-1 flex items-center justify-center text-sm text-muted-foreground"
    >
      No diff available
    </div>
  </div>
</template>
