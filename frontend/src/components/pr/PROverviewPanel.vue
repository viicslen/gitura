<script setup lang="ts">
import { computed } from 'vue'
import type * as model from '@/bindings/gitura/internal/model'
import PlainMarkdownBody from '@/components/markdown/PlainMarkdownBody.vue'
import EditableComment from '@/components/comment/EditableComment.vue'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { useAvatarFallback } from '@/composables/useAvatarFallback'
import { taskToggler } from '@/lib/taskList'
import { collapse, collapsible, crossfade, scaleIn } from '@/components/motion/motion'

const props = defineProps<{
  overview: model.PROverviewDTO | null
}>()
const showResolvedReviews = defineModel<boolean>('showResolvedReviews', { default: false })

const { avatarSrc, handleAvatarError, avatarInitial } = useAvatarFallback()


const visibleGeneralComments = computed(() => props.overview?.general_comments ?? [])

const visibleReviews = computed(() => props.overview?.reviews ?? [])
// Reviews still waiting on unresolved conversations go above general comments.
const hasOpenReview = computed(() => visibleReviews.value.some((r) => r.has_threads && !r.all_threads_resolved))

const sectionOrder = computed(() => (hasOpenReview.value ? ['reviews', 'general'] : ['general', 'reviews']))
const reviewTotal = computed(() => Object.keys(props.overview?.review_numbers ?? {}).length)
const reviewCount = computed(() => {
  const shown = props.overview?.reviews.length ?? 0
  return reviewTotal.value > shown ? `${shown} of ${reviewTotal.value}` : `${shown}`
})

// Off-screen cards skip rendering and stand in at their last rendered height, or 320px before they were ever rendered.
const cardClass = 'gap-4 py-4 shadow-none [content-visibility:auto] [contain-intrinsic-size:auto_320px]'
// A card that is opening is clipped, which counts as off-screen: it would open to 320px and then snap to its real height.
const cardCollapse = { ...collapse, enterActiveClass: `${collapse.enterActiveClass} **:[content-visibility:visible]!` }

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

function reviewLabel(review: model.ReviewDTO): string {
  const number = props.overview?.review_numbers?.[`${review.review_id}`]
  return number ? `Review ${number}` : 'Review'
}

function reviewStateLabel(review: model.ReviewDTO): string {
  return review.state.replaceAll('_', ' ').toLowerCase()
}

function avatarKey(prefix: string, id: number): string {
  return `${prefix}:${id}`
}

</script>

