<script lang="ts">
import type * as model from '@/bindings/gitura/internal/model'
import { isVerdict, type Verdict } from './VerdictPicker.vue'

export interface SubmoduleChoice {
  mode: 'review' | 'comments' | 'fold'
  verdict: Verdict
  body: string
}

/** Whether the user picks where a submodule's drafts go: it has an open PR with some of them in its diff. */
export const routable = (d: model.SubmoduleDraftsDTO): boolean => !!d.pr && d.to_pr > 0

const isMode = (m: unknown): m is SubmoduleChoice['mode'] => m === 'review' || m === 'comments' || m === 'fold'

/** The verdict and text an unsettled earlier submit sent, which the next one has to send too; null when there is none. */
export const inFlightReview = (r?: model.InFlightReviewDTO | null): Pick<SubmoduleChoice, 'verdict' | 'body'> | null =>
  r && isVerdict(r.verdict) ? { verdict: r.verdict, body: r.body } : null

// Own properties only: a submodule at a path such as `constructor` must not read Object.prototype.
export const choiceOf = (choices: Record<string, SubmoduleChoice>, path: string): SubmoduleChoice | undefined =>
  Object.hasOwn(choices, path) ? choices[path] : undefined
const newChoice = (): SubmoduleChoice => ({ mode: 'review', verdict: 'COMMENT', body: '' })

/** A choice per routable submodule, keeping those already made; one an unsettled earlier submit sent with is held to what it sent. */
export const keepChoices = (drafts: model.SubmoduleDraftsDTO[], prev: Record<string, SubmoduleChoice>): Record<string, SubmoduleChoice> =>
  Object.fromEntries(
    drafts.filter(routable).map((d) => {
      const choice = choiceOf(prev, d.path) ?? newChoice()
      return [d.path, isMode(d.in_flight) ? { ...choice, mode: d.in_flight, ...inFlightReview(d.in_flight_review) } : choice]
    }),
  )

/** Why SubmitReview would refuse the review, as the backend checks it; '' when it can go. */
export function submitBlocker(
  verdict: Verdict,
  body: string,
  comments: model.DraftCommentDTO[],
  drafts: model.SubmoduleDraftsDTO[],
  choices: Record<string, SubmoduleChoice>,
): string {
  if (verdict === 'APPROVE' || body.trim()) return ''
  const toPR = (d: model.SubmoduleDraftsDTO) => routable(d) && choiceOf(choices, d.path)?.mode !== 'fold'
  const parent = comments.filter((c) => !c.submodule).length + drafts.reduce((n, d) => n + d.folded + (toPR(d) ? 0 : d.to_pr), 0)
  if (parent) return ''
  if (verdict === 'REQUEST_CHANGES') return drafts.some(toPR) ? 'Requesting changes needs text here: every comment goes to a submodule PR.' : 'Requesting changes needs text or a comment.'
  return drafts.some(toPR) ? '' : 'Add text or a comment to submit.'
}

/** The choices as SubmitReview takes them, each with the routing shown, so a change since is refused rather than sent. */
export const submoduleRequest = (drafts: model.SubmoduleDraftsDTO[], choices: Record<string, SubmoduleChoice>): model.SubmoduleReviewChoice[] =>
  drafts.map((d) => ({
    path: d.path,
    pr: d.pr,
    to_pr: d.to_pr,
    folded: d.folded,
    ...(routable(d) ? choiceOf(choices, d.path) ?? newChoice() : { mode: 'fold' }),
  }))
</script>

<script setup lang="ts">
import { computed } from 'vue'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import Reveal from '@/components/motion/Reveal.vue'
import VerdictPicker from './VerdictPicker.vue'

/** Where the submit-review dialog sends the drafts on each submodule's files. */
const props = defineProps<{ drafts: model.SubmoduleDraftsDTO[] }>()
const emit = defineEmits<{ (e: 'retry'): void }>()
const choices = defineModel<Record<string, SubmoduleChoice>>('choices', { required: true })

