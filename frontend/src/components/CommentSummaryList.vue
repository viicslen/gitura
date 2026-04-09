<script setup lang="ts">
import { computed } from 'vue'
import type { model } from '../wailsjs/go/models'
import { Badge } from '@/components/ui/badge'
import { Loader2, CheckCircle2, XCircle, MinusCircle } from 'lucide-vue-next'
import { useRuns } from '@/composables/useRuns'
import { useAvatarFallback } from '@/composables/useAvatarFallback'

const props = defineProps<{
  threads: model.CommentThreadDTO[]
  currentIndex: number
  showResolved: boolean
  commands: model.CommandDTO[]
  defaultCommandId: string
}>()

const { runsForThread } = useRuns()
const { avatarSrc, handleAvatarError, avatarInitial } = useAvatarFallback()

const emit = defineEmits<{
  (e: 'select', index: number): void
  (e: 'ran'): void
}>()

const visibleThreads = computed(() =>
  props.showResolved
    ? props.threads
    : props.threads.filter((t) => !t.resolved)
)

function rootComment(thread: model.CommentThreadDTO): model.CommentDTO | null {
  return thread.comments?.[0] ?? null
}

function excerpt(body: string): string {
  if (!body) return ''
  const trimmed = body.replace(/\n/g, ' ').trim()
  return trimmed.length > 200 ? trimmed.slice(0, 200) + '…' : trimmed
}

function lastRunStatus(thread: model.CommentThreadDTO): 'running' | 'success' | 'error' | 'cancelled' | null {
  const runs = runsForThread(thread.root_id).value
  if (runs.length === 0) return null
  const run = runs[0]
  if (run.running) return 'running'
  if (run.cancelled) return 'cancelled'
  if (run.exit_code === 0) return 'success'
  return 'error'
}

function handleKeydown(event: KeyboardEvent, index: number): void {
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    if (index > 0) emit('select', index - 1)
  } else if (event.key === 'ArrowDown') {
    event.preventDefault()
    if (index < visibleThreads.value.length - 1) emit('select', index + 1)
  } else if (event.key === 'Enter') {
    emit('select', index)
  }
}

function threadAvatarKey(thread: model.CommentThreadDTO): string {
  return `thread:${thread.root_id}`
}
</script>

<template>
  <div
    role="listbox"
    :aria-activedescendant="visibleThreads.length > 0 ? `thread-item-${currentIndex}` : undefined"
    aria-label="Comment threads"
    class="flex flex-col overflow-y-auto h-full"
  >
    <div
      v-if="visibleThreads.length === 0"
      class="flex-1 flex items-center justify-center text-muted-foreground text-sm p-4"
    >
      No comments to show.
    </div>

    <button
      v-for="(thread, index) in visibleThreads"
      :id="`thread-item-${index}`"
      :key="thread.root_id"
      role="option"
      :aria-selected="index === currentIndex"
      class="block w-full text-left px-3 py-2.5 border-b border-border focus:outline-none focus-visible:ring-2 focus-visible:ring-ring transition-colors cursor-pointer"
      :class="[
        index === currentIndex
          ? 'bg-accent text-accent-foreground'
          : 'hover:bg-muted/50',
      ]"
      @click="emit('select', index)"
      @keydown="handleKeydown($event, index)"
    >
      <div class="flex items-start gap-2 min-w-0">
        <!-- Avatar + run status indicator -->
        <div class="flex flex-col items-center shrink-0 gap-0.5 mt-0.5">
          <img
            v-if="rootComment(thread)?.author_avatar && avatarSrc(threadAvatarKey(thread), rootComment(thread)!.author_avatar)"
            :src="avatarSrc(threadAvatarKey(thread), rootComment(thread)!.author_avatar)"
            :alt="rootComment(thread)!.author_login"
            class="w-5 h-5 rounded-full"
            @error="handleAvatarError(threadAvatarKey(thread), rootComment(thread)!.author_avatar)"
          />
          <span
            v-else
            class="w-5 h-5 rounded-full bg-background/95 border border-border text-[10px] font-semibold text-foreground inline-flex items-center justify-center shadow-sm"
            :title="rootComment(thread)?.author_login ?? 'Unknown'"
            aria-hidden="true"
          >
            {{ avatarInitial(rootComment(thread)?.author_login ?? '') }}
          </span>
          <Loader2
            v-if="lastRunStatus(thread) === 'running'"
            class="w-3 h-3 text-muted-foreground animate-spin"
          />
          <CheckCircle2
            v-else-if="lastRunStatus(thread) === 'success'"
            class="w-3 h-3 text-green-500"
          />
          <XCircle
            v-else-if="lastRunStatus(thread) === 'error'"
            class="w-3 h-3 text-destructive"
          />
          <MinusCircle
            v-else-if="lastRunStatus(thread) === 'cancelled'"
            class="w-3 h-3 text-muted-foreground"
          />
        </div>
        <div class="min-w-0 flex-1 pr-2">
          <!-- Author row: name on left, Resolved badge on right -->
          <div class="flex items-center gap-2 min-w-0">
            <span class="font-medium text-sm truncate max-w-[120px]">
              {{ rootComment(thread)?.author_login ?? 'Unknown' }}
            </span>
            <span class="flex-1" />
            <Badge
              v-if="thread.resolved && showResolved"
              variant="outline"
              class="text-xs shrink-0 text-muted-foreground border-muted-foreground/40"
            >
              Resolved
            </Badge>
          </div>
          <!-- File path -->
          <div class="text-xs text-muted-foreground truncate min-w-0">
            {{ thread.path }}<span v-if="thread.line" class="text-muted-foreground/70">:{{ thread.line }}</span>
          </div>
          <!-- Excerpt -->
          <p class="text-xs text-muted-foreground mt-0.5 line-clamp-2">
            {{ excerpt(rootComment(thread)?.body ?? '') }}
          </p>
        </div>
      </div>
    </button>
  </div>
</template>
