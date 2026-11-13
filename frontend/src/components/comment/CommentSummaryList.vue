<script setup lang="ts">
import { computed } from 'vue'
import type * as model from '@/bindings/gitura/internal/model'
import { Button } from '@/components/ui/button'
import CommentSummaryRow from './CommentSummaryRow.vue'

const props = defineProps<{
  threads: model.CommentThreadDTO[]
  /** Threads on the PR under any filter; tells "all resolved" from "none at all". */
  total: number
  currentIndex: number
  showResolved: boolean
  commands: model.CommandDTO[]
  defaultCommandName: string
  /** PR-wide review numbers, so labels match the Overview and don't shift with filters. */
  reviewNumbers?: model.PROverviewDTO['review_numbers']
}>()

const emit = defineEmits<{
  (e: 'select', index: number): void
  (e: 'show-all'): void
  (e: 'ran'): void
}>()

const visibleThreads = computed(() =>
  props.showResolved
    ? props.threads
    : props.threads.filter((t) => !t.resolved)
)

// Rows glide (fade in and out, slide to their new place) only up to this many threads on the PR: every moved row is
// animated on its own, and a filter change dropped frames from about 36 threads up. A longer list cuts, and fades in as
// one block on a filter change: the two classes below are the same fade under two animation names, so switching
// between them replays it without remounting hundreds of rows.
const GLIDE_MAX_THREADS = 30
const rowMotion = computed(() =>
  props.total <= GLIDE_MAX_THREADS
    ? {
        enterActiveClass: 'transition-opacity',
        enterFromClass: 'opacity-0',
        leaveActiveClass: 'absolute transition-opacity duration-fast pointer-events-none',
        leaveToClass: 'opacity-0',
        moveClass: 'transition-transform ease-in-out',
      }
    : {
        css: false,
        class: props.showResolved
          ? 'animate-in fade-in-0 animation-duration-base'
          : 'animate-out fade-out-0 direction-reverse animation-duration-base',
      },
)

function select(index: number): void {
  emit('select', index)
}

function move(target: number): void {
  if (target < 0 || target >= visibleThreads.value.length) return
  emit('select', target)
  document.getElementById(`thread-item-${target}`)?.focus()
}

function reviewLabel(thread: model.CommentThreadDTO): string {
  const number = props.reviewNumbers?.[`${thread.review_id}`]
  return number ? `Review ${number}` : 'Single comment'
}
</script>

<template>
  <TransitionGroup
    tag="div"
    role="listbox"
    :aria-activedescendant="visibleThreads.length > 0 ? `thread-item-${currentIndex}` : undefined"
    aria-label="Comment threads"
    class="relative h-full overflow-y-auto"
    v-bind="rowMotion"
  >
    <div
      v-if="visibleThreads.length === 0"
      key="empty"
      class="h-full w-full flex flex-col items-center justify-center gap-2 p-4 text-center text-sm text-muted-foreground"
    >
      <template v-if="!showResolved && total > 0">
        All {{ total }} threads are resolved.
        <Button variant="outline" size="sm" class="h-7 text-xs" @click="emit('show-all')">Show all threads</Button>
      </template>
      <template v-else>No comments to show.</template>
    </div>

    <!-- Handlers are passed by reference: an inline arrow would be new on every render and re-render every row. -->
    <CommentSummaryRow
      v-for="(thread, index) in visibleThreads"
      :key="thread.root_id"
      :thread="thread"
      :index="index"
      :selected="index === currentIndex"
      :review-label="reviewLabel(thread)"
      @select="select"
      @move="move"
    />
  </TransitionGroup>
</template>
