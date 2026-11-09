<script setup lang="ts">
import { computed } from 'vue'
import type { model } from '@/wailsjs/go/models'
import StaticMarkdownBody from '@/components/StaticMarkdownBody.vue'
import EditableComment from '@/components/EditableComment.vue'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { useAvatarFallback } from '@/composables/useAvatarFallback'
import { taskToggler } from '@/lib/taskList'

const props = defineProps<{
  overview: model.PROverviewDTO | null
}>()
const showResolvedReviews = defineModel<boolean>('showResolvedReviews', { default: false })

const { avatarSrc, handleAvatarError, avatarInitial } = useAvatarFallback()

const CARD_VISIBILITY_STYLE = {
  contentVisibility: 'auto',
  containIntrinsicSize: '320px',
}

const visibleGeneralComments = computed(() => props.overview?.general_comments ?? [])

const visibleReviews = computed(() => props.overview?.reviews ?? [])
// Reviews still waiting on unresolved conversations go above general comments.
const hasOpenReview = computed(() => visibleReviews.value.some((r) => r.has_threads && !r.all_threads_resolved))

const sectionOrder = computed(() => (hasOpenReview.value ? ['reviews', 'general'] : ['general', 'reviews']))
const reviewTotal = computed(() => Object.keys(props.overview?.review_numbers ?? {}).length)

const cardClass = computed(() => [
  'gap-4 py-4',
  'shadow-none',
])

const surfaceClass = 'rounded-lg bg-muted/30 px-4 py-3'

const avatarFallbackClass = 'inline-flex h-7 w-7 items-center justify-center rounded-full bg-muted text-xs font-semibold text-muted-foreground'

function labelStyle(color: string): Record<string, string> {
  const hex = color.replace(/^#/, '')
  // Pick whichever text colour has the higher WCAG contrast against the label colour.
  const channel = (i: number) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  }
  const luminance = 0.2126 * channel(0) + 0.7152 * channel(2) + 0.0722 * channel(4)
  const DARK_TEXT_LUMINANCE = 0.016 // #1f2328
  const whiteContrast = 1.05 / (luminance + 0.05)
  const darkContrast = (luminance + 0.05) / (DARK_TEXT_LUMINANCE + 0.05)
  return { backgroundColor: `#${hex}`, color: whiteContrast >= darkContrast ? '#fff' : '#1f2328' }
}

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
  const number = props.overview?.review_numbers?.[review.review_id]
  return number ? `Review ${number}` : 'Review'
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
    <div class="mx-auto flex w-full max-w-4xl flex-col gap-4 p-4">
      <Card class="shadow-none">
        <CardContent>
          <ul v-if="overview.summary.labels?.length" class="mb-3 flex flex-wrap gap-1.5" aria-label="Labels">
            <li
              v-for="label in overview.summary.labels"
              :key="label.name"
              :title="label.description || undefined"
              class="rounded-full px-2 py-0.5 text-xs font-medium"
              :style="labelStyle(label.color)"
            >{{ label.name }}</li>
          </ul>
          <div v-if="overview.summary.body">
            <StaticMarkdownBody
              :content="overview.summary.body"
              :base-url="overview.summary.html_url"
              :repo-owner="overview.summary.head_owner || overview.summary.owner"
              :repo-name="overview.summary.head_repo || overview.summary.repo"
              :repo-ref="overview.summary.head_sha"
              :toggle-task="taskToggler('pr_body', 0, overview.summary)"
            />
          </div>
          <p v-else class="text-sm text-muted-foreground">No pull request description.</p>
        </CardContent>
      </Card>

      <template v-for="section in sectionOrder" :key="section">
        <section v-if="section === 'general'" class="space-y-3">
          <div class="flex items-center justify-between">
            <h2 class="text-sm font-semibold">General comments</h2>
            <span class="text-xs text-muted-foreground tabular-nums">{{ visibleGeneralComments.length }}</span>
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
                <EditableComment kind="issue_comment" :id="comment.id" :target="comment">
                  <StaticMarkdownBody :content="comment.body" :base-url="comment.html_url" :toggle-task="taskToggler('issue_comment', comment.id, comment)" />
                </EditableComment>
              </div>
            </CardContent>
          </Card>
        </section>

        <section v-else class="space-y-3">
          <div class="flex items-center justify-between">
            <h2 class="text-sm font-semibold">
              Reviews
              <span v-if="hasOpenReview" class="ml-1 font-normal text-xs text-muted-foreground">· open conversations</span>
            </h2>
            <div class="flex items-center gap-3">
              <label class="flex items-center gap-2 text-xs text-muted-foreground cursor-pointer select-none">
                <Switch v-model="showResolvedReviews" />
                Include fully resolved
              </label>
              <span class="text-xs text-muted-foreground tabular-nums">
                {{ reviewTotal > overview.reviews.length ? `${overview.reviews.length} of ${reviewTotal}` : overview.reviews.length }}
              </span>
            </div>
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
                <EditableComment kind="review" :id="review.review_id" :target="review">
                  <StaticMarkdownBody :content="review.body" :base-url="review.html_url" :toggle-task="taskToggler('review', review.review_id, review)" />
                </EditableComment>
              </div>
              <div v-else class="text-sm text-muted-foreground">No review body.</div>

              <div v-if="review.has_threads" class="text-xs text-muted-foreground">
                {{ review.resolved_thread_count }}/{{ review.total_thread_count }} linked conversation{{ review.total_thread_count !== 1 ? 's' : '' }} resolved
              </div>
            </CardContent>
          </Card>
        </section>
      </template>
    </div>
  </div>
</template>
