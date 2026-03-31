<script setup lang="ts">
import { computed } from 'vue'
import type { model } from '../wailsjs/go/models'
import DiffHunkView from './DiffHunkView.vue'
import MarkdownBody from './MarkdownBody.vue'
import ReplyComposer from './ReplyComposer.vue'
import SuggestionBlock from './SuggestionBlock.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { CheckCircle, Circle } from 'lucide-vue-next'

const props = defineProps<{
  thread: model.CommentThreadDTO | null
  isAtEnd: boolean
}>()

const emit = defineEmits<{
  (e: 'resolve', rootId: number): void
  (e: 'unresolve', rootId: number): void
  (e: 'reply-sent', comment: model.CommentDTO): void
  (e: 'suggestion-committed', result: model.SuggestionCommitResult): void
}>()

function formatTimestamp(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  return d.toLocaleString([], {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const rootComment = computed(() => props.thread?.comments?.[0] ?? null)
const replies = computed(() => props.thread?.comments?.slice(1) ?? [])

const EXT_LANG: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
  vue: 'xml', py: 'python', rb: 'ruby', go: 'go', rs: 'rust',
  java: 'java', cs: 'csharp', cpp: 'cpp', c: 'c', h: 'c',
  php: 'php', html: 'html', css: 'css', scss: 'scss',
  json: 'json', yaml: 'yaml', yml: 'yaml', sh: 'bash', bash: 'bash',
  md: 'markdown', sql: 'sql', xml: 'xml', swift: 'swift',
  kt: 'kotlin', kts: 'kotlin', toml: 'ini',
}
function langFromPath(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase() ?? ''
  return EXT_LANG[ext] ?? 'plaintext'
}

function handleReplySent(comment: model.CommentDTO): void {
  emit('reply-sent', comment)
}

function handleSuggestionCommitted(result: model.SuggestionCommitResult): void {
  emit('suggestion-committed', result)
}

function toggleResolved(): void {
  if (!props.thread) return
  if (props.thread.resolved) {
    emit('unresolve', props.thread.root_id)
  } else {
    emit('resolve', props.thread.root_id)
  }
}
</script>

<template>
  <!-- No thread selected -->
  <div
    v-if="!thread"
    class="flex-1 flex items-center justify-center text-muted-foreground text-sm p-6"
  >
    Select a comment to begin.
  </div>

  <!-- Thread detail -->
  <ScrollArea v-else class="flex-1 h-full">
    <div class="p-4 space-y-4">
      <!-- Thread location -->
      <div class="flex items-center gap-2 text-xs text-muted-foreground">
        <span class="font-mono truncate">{{ thread.path }}<span v-if="thread.line">:{{ thread.line }}</span></span>
        <Badge v-if="thread.resolved" variant="secondary" class="text-xs opacity-70 shrink-0">
          Resolved
        </Badge>
      </div>

      <!-- Root comment -->
      <div v-if="rootComment" class="space-y-2">
        <div class="flex items-center gap-2">
          <img
            v-if="rootComment.author_avatar"
            :src="rootComment.author_avatar"
            :alt="rootComment.author_login"
            class="w-6 h-6 rounded-full"
          />
          <span class="text-sm font-medium">{{ rootComment.author_login }}</span>
          <span class="text-xs text-muted-foreground">{{ formatTimestamp(rootComment.created_at) }}</span>
          <Badge v-if="rootComment.is_suggestion" variant="outline" class="text-xs">
            Suggestion
          </Badge>
        </div>

        <!-- Body -->
        <div class="rounded-md bg-muted/40 px-3 py-2">
          <MarkdownBody :content="rootComment.body" />
        </div>

        <!-- Diff hunk -->
        <DiffHunkView
          v-if="rootComment.diff_hunk"
          :diff-hunk="rootComment.diff_hunk"
          :highlight-line="thread.line || undefined"
          :language="langFromPath(thread.path)"
        />

        <!-- Suggestion block (accept + commit) -->
        <SuggestionBlock
          v-if="rootComment.is_suggestion"
          :comment="rootComment"
          @committed="handleSuggestionCommitted"
        />
      </div>

      <!-- Replies -->
      <div v-if="replies.length > 0" class="space-y-3 border-l-2 border-border pl-4">
        <div v-for="reply in replies" :key="reply.id" class="space-y-1">
          <div class="flex items-center gap-2">
            <img
              v-if="reply.author_avatar"
              :src="reply.author_avatar"
              :alt="reply.author_login"
              class="w-5 h-5 rounded-full"
            />
            <span class="text-sm font-medium">{{ reply.author_login }}</span>
            <span class="text-xs text-muted-foreground">{{ formatTimestamp(reply.created_at) }}</span>
          </div>
          <div class="rounded-md bg-muted/30 px-3 py-2">
            <MarkdownBody :content="reply.body" />
          </div>
        </div>
      </div>

      <!-- Reply composer + resolve/unresolve -->
      <div class="border-t border-border pt-3">
        <ReplyComposer
          :thread-root-id="thread.root_id"
          @reply-sent="handleReplySent"
        >
          <template #leading>
            <Button
              variant="outline"
              size="sm"
              class="gap-2"
              @click="toggleResolved"
            >
              <CheckCircle v-if="!thread.resolved" class="h-4 w-4 text-green-600 dark:text-green-400" />
              <Circle v-else class="h-4 w-4" />
              {{ thread.resolved ? 'Unresolve' : 'Resolve' }}
            </Button>
          </template>
        </ReplyComposer>
      </div>

      <!-- End-of-queue message -->
      <div
        v-if="isAtEnd"
        class="rounded-md bg-muted/50 border border-border px-4 py-3 text-sm text-center text-muted-foreground"
      >
        All comments reviewed.
      </div>
    </div>
  </ScrollArea>
</template>
