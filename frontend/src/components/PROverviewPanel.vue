<script setup lang="ts">
import { computed } from 'vue'
import type { model } from '@/wailsjs/go/models'
import StaticMarkdownBody from '@/components/StaticMarkdownBody.vue'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { useAvatarFallback } from '@/composables/useAvatarFallback'

const props = defineProps<{
  overview: model.PROverviewDTO | null
}>()

const { avatarSrc, handleAvatarError, avatarInitial } = useAvatarFallback()

const CARD_VISIBILITY_STYLE = {
  contentVisibility: 'auto',
  containIntrinsicSize: '320px',
}

const visibleGeneralComments = computed(() => props.overview?.general_comments ?? [])

const visibleReviews = computed(() => props.overview?.reviews ?? [])

const reviewIndexById = computed(() => {
  const indices = new Map<number, number>()
  let nextIndex = 1
  for (const review of visibleReviews.value) {
    if (review.review_id <= 0 || indices.has(review.review_id)) continue
    indices.set(review.review_id, nextIndex)
    nextIndex++
  }
  return indices
})

const cardClass = computed(() => [
  'gap-4 py-4',
  'shadow-none',
])

const surfaceClass = 'rounded-lg bg-muted/30 px-4 py-3'

const avatarFallbackClass = 'inline-flex h-7 w-7 items-center justify-center rounded-full bg-muted text-xs font-semibold text-muted-foreground'

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

function reviewLabel(review: model.ReviewOverviewDTO): string {
  const index = reviewIndexById.value.get(review.review_id)
  return index ? `Review ${index}` : 'Review'
}

function reviewStateLabel(review: model.ReviewOverviewDTO): string {
  return review.state.replaceAll('_', ' ').toLowerCase()
}

function avatarKey(prefix: string, id: number): string {
  return `${prefix}:${id}`
}

</script>

<template>
  <div v-if="!overview" class="flex-1 flex items-center justify-center text-sm text-muted-foreground">
    No overview data available.
  </div>

  <div v-else class="flex-1 overflow-y-auto">
    <div class="p-4 space-y-4">
      <Card class="shadow-none">
        <CardContent>
          <div v-if="overview.summary.body">
            <StaticMarkdownBody
              :content="overview.summary.body"
              :base-url="overview.summary.html_url"
              :repo-owner="overview.summary.head_owner || overview.summary.owner"
              :repo-name="overview.summary.head_repo || overview.summary.repo"
              :repo-ref="overview.summary.head_sha"
            />
          </div>
          <p v-else class="text-sm text-muted-foreground">No pull request description.</p>
        </CardContent>
      </Card>

      <section class="space-y-3">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-semibold">General comments</h2>
          <span class="text-xs text-muted-foreground">{{ visibleGeneralComments.length }}/{{ overview.general_comments.length }}</span>
        </div>

        <div v-if="visibleGeneralComments.length === 0" class="rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
          No general pull request comments.
        </div>

        <Card v-for="comment in visibleGeneralComments" :key="comment.id" :style="CARD_VISIBILITY_STYLE" :class="cardClass">
          <CardHeader class="px-4 pb-0">
            <div class="flex items-center gap-3 min-w-0">
              <img
                v-if="comment.author_avatar && avatarSrc(avatarKey('pr-comment', comment.id), comment.author_avatar)"
                :src="avatarSrc(avatarKey('pr-comment', comment.id), comment.author_avatar)"
                :alt="comment.author_login"
                class="h-7 w-7 rounded-full"
                @error="handleAvatarError(avatarKey('pr-comment', comment.id), comment.author_avatar)"
              />
              <span v-else :class="avatarFallbackClass">
                {{ avatarInitial(comment.author_login) }}
              </span>
              <div class="min-w-0">
                <div class="text-sm font-medium truncate">{{ comment.author_login }}</div>
                <div class="text-xs text-muted-foreground">{{ formatTimestamp(comment.created_at) }}</div>
              </div>
            </div>
          </CardHeader>
          <CardContent class="px-4">
            <div :class="surfaceClass">
              <StaticMarkdownBody :content="comment.body" :base-url="comment.html_url" />
            </div>
          </CardContent>
        </Card>
      </section>

      <section class="space-y-3">
        <div class="flex items-center justify-between">
          <h2 class="text-sm font-semibold">Reviews</h2>
          <span class="text-xs text-muted-foreground">{{ visibleReviews.length }}/{{ overview.reviews.length }}</span>
        </div>

        <div v-if="visibleReviews.length === 0" class="rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
          No submitted reviews to show.
        </div>

        <Card v-for="review in visibleReviews" :key="review.review_id" :style="CARD_VISIBILITY_STYLE" :class="cardClass">
          <CardHeader class="px-4 pb-0">
            <div class="flex items-start gap-3 justify-between">
              <div class="flex items-center gap-3 min-w-0">
                <img
                  v-if="review.author_avatar && avatarSrc(avatarKey('review', review.review_id), review.author_avatar)"
                  :src="avatarSrc(avatarKey('review', review.review_id), review.author_avatar)"
                  :alt="review.author_login"
                  class="h-7 w-7 rounded-full"
                  @error="handleAvatarError(avatarKey('review', review.review_id), review.author_avatar)"
                />
                <span v-else :class="avatarFallbackClass">
                  {{ avatarInitial(review.author_login) }}
                </span>
                <div class="min-w-0">
                  <div class="flex items-center gap-2 min-w-0 flex-wrap">
                    <span class="text-sm font-medium truncate">{{ review.author_login }}</span>
                    <Badge variant="secondary" class="text-[10px] font-normal shrink-0">{{ reviewLabel(review) }}</Badge>
                    <Badge variant="outline" class="text-[10px] uppercase shrink-0">{{ reviewStateLabel(review) }}</Badge>
                  </div>
                  <div class="text-xs text-muted-foreground">{{ formatTimestamp(review.submitted_at) }}</div>
                </div>
              </div>
              <Badge v-if="review.all_threads_resolved" variant="secondary" class="shrink-0 text-[10px]">
                All conversations resolved
              </Badge>
            </div>
          </CardHeader>
          <CardContent class="px-4 space-y-3">
            <div v-if="review.body" :class="surfaceClass">
              <StaticMarkdownBody :content="review.body" :base-url="review.html_url" />
            </div>
            <div v-else class="text-sm text-muted-foreground">No review body.</div>

            <div v-if="review.has_threads" class="text-xs text-muted-foreground">
              {{ review.resolved_thread_count }}/{{ review.total_thread_count }} linked conversation{{ review.total_thread_count !== 1 ? 's' : '' }} resolved
            </div>
          </CardContent>
        </Card>
      </section>
    </div>
  </div>
</template>
