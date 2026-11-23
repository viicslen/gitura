<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type * as model from '@/bindings/gitura/internal/model'
import CommentDiffHunk from './CommentDiffHunk.vue'
import MarkdownBody from '@/components/markdown/MarkdownBody.vue'
import CommentBody from './CommentBody.vue'
import ReplyComposer from './ReplyComposer.vue'
import EditableComment from './EditableComment.vue'
import SuggestionBlock from './SuggestionBlock.vue'
import DrawnCheck from './DrawnCheck.vue'
import SplitRunButton from '@/components/run/SplitRunButton.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { CheckCircle, Circle, FileCode, History } from '@lucide/vue'
import SwapBox from '@/components/motion/SwapBox.vue'
import { GetFileAtHead } from '@/bindings/gitura/internal/app/app'
import { currentCode, type CurrentCode } from '@/lib/currentCode'
import { errorMessage } from '@/lib/utils'
import { scrollToNext } from '@/lib/scrollJump'
import { langFromPath } from '@/lib/highlight'
import { useAvatarFallback } from '@/composables/useAvatarFallback'
import { taskToggler } from '@/lib/taskList'
import { collapse, collapsible, crossfade } from '@/components/motion/motion'

const props = defineProps<{
  thread: model.CommentThreadDTO | null
  /** Position of the thread in its list; decides which way a new thread comes in. */
  index: number
  isAtEnd: boolean
  /** No thread is open because every one is resolved. */
  allResolved?: boolean
  commands: model.CommandDTO[]
}>()

const emit = defineEmits<{
  (e: 'resolve', rootId: number): void
  (e: 'unresolve', rootId: number): void
  (e: 'reply-sent', comment: model.ReviewCommentDTO): void
  (e: 'suggestion-committed', result: model.SuggestionCommitResult): void
  (e: 'ran'): void
}>()

// A swap landing inside the previous one (held j/k, rapid clicks) cuts: stacked half-faded threads never become readable.
const CUT_WITHIN_MS = 150
const swap = reactive({ back: false, cut: false, at: 0 })
watch(() => [props.thread?.root_id, props.index] as const, ([rootId, index], [oldRootId, oldIndex]) => {
  if (rootId === oldRootId) return
  const now = performance.now()
  swap.cut = now - swap.at < CUT_WITHIN_MS
  swap.at = now
  swap.back = index < oldIndex
})

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
const { avatarSrc, handleAvatarError, avatarInitial } = useAvatarFallback()

const freshReplyId = ref(0)
function handleReplySent(comment: model.ReviewCommentDTO): void {
  freshReplyId.value = comment.id
  setTimeout(() => { if (freshReplyId.value === comment.id) freshReplyId.value = 0 }, 2400)
  emit('reply-sent', comment)
}

// Sticks across threads, so stepping through outdated threads keeps showing the code as it is now.
const showCurrent = ref(false)
// A LEFT-side thread is on the base file, which has no "now" to show.
const canShowCurrent = computed(() => !!props.thread?.outdated && !!props.thread.line && props.thread.side !== 'LEFT' && !!rootComment.value?.diff_hunk)
// Keyed on the thread object, which a PR refresh replaces, so a new head commit is fetched again.
const current = ref<{ thread: model.CommentThreadDTO; code?: CurrentCode; error?: string } | null>(null)
watch([() => props.thread, () => showCurrent.value && canShowCurrent.value], async ([thread, wanted]) => {
  const hunk = rootComment.value?.diff_hunk
  if (!wanted || !thread || !hunk || (current.value?.thread === thread && !current.value.error)) return
  let next: NonNullable<typeof current.value>
  try {
    next = { thread, code: currentCode(await GetFileAtHead(thread.path), hunk, thread.line) }
  } catch (err) {
    const msg = errorMessage(err)
    next = { thread, error: msg.startsWith('notfound:') ? 'This file is no longer at this path; it was deleted or renamed.' : `Could not load the file: ${msg}` }
  }
  if (props.thread === thread) current.value = next
}, { immediate: true })
const shownCurrent = computed(() => (current.value?.thread === props.thread ? current.value : null))

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

