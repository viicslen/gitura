<script setup lang="ts">
// One row per component, so selecting a thread re-renders two rows and not the whole list.
import { computed } from 'vue'
import type * as model from '@/bindings/gitura/internal/model'
import { Badge } from '@/components/ui/badge'
import { Loader2, CheckCircle2, XCircle, MinusCircle } from '@lucide/vue'
import { useRuns } from '@/composables/useRuns'
import { useAvatarFallback } from '@/composables/useAvatarFallback'
import { collapseX, collapsibleX, scaleIn } from '@/components/motion/motion'

const props = defineProps<{
  thread: model.CommentThreadDTO
  index: number
  selected: boolean
  reviewLabel: string
}>()

const emit = defineEmits<{
  (e: 'select', index: number): void
  /** Arrow keys: the index the selection should move to. */
  (e: 'move', index: number): void
}>()

const { runsForThread } = useRuns()
const { avatarSrc, handleAvatarError, avatarInitial } = useAvatarFallback()

const root = computed(() => props.thread.comments?.[0] ?? null)
const avatarKey = computed(() => `thread:${props.thread.root_id}`)
const avatar = computed(() => (root.value?.author_avatar ? avatarSrc(avatarKey.value, root.value.author_avatar) : ''))

const excerpt = computed(() => {
  const trimmed = (root.value?.body ?? '').replace(/\n/g, ' ').trim()
  return trimmed.length > 200 ? trimmed.slice(0, 200) + '…' : trimmed
})

const runIcons = {
  running: { icon: Loader2, class: 'text-muted-foreground animate-spin' },
  success: { icon: CheckCircle2, class: 'text-green-500' },
  error: { icon: XCircle, class: 'text-destructive' },
  cancelled: { icon: MinusCircle, class: 'text-muted-foreground' },
}

const lastRun = computed(() => {
  const run = runsForThread(props.thread.root_id).value[0]
  if (!run) return null
  const status = run.running ? 'running' : run.cancelled ? 'cancelled' : run.exit_code === 0 ? 'success' : 'error'
  return { status, ...runIcons[status] }
})

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
    event.preventDefault()
    emit('move', props.index + (event.key === 'ArrowUp' ? -1 : 1))
  } else if (event.key === 'Enter') {
    emit('select', props.index)
  }
}
</script>

<template>
  <button
    :id="`thread-item-${index}`"
    role="option"
    :aria-selected="selected"
    class="block w-full text-left px-3 py-2.5 border-b border-border focus:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer"
    :class="selected ? 'bg-accent text-accent-foreground' : 'hover:bg-muted/50'"
    @click="emit('select', index)"
    @keydown="handleKeydown"
  >
    <div class="flex items-start gap-2 min-w-0">
      <div class="flex flex-col items-center shrink-0 gap-0.5 mt-0.5">
        <img
          v-if="avatar"
          :src="avatar"
          :alt="root?.author_login"
          class="w-5 h-5 rounded-full bg-muted"
          @error="handleAvatarError(avatarKey, root?.author_avatar ?? '')"
        />
        <span
          v-else
          class="w-5 h-5 rounded-full bg-background/95 border border-border text-[0.625rem] font-semibold text-foreground inline-flex items-center justify-center shadow-sm"
          :title="root?.author_login ?? 'Unknown'"
          aria-hidden="true"
        >
          {{ avatarInitial(root?.author_login ?? '') }}
        </span>
        <!-- Last run of the thread; one cell, so a new status crossfades over the old one. -->
        <span class="grid size-3">
          <Transition v-bind="scaleIn">
            <component :is="lastRun.icon" v-if="lastRun" :key="lastRun.status" class="size-3 [grid-area:1/1]" :class="lastRun.class" />
          </Transition>
        </span>
      </div>
      <div class="min-w-0 flex-1 pr-2">
        <!-- Fixed height: the Resolved badge is the tallest item and must not grow the row when it comes in. -->
        <div class="flex h-6 items-center gap-2 min-w-0">
          <span class="font-medium text-sm truncate max-w-[120px]">
            {{ root?.author_login ?? 'Unknown' }}
          </span>
          <span class="flex-1" />
          <!-- transition-none: the row's highlight is instant, and so is the badge that follows it. -->
          <Badge
            variant="secondary"
            :class="[
              'text-[0.625rem] font-normal shrink-0 border transition-none',
              selected
                ? 'bg-background/90 border-border text-foreground'
                : 'bg-muted border-transparent text-muted-foreground',
            ]"
          >
            {{ reviewLabel }}
          </Badge>
          <Transition v-bind="collapseX('-ml-2!')">
            <span v-if="thread.resolved" class="shrink-0" :class="collapsibleX">
              <span class="flex min-w-0">
                <Badge variant="outline" class="text-xs text-muted-foreground border-muted-foreground/40">Resolved</Badge>
              </span>
            </span>
          </Transition>
        </div>
        <div class="text-xs text-muted-foreground truncate min-w-0">
          {{ thread.path }}<span v-if="thread.line" class="text-muted-foreground/70">:{{ thread.line }}</span>
        </div>
        <!-- Two lines reserved: an edited comment must not change the height of its row and move the rows below. -->
        <p class="text-xs text-muted-foreground mt-0.5 min-h-8 line-clamp-2">
          {{ excerpt }}
        </p>
      </div>
    </div>
  </button>
</template>
