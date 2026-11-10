<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronDown, ChevronRight, CheckCircle2, MessageSquare } from 'lucide-vue-next'
import type { model } from '@/wailsjs/go/models'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import MarkdownBody from '@/components/markdown/MarkdownBody.vue'
import ReplyComposer from '@/components/comment/ReplyComposer.vue'
import EditableComment from '@/components/comment/EditableComment.vue'
import CommentDiffHunk from '@/components/comment/CommentDiffHunk.vue'
import { useAvatarFallback } from '@/composables/useAvatarFallback'
import { taskToggler } from '@/lib/taskList'
import { markdownPreview } from '@/lib/markdown'

const props = defineProps<{
  thread: model.CommentThreadDTO
  language: string
}>()

const emit = defineEmits<{
  (e: 'resolve', rootId: number): void
  (e: 'unresolve', rootId: number): void
  (e: 'reply-sent', comment: model.ReviewCommentDTO): void
}>()

const collapsed = ref(props.thread.resolved)
const root = computed(() => props.thread.comments[0])
const preview = computed(() => markdownPreview(root.value?.body ?? ''))
const { avatarSrc, handleAvatarError, avatarInitial } = useAvatarFallback()

function formatTimestamp(iso: string): string {
  return iso ? new Date(iso).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : ''
}

function toggleResolved(): void {
  if (props.thread.resolved) {
    emit('unresolve', props.thread.root_id)
  } else {
    emit('resolve', props.thread.root_id)
    collapsed.value = true
  }
}
</script>

<template>
  <div class="mx-3 my-2 overflow-hidden rounded-md border border-border bg-card font-sans text-card-foreground shadow-sm">
    <button
      type="button"
      class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs hover:bg-accent/50"
      :aria-expanded="!collapsed"
      @click="collapsed = !collapsed"
    >
      <component :is="collapsed ? ChevronRight : ChevronDown" class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      <MessageSquare class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      <span class="font-medium">{{ root?.author_login ?? 'Unknown' }}</span>
      <span v-if="collapsed" class="min-w-0 flex-1 truncate text-muted-foreground">{{ preview }}</span>
      <span v-else class="flex-1" />
      <span v-if="thread.comments.length > 1" class="shrink-0 text-muted-foreground">
        {{ thread.comments.length - 1 }} repl{{ thread.comments.length === 2 ? 'y' : 'ies' }}
      </span>
      <Badge v-if="thread.outdated" variant="secondary" class="shrink-0 text-[0.625rem]">Outdated</Badge>
      <Badge v-if="thread.resolved" variant="secondary" class="shrink-0 gap-1 text-[0.625rem]">
        <CheckCircle2 class="h-3 w-3" /> Resolved
      </Badge>
    </button>

    <!-- v-show keeps an unsent reply when the thread is collapsed. -->
    <div v-show="!collapsed">
      <div v-if="thread.outdated && root?.diff_hunk" class="border-t border-border p-3">
        <CommentDiffHunk :diff-hunk="root.diff_hunk" :language="language" />
      </div>

      <div
        v-for="comment in thread.comments"
        :key="comment.id"
        class="border-t border-border px-3 py-2"
      >
        <div class="mb-1 flex items-center gap-2 text-xs">
          <img
            v-if="avatarSrc(`diff:${comment.id}`, comment.author_avatar)"
            :src="avatarSrc(`diff:${comment.id}`, comment.author_avatar)"
            :alt="comment.author_login"
            class="h-5 w-5 rounded-full"
            @error="handleAvatarError(`diff:${comment.id}`, comment.author_avatar)"
          />
          <span
            v-else
            class="inline-flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[0.625rem] font-semibold text-muted-foreground"
            aria-hidden="true"
          >{{ avatarInitial(comment.author_login) }}</span>
          <span class="font-medium">{{ comment.author_login }}</span>
          <span class="text-muted-foreground">{{ formatTimestamp(comment.created_at) }}</span>
        </div>
        <EditableComment kind="review_comment" :id="comment.id" :target="comment">
          <MarkdownBody :content="comment.body" :toggle-task="taskToggler('review_comment', comment.id, comment)" />
        </EditableComment>
      </div>

      <div class="border-t border-border bg-muted/30 px-3 py-2">
        <ReplyComposer :thread-root-id="thread.root_id" @reply-sent="emit('reply-sent', $event)">
          <template #leading>
            <Button variant="outline" size="sm" @click="toggleResolved">
              {{ thread.resolved ? 'Unresolve' : 'Resolve conversation' }}
            </Button>
          </template>
        </ReplyComposer>
      </div>
    </div>
  </div>
</template>