const shown = computed(() => props.drafts.filter((d) => d.to_pr + d.folded > 0))
const comments = (n: number) => `${n} comment${n === 1 ? '' : 's'}`
const goes = (n: number) => (n === 1 ? 'goes' : 'go')
function foldReason(d: model.SubmoduleDraftsDTO, pr = `${d.pr}'s`): string {
  if (!d.outdated) return d.pr ? `outside ${pr} diff` : 'no open pull request'
  return d.pr && d.outdated < d.folded ? `written on an earlier version of the submodule or outside ${pr} diff` : 'written on an earlier version of the submodule'
}

function setMode(path: string, mode: string | number): void {
  if (isMode(mode)) choices.value[path].mode = mode
}
const held = (d: model.SubmoduleDraftsDTO, mode: SubmoduleChoice['mode']) => !!d.in_flight && d.in_flight !== mode
</script>

<template>
  <section v-if="shown.length" class="flex min-w-0 flex-col gap-2" aria-label="Comments on submodule files">
    <p class="text-xs font-medium text-muted-foreground">Comments on submodule files</p>
    <template v-for="d in shown" :key="d.path">
      <div v-if="routable(d) && choiceOf(choices, d.path)" class="flex min-w-0 flex-col gap-2 rounded-md border border-border px-3 py-2" :data-submodule="d.path">
        <div class="flex min-w-0 items-baseline gap-2 text-xs">
          <span class="truncate font-mono">{{ d.path }}</span>
          <a :href="d.pr_url" class="shrink-0 underline underline-offset-2">{{ d.pr }}</a>
          <span class="ml-auto shrink-0 text-muted-foreground">{{ comments(d.to_pr + d.folded) }}</span>
        </div>
        <Tabs :model-value="choices[d.path].mode" @update:model-value="setMode(d.path, $event)">
          <TabsList class="w-full" :aria-label="`Where the comments on ${d.path} go`">
            <TabsTrigger value="review" class="text-xs" :disabled="held(d, 'review')">Review</TabsTrigger>
            <TabsTrigger value="comments" class="text-xs" :disabled="held(d, 'comments')">Single comments</TabsTrigger>
            <TabsTrigger value="fold" class="text-xs" :disabled="held(d, 'fold')">Add to this review's text</TabsTrigger>
          </TabsList>
        </Tabs>
        <p v-if="d.in_flight" class="text-xs text-muted-foreground">An earlier submit may have reached {{ d.pr }}; submit again to settle it.</p>
        <Reveal axis="y" :open="choices[d.path].mode === 'review'" :class="{ '-mt-2': choices[d.path].mode !== 'review' }">
          <div class="flex flex-col gap-2">
            <VerdictPicker v-model="choices[d.path].verdict" compact :own-pr="d.own_pr" :disabled="!!d.in_flight_review" :name="`verdict-${d.path}`" :label="`Verdict on ${d.pr}`" />
            <Textarea
              v-model="choices[d.path].body"
              :disabled="!!d.in_flight_review"
              rows="2"
              class="text-sm resize-none"
              :placeholder="`Text for the review on ${d.pr} (optional)…`"
              :aria-label="`Review text for ${d.pr}`"
            />
          </div>
        </Reveal>
        <p v-if="choices[d.path].mode === 'fold'" class="text-xs text-muted-foreground">
          {{ d.to_pr + d.folded === 1 ? 'Its comment goes' : `All ${comments(d.to_pr + d.folded)} go` }} into this review's text.
        </p>
        <p v-else-if="d.folded" class="text-xs text-muted-foreground">
          {{ comments(d.folded) }} {{ d.outdated ? `${goes(d.folded)} into this review's text: ${foldReason(d, 'its PR\'s')}` : `outside its PR's diff ${goes(d.folded)} into this review's text` }}.
        </p>
      </div>
      <p v-else-if="d.pr_unknown" class="text-xs text-muted-foreground" :data-submodule="d.path">
        Couldn't check <span class="font-mono">{{ d.path }}</span> for an open pull request — its comments will be added to this review's text.
        <Button variant="link" size="sm" class="h-auto p-0 text-xs" @click="emit('retry')">Retry</Button>
      </p>
      <p v-else class="text-xs text-muted-foreground" :data-submodule="d.path">
        <span class="font-mono">{{ d.path }}</span>: {{ comments(d.folded) }} added to this review's text
        ({{ foldReason(d) }}).
      </p>
    </template>
  </section>
</template>