<template>
  <div class="grid grid-cols-1 grid-rows-1">
    <Transition
      enter-active-class="transition-[opacity,translate] duration-slow"
      enter-from-class="opacity-0 translate-y-1.5"
      leave-active-class="transition-opacity duration-base"
      leave-to-class="opacity-0"
    >
      <div v-if="!overview" class="[grid-area:1/1] grid place-items-center">
        <!-- Delayed so the message does not flash while the overview is still loading. -->
        <span class="text-sm text-muted-foreground animate-in fade-in-0 animation-duration-base delay-200 fill-mode-backwards">
          No overview data available.
        </span>
      </div>

      <div v-else class="[grid-area:1/1] min-h-0 overflow-y-auto">
        <!-- The two sections trade places when the last open conversation is resolved; they glide instead of jumping. -->
        <TransitionGroup tag="div" class="mx-auto flex w-full max-w-4xl flex-col gap-4 p-4" move-class="transition-transform duration-slow ease-in-out">
          <Card key="description" class="shadow-none">
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
                <PlainMarkdownBody
                  :content="overview.summary.body"
                  :baseURL="overview.summary.html_url"
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
            <section v-if="section === 'general'">
              <div class="flex items-center justify-between">
                <h2 class="text-sm font-semibold">General comments</h2>
                <span class="text-xs text-muted-foreground tabular-nums">{{ visibleGeneralComments.length }}</span>
              </div>

              <!-- Margins on the items, not space-y: a card opening or closing takes its gap with it. -->
              <TransitionGroup tag="div" v-bind="cardCollapse">
                <div v-if="visibleGeneralComments.length === 0" key="empty" class="mt-3" :class="collapsible">
                  <div class="min-h-0">
                    <div class="rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                      No general pull request comments.
                    </div>
                  </div>
                </div>

                <div v-for="comment in visibleGeneralComments" :key="comment.id" class="mt-3" :class="collapsible">
                  <div class="min-h-0">
                    <Card :class="cardClass">
                      <CardHeader class="px-4 pb-0">
                        <div class="flex items-center gap-3 min-w-0">
                          <img
                            v-if="comment.author_avatar && avatarSrc(avatarKey('pr-comment', comment.id), comment.author_avatar)"
                            :src="avatarSrc(avatarKey('pr-comment', comment.id), comment.author_avatar)"
                            :alt="comment.author_login"
                            class="h-7 w-7 rounded-full bg-muted"
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
                            <PlainMarkdownBody :content="comment.body" :baseURL="comment.html_url" :toggle-task="taskToggler('issue_comment', comment.id, comment)" />
                          </EditableComment>
                        </div>
                      </CardContent>
                    </Card>
                  </div>
                </div>
              </TransitionGroup>
            </section>

            <section v-else>
              <div class="flex items-center justify-between">
                <h2 class="text-sm font-semibold">
                  Reviews
                  <Transition v-bind="crossfade">
                    <span v-if="hasOpenReview" class="ml-1 font-normal text-xs text-muted-foreground">· open conversations</span>
                  </Transition>
                </h2>
                <div class="flex items-center gap-3">
                  <label class="flex items-center gap-2 text-xs text-muted-foreground cursor-pointer select-none">
                    <Switch v-model="showResolvedReviews" />
                    Include fully resolved
                  </label>
                  <!-- The pseudo-element reserves the widest count, so the switch beside it never moves under the pointer. -->
                  <span
                    class="grid justify-items-end text-xs text-muted-foreground tabular-nums after:invisible after:h-0 after:content-[attr(data-widest)] after:[grid-area:1/1]"
                    :data-widest="`${reviewTotal} of ${reviewTotal}`"
                  >
                    <Transition v-bind="crossfade">
                      <span :key="reviewCount" class="[grid-area:1/1]">{{ reviewCount }}</span>
                    </Transition>
                  </span>
                </div>
              </div>

              <TransitionGroup tag="div" v-bind="cardCollapse">
                <div v-if="visibleReviews.length === 0" key="empty" class="mt-3" :class="collapsible">
                  <div class="min-h-0">
                    <div class="rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                      No submitted reviews to show.
                    </div>
                  </div>
                </div>

                <div v-for="review in visibleReviews" :key="review.review_id" class="mt-3" :class="collapsible">
                  <div class="min-h-0">
                    <Card :class="cardClass">
                      <CardHeader class="px-4 pb-0">
                        <div class="flex items-start gap-3 justify-between">
                          <div class="flex items-center gap-3 min-w-0">
                            <img
                              v-if="review.author_avatar && avatarSrc(avatarKey('review', review.review_id), review.author_avatar)"
                              :src="avatarSrc(avatarKey('review', review.review_id), review.author_avatar)"
                              :alt="review.author_login"
                              class="h-7 w-7 rounded-full bg-muted"
                              @error="handleAvatarError(avatarKey('review', review.review_id), review.author_avatar)"
                            />
                            <span v-else :class="avatarFallbackClass">
                              {{ avatarInitial(review.author_login) }}
                            </span>
                            <div class="min-w-0">
                              <div class="flex items-center gap-2 min-w-0 flex-wrap">
                                <span class="text-sm font-medium truncate">{{ review.author_login }}</span>
                                <Badge variant="secondary" class="text-[0.625rem] font-normal shrink-0">{{ reviewLabel(review) }}</Badge>
                                <Badge variant="outline" class="text-[0.625rem] uppercase shrink-0">{{ reviewStateLabel(review) }}</Badge>
                              </div>
                              <div class="text-xs text-muted-foreground">{{ formatTimestamp(review.submitted_at) }}</div>
                            </div>
                          </div>
                          <Transition v-bind="scaleIn">
                            <span v-if="review.all_threads_resolved" class="inline-flex shrink-0">
                              <Badge variant="secondary" class="text-[0.625rem]">All conversations resolved</Badge>
                            </span>
                          </Transition>
                        </div>
                      </CardHeader>
                      <CardContent class="px-4 space-y-3">
                        <div v-if="review.body" :class="surfaceClass">
                          <EditableComment kind="review" :id="review.review_id" :target="review">
                            <PlainMarkdownBody :content="review.body" :baseURL="review.html_url" :toggle-task="taskToggler('review', review.review_id, review)" />
                          </EditableComment>
                        </div>
                        <div v-else class="text-sm text-muted-foreground">No review body.</div>

                        <div v-if="review.has_threads" class="text-xs text-muted-foreground">
                          {{ review.resolved_thread_count }}/{{ review.total_thread_count }} linked conversation{{ review.total_thread_count !== 1 ? 's' : '' }} resolved
                        </div>
                      </CardContent>
                    </Card>
                  </div>
                </div>
              </TransitionGroup>
            </section>
          </template>
        </TransitionGroup>
      </div>
    </Transition>
  </div>
</template>