function rootAvatarKey(): string {
  return `thread-root:${props.thread?.root_id ?? 0}`
}

function replyAvatarKey(replyId: number): string {
  return `thread-reply:${props.thread?.root_id ?? 0}:${replyId}`
}

const scroller = ref<InstanceType<typeof ScrollArea> | null>(null)
/** Scrolls to the next (or previous) comment in the thread; false when there's none that way. */
function jumpComment(dir: 1 | -1): boolean {
  const root: unknown = scroller.value?.$el
  const viewport = root instanceof Element ? root.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]') : null
  if (!viewport) return false
  const top = viewport.getBoundingClientRect().top - viewport.scrollTop
  const tops = Array.from(viewport.querySelectorAll('[data-thread-comment]'), (c) => c.getBoundingClientRect().top - top)
  return scrollToNext(viewport, tops, dir, 16)
}
defineExpose({ jumpComment })</script>

<template>
  <div class="grid min-h-0 flex-1 grid-cols-1 grid-rows-1 overflow-hidden">
    <!-- A deck of cards: the open thread drops back and the next one comes forward, from below or from above. -->
    <Transition :css="!swap.cut" :name="swap.back ? 'deck-back' : 'deck'">
      <div
        v-if="!thread"
        :key="allResolved ? 'done' : 'none'"
        class="relative overflow-hidden [grid-area:1/1] flex flex-col items-center justify-center gap-3 text-muted-foreground text-sm p-6"
      >
        <template v-if="allResolved">
          <span class="relative grid size-24 place-items-center rounded-full text-wow-2 wow-pop wow-ring"><DrawnCheck ring class="size-20" /></span>
          <p class="relative text-xl font-semibold text-foreground wow-focus-in">All conversations resolved</p>
          <p class="relative wow-rise [--i:6]">Nothing on this pull request is waiting for an answer.</p>
        </template>
        <template v-else>Select a comment to begin.</template>
      </div>

      <!-- Keyed so each thread comes in on its own and starts at the top. -->
      <ScrollArea v-else ref="scroller" :key="thread.root_id" class="[grid-area:1/1] min-h-0">
        <div class="p-4">
          <!-- Fixed height: the badge is taller than the path and must not push the thread down. -->
          <div class="flex h-6 items-center gap-2 text-xs text-muted-foreground">
            <span class="font-mono truncate">{{ thread.path }}<span v-if="thread.line">:{{ thread.line }}</span></span>
            <Badge v-if="thread.outdated" variant="outline" class="shrink-0 text-xs">Outdated</Badge>
            <!-- The explicit duration lets the ring finish: left to itself the enter ends with the shorter pop. -->
            <Transition
              enter-active-class="wow-pop wow-ring"
              leave-active-class="transition-[opacity,scale] duration-faster"
              leave-to-class="opacity-0 scale-90"
              :duration="{ enter: 700, leave: 80 }"
            >
              <span v-if="thread.resolved" class="relative inline-flex shrink-0 rounded-full">
                <Badge variant="secondary" class="text-xs opacity-70"><DrawnCheck /> Resolved</Badge>
              </span>
            </Transition>
            <Button
              v-if="canShowCurrent"
              variant="ghost"
              size="sm"
              class="ml-auto h-6 shrink-0 gap-1.5 px-2 text-xs"
              :aria-pressed="showCurrent"
              @click="showCurrent = !showCurrent"
            >
              <component :is="showCurrent ? History : FileCode" class="h-3.5 w-3.5" />
              {{ showCurrent ? 'Show original diff' : 'Show current code' }}
            </Button>
          </div>

          <div v-if="rootComment" data-thread-comment class="mt-4 space-y-2">
            <EditableComment kind="review_comment" :id="rootComment.id" :target="rootComment" class="space-y-2">
              <template #header>
              <img
                v-if="rootComment.author_avatar && avatarSrc(rootAvatarKey(), rootComment.author_avatar)"
                :src="avatarSrc(rootAvatarKey(), rootComment.author_avatar)"
                :alt="rootComment.author_login"
                class="w-6 h-6 rounded-full bg-muted"
                @error="handleAvatarError(rootAvatarKey(), rootComment.author_avatar)"
              />
              <span
                v-else
                class="w-6 h-6 rounded-full bg-muted text-xs font-semibold text-muted-foreground inline-flex items-center justify-center"
                :title="rootComment.author_login"
                aria-hidden="true"
              >
                {{ avatarInitial(rootComment.author_login) }}
              </span>
              <span class="text-sm font-medium">{{ rootComment.author_login }}</span>
              <span class="text-xs text-muted-foreground">{{ formatTimestamp(rootComment.created_at) }}</span>
              <Badge v-if="rootComment.is_suggestion" variant="outline" class="text-xs">
                Suggestion
              </Badge>
              </template>
              <div class="relative rounded-md bg-muted/40 px-3 py-2 group/body">
                <CommentBody
                  :content="rootComment.body"
                  :commands="commands"
                  :thread-root-id="thread.root_id"
                  :comment-id="rootComment.id"
                  :toggle-task="taskToggler('review_comment', rootComment.id, rootComment)"
                  @ran="emit('ran')"
                />
                <!-- Stays visible while its own menu is open, when the pointer has left the comment. -->
                <div
                  v-if="commands.length > 0"
                  class="absolute top-2 right-2 opacity-0 transition-opacity duration-faster group-hover/body:opacity-100 group-hover/body:duration-0 focus-within:opacity-100 focus-within:duration-0 has-[[data-state=open]]:opacity-100"
                >
                  <SplitRunButton
                    :commands="commands"
                    kind="comment"
                    :input="rootComment.body"
                    :thread-root-id="thread.root_id"
                    :comment-id="rootComment.id"
                    size="sm"
                    @ran="emit('ran')"
                  />
                </div>
              </div>
            </EditableComment>

            <SwapBox v-if="rootComment.diff_hunk">
              <div v-if="showCurrent && canShowCurrent" :key="shownCurrent ? 'current' : 'loading'">
                <p v-if="!shownCurrent" class="px-1 py-2 text-xs text-muted-foreground">Loading the current code…</p>
                <p v-else-if="shownCurrent.error" class="px-1 py-2 text-xs text-destructive">{{ shownCurrent.error }}</p>
                <template v-else-if="shownCurrent.code">
                  <p v-if="!shownCurrent.code.markedLine" class="px-1 pb-1 text-xs text-muted-foreground">
                    The commented line is no longer in the file; showing the code around where it was.
                  </p>
                  <CommentDiffHunk
                    :diff-hunk="shownCurrent.code.hunk"
                    :marked-line="shownCurrent.code.markedLine"
                    :language="langFromPath(thread.path)"
                  />
                </template>
              </div>
              <CommentDiffHunk
                v-else
                key="original"
                :diff-hunk="rootComment.diff_hunk"
                :marked-line="thread.line || undefined"
                :language="langFromPath(thread.path)"
              />
            </SwapBox>

            <SuggestionBlock
              v-if="rootComment.is_suggestion"
              :comment="rootComment"
              @committed="handleSuggestionCommitted"
            />
          </div>

          <!-- Block layout with margins on the replies, not space-y: a reply opening or closing takes its gap with it. -->
          <TransitionGroup tag="div" class="border-l-2 border-border pl-4" v-bind="collapse">
            <div v-for="reply in replies" :key="reply.id" data-thread-comment class="mt-3 first:mt-4" :class="collapsible">
              <EditableComment kind="review_comment" :id="reply.id" :target="reply" class="min-h-0 space-y-1" :class="reply.id === freshReplyId && 'wow-rise'">
                <template #header>
                  <img
                    v-if="reply.author_avatar && avatarSrc(replyAvatarKey(reply.id), reply.author_avatar)"
                    :src="avatarSrc(replyAvatarKey(reply.id), reply.author_avatar)"
                    :alt="reply.author_login"
                    class="w-5 h-5 rounded-full bg-muted"
                    @error="handleAvatarError(replyAvatarKey(reply.id), reply.author_avatar)"
                  />
                  <span
                    v-else
                    class="w-5 h-5 rounded-full bg-muted text-[0.625rem] font-semibold text-muted-foreground inline-flex items-center justify-center"
                    :title="reply.author_login"
                    aria-hidden="true"
                  >
                    {{ avatarInitial(reply.author_login) }}
                  </span>
                  <span class="text-sm font-medium">{{ reply.author_login }}</span>
                  <span class="text-xs text-muted-foreground">{{ formatTimestamp(reply.created_at) }}</span>
                </template>
                <div class="relative rounded-md bg-muted/30 px-3 py-2" :class="reply.id === freshReplyId && 'fresh-reply wow-glow'">
                  <MarkdownBody :content="reply.body" :toggle-task="taskToggler('review_comment', reply.id, reply)" />
                </div>
              </EditableComment>
            </div>
          </TransitionGroup>

          <div class="mt-4 border-t border-border pt-3">
            <ReplyComposer
              :thread-root-id="thread.root_id"
              @reply-sent="handleReplySent"
            >
              <template #leading>
                <Button
                  variant="outline"
                  size="sm"
                  @click="toggleResolved"
                >
                  <!-- The pseudo-element reserves the wider label, so the buttons beside this one stay put. -->
                  <span class="grid justify-items-center after:invisible after:h-0 after:pl-6 after:content-['Unresolve'] after:[grid-area:1/1]">
                    <Transition v-bind="crossfade">
                      <span :key="String(thread.resolved)" class="flex items-center gap-2 [grid-area:1/1]">
                        <Circle v-if="thread.resolved" class="h-4 w-4" />
                        <CheckCircle v-else class="h-4 w-4 text-green-600 dark:text-green-400" />
                        {{ thread.resolved ? 'Unresolve' : 'Resolve' }}
                      </span>
                    </Transition>
                  </span>
                </Button>
                <SplitRunButton
                  v-if="commands.length > 0 && rootComment"
                  :commands="commands"
                  kind="comment"
                  :input="rootComment.body"
                  :thread-root-id="thread.root_id"
                  :comment-id="rootComment.id"
                  label="Run comment"
                  @ran="emit('ran')"
                />
              </template>
            </ReplyComposer>
          </div>

          <Transition v-bind="collapse">
            <div v-if="isAtEnd" class="mt-4" :class="collapsible">
              <div class="min-h-0">
                <div class="rounded-md bg-muted/50 border border-border px-4 py-3 text-sm text-center text-muted-foreground">
                  This is the last thread.
                </div>
              </div>
            </div>
          </Transition>
        </div>
      </ScrollArea>
    </Transition>
  </div>
</template>

<style scoped>
.deck-enter-active,
.deck-back-enter-active {
  z-index: 1;
  transition: opacity var(--transition-duration-base) var(--ease-out), transform var(--transition-duration-slower) var(--ease-spring);
}
.deck-enter-from {
  opacity: 0;
  transform: perspective(60rem) translateY(3.5rem) rotateX(-10deg) scale(1.05);
}
.deck-back-enter-from {
  opacity: 0;
  transform: perspective(60rem) translateY(-3.5rem) rotateX(10deg) scale(1.05);
}
.deck-leave-active,
.deck-back-leave-active {
  pointer-events: none;
  transition: opacity var(--transition-duration-base) var(--ease-out), transform var(--transition-duration-base) var(--ease-out);
}
.deck-leave-to,
.deck-back-leave-to {
  opacity: 0;
  transform: perspective(60rem) translateZ(-14rem);
}
/* The glow of a reply just sent fades out by itself, so taking the class off later changes nothing on screen. */
.fresh-reply::before {
  animation: wow-spin 2.4s linear infinite, fresh-reply-glow 2.2s var(--ease-out) forwards;
}
@keyframes fresh-reply-glow {
  0%, 60% { opacity: 1; }
  100% { opacity: 0; }
}
</style>
