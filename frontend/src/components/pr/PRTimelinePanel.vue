<script setup lang="ts">
import { computed } from 'vue'
import { Check, CornerDownRight, Eye, GitCommitHorizontal, MessageSquare, MessageSquareCode, X } from '@lucide/vue'
import type * as model from '@/bindings/gitura/internal/model'
import { Badge } from '@/components/ui/badge'
import { buildTimeline, type TimelineEntry } from '@/lib/timeline'

const props = defineProps<{
  timeline: model.PRTimelineDTO | null
  threads: model.CommentThreadDTO[]
}>()

const entries = computed(() => (props.timeline ? buildTimeline(props.timeline, props.threads) : []))

const REVIEW_VERBS: Record<string, string> = { APPROVED: 'approved', CHANGES_REQUESTED: 'requested changes', DISMISSED: 'reviewed (dismissed)' }

function icon(e: TimelineEntry) {
  if (e.kind === 'commit') return GitCommitHorizontal
  if (e.kind === 'comment') return MessageSquare
  if (e.kind === 'thread') return e.reply ? CornerDownRight : MessageSquareCode
  return e.review.state === 'APPROVED' ? Check : e.review.state === 'CHANGES_REQUESTED' ? X : Eye
}

function author(e: TimelineEntry): string {
  return e.kind === 'commit' ? e.commit.author_login : e.kind === 'review' ? e.review.author_login : e.comment.author_login
}

function action(e: TimelineEntry): string {
  if (e.kind === 'commit') return 'committed'
  if (e.kind === 'comment') return 'commented'
  if (e.kind === 'review') return `${REVIEW_VERBS[e.review.state] ?? 'reviewed'} · Review ${e.number}`
  return `${e.reply ? 'replied on' : 'commented on'} ${e.thread.path}${e.thread.line ? `:${e.thread.line}` : ''}`
}

function excerpt(e: TimelineEntry): string {
  const body = e.kind === 'commit' ? e.commit.headline : e.kind === 'review' ? e.review.body : e.comment.body
  return body.trim().split('\n')[0].replace(/[*`#>]+/g, '').trim()
}

function key(e: TimelineEntry): string {
  return e.kind === 'commit' ? `c:${e.commit.sha}` : e.kind === 'review' ? `r:${e.review.review_id}` : `${e.kind}:${e.comment.id}`
}

function formatTimestamp(iso: string): string {
  return iso ? new Date(iso).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : ''
}
</script>

<template>
  <div>
    <p v-if="!timeline" class="py-6 text-center text-sm text-muted-foreground animate-in fade-in-0 animation-duration-base delay-200 fill-mode-backwards">
      Loading timeline…
    </p>
    <ol v-else aria-label="Pull request timeline">
      <li
        v-for="e in entries"
        :key="key(e)"
        class="relative flex gap-3 pb-3 before:absolute before:top-7 before:bottom-0 before:left-3.5 before:w-px before:bg-border last:before:hidden"
        :class="e.kind === 'thread' && e.reply && 'pl-6 before:left-9.5'"
      >
        <span
          class="relative grid size-7 shrink-0 place-items-center rounded-full border"
          :class="e.kind === 'commit' ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-muted text-muted-foreground'"
        >
          <component :is="icon(e)" class="size-3.5" />
        </span>
        <div class="min-w-0 flex-1 pt-1">
          <div class="flex items-center gap-2 text-sm">
            <span class="font-medium">{{ author(e) }}</span>
            <span class="min-w-0 truncate text-muted-foreground">{{ action(e) }}</span>
            <template v-if="e.kind === 'thread' && !e.reply">
              <Badge v-if="e.thread.resolved" variant="secondary" class="shrink-0">Resolved</Badge>
              <Badge v-if="e.thread.outdated" variant="outline" class="shrink-0">Outdated</Badge>
            </template>
            <span class="ml-auto shrink-0 text-xs text-muted-foreground tabular-nums" :title="e.at">{{ formatTimestamp(e.at) }}</span>
          </div>
          <p v-if="excerpt(e)" class="mt-0.5 truncate text-xs text-muted-foreground">
            <a v-if="e.kind === 'commit'" :href="e.commit.html_url" class="mr-1.5 font-mono text-foreground hover:underline">{{ e.commit.sha.slice(0, 7) }}</a>{{ excerpt(e) }}
          </p>
        </div>
      </li>
      <li v-if="entries.length === 0" class="text-sm text-muted-foreground">Nothing has happened on this pull request yet.</li>
    </ol>
  </div>
</template>
