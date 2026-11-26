<script setup lang="ts">
import type * as model from '@/bindings/gitura/internal/model'
import { computed } from 'vue'
import { prLabel } from '@/lib/submodules'

/** What a submitted review did with comments on submodule files, under the "Review submitted" toast. */
const props = defineProps<{ result: model.ReviewSubmitResult }>()

/** The single comments by PR, linked through the first one on each. */
const commented = computed(() => {
  const byPR = new Map<string, { url: string; count: number }>()
  for (const url of props.result.submodule_comment_urls ?? []) {
    const pr = byPR.get(prLabel(url))
    if (pr) pr.count++
    else byPR.set(prLabel(url), { url, count: 1 })
  }
  return [...byPR]
})
</script>

<template>
  <div class="flex flex-col gap-0.5">
    <span v-if="result.submodule_review_urls?.length">
      Submodule {{ result.submodule_review_urls.length === 1 ? 'review' : 'reviews' }}:
      <template v-for="(url, i) in result.submodule_review_urls" :key="url">{{ i ? ', ' : '' }}<a :href="url" class="underline underline-offset-2">{{ prLabel(url) }}</a></template>
    </span>
    <span v-for="[pr, { url, count }] in commented" :key="pr">
      {{ count }} {{ count === 1 ? 'comment' : 'comments' }} posted on <a :href="url" class="underline underline-offset-2">{{ pr }}</a>
    </span>
    <span v-if="result.folded_comments">
      {{ result.folded_comments }} {{ result.folded_comments === 1 ? 'comment' : 'comments' }} on submodule files added to the review text
    </span>
  </div>
</template>
