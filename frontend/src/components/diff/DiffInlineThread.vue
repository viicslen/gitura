<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ChevronRight, CheckCircle2, MessageSquare } from '@lucide/vue'
import type * as model from '@/bindings/gitura/internal/model'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import MarkdownBody from '@/components/markdown/MarkdownBody.vue'
import ReplyComposer from '@/components/comment/ReplyComposer.vue'
import EditableComment from '@/components/comment/EditableComment.vue'
import CommentDiffHunk from '@/components/comment/CommentDiffHunk.vue'
import { useAvatarFallback } from '@/composables/useAvatarFallback'
import { taskToggler } from '@/lib/taskList'
import { markdownPreview } from '@/lib/markdown'
import GlideList from '@/components/motion/GlideList.vue'
import Reveal from '@/components/motion/Reveal.vue'

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
// Comments the thread opens with don't animate, replies added later do.
let settled = false
onMounted(() => { settled = true })

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
  <!-- No shadow, and overflow-clip not overflow-hidden: both made scrolling repaint slowly under WebKitGTK's software rendering. -->
  <div class="mx-3 my-2 overflow-clip rounded-md border border-border bg-card font-sans text-card-foreground">
    <button
      type="button"
      class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs transition-colors duration-faster hover:bg-accent/50 hover:duration-0"
      :aria-expanded="!collapsed"
      @click="collapsed = !collapsed"
    >
      <ChevronRight class="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform duration-fast" :class="{ 'rotate-90': !collapsed }" />
      <MessageSquare class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      <span class="font-medium">{{ root?.author_login ?? 'Unknown' }}</span>
      <span class="min-w-0 flex-1 truncate text-muted-foreground transition-opacity duration-fast" :class="{ 'opacity-0': !collapsed }" :aria-hidden="!collapsed">{{ preview }}</span>
      <Reveal axis="x" :open="thread.comments.length > 1" class="shrink-0 text-muted-foreground" :class="{ '-ml-2': thread.comments.length < 2 }">
        {{ Math.max(thread.comments.length - 1, 1) }} repl{{ thread.comments.length > 2 ? 'ies' : 'y' }}
      </Reveal>
      <Badge v-if="thread.outdated" variant="secondary" class="shrink-0 text-[0.625rem]">Outdated</Badge>
      <Reveal axis="x" :open="thread.resolved" class="shrink-0" :class="{ '-ml-2': !thread.resolved }">
        <Badge variant="secondary" class="gap-1 text-[0.625rem]">
          <CheckCircle2 class="h-3 w-3" /> Resolved
        </Badge>
      </Reveal>
    </button>

    <!-- Stays mounted while collapsed, which keeps an unsent reply. -->
    <Reveal axis="y" :open="!collapsed">
      <div v-if="thread.outdated && root?.diff_hunk" class="border-t border-border p-3">
        <CommentDiffHunk :diff-hunk="root.diff_hunk" :language="language" />
      </div>

      <GlideList v-slot="{ item: comment }" :items="thread.comments" :item-key="(c) => c.id" :appear="settled">
        <div class="grid">
          <div class="min-h-0">
            <div class="border-t border-border px-3 py-2">
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
          </div>
        </div>
      </GlideList>

      <div class="border-t border-border bg-muted/30 px-3 py-2">
        <ReplyComposer :thread-root-id="thread.root_id" @reply-sent="emit('reply-sent', $event)">
          <template #leading>
            <Button variant="outline" size="sm" @click="toggleResolved">
              <!-- Both labels share one cell, so the button keeps its width when they swap. -->
              <span class="grid text-center">
                <span
                  v-for="(label, resolved) in ['Resolve conversation', 'Unresolve']"
                  :key="label"
                  class="[grid-area:1/1] transition-[opacity,visibility] duration-fast"
                  :class="{ 'invisible opacity-0': thread.resolved !== !!resolved }"
                >{{ label }}</span>
              </span>
            </Button>
          </template>
        </ReplyComposer>
      </div>
    </Reveal>
  </div>
</template>
