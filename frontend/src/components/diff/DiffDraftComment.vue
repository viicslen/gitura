<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import { inject } from 'vue'
import { createReusableTemplate } from '@vueuse/core'
import MarkdownBody from '@/components/markdown/MarkdownBody.vue'
import EditableComment from '@/components/comment/EditableComment.vue'
import type * as model from '@/bindings/gitura/internal/model'

export type DraftActions = {
  indexOf: (draft: model.DraftCommentDTO) => number
  update: (draft: model.DraftCommentDTO, base: string, next: string) => Promise<void>
  remove: (draft: model.DraftCommentDTO) => Promise<void>
}

const props = defineProps<{ draft: model.DraftCommentDTO }>()
const actions = inject<DraftActions | null>('draftActions', null)
const [DefineHeader, ReuseHeader] = createReusableTemplate()
</script>

<template>
  <div class="relative mx-3 my-2 rounded-md border border-dashed border-amber-500/60 bg-amber-500/5 px-3 py-2 font-sans">
    <DefineHeader>
      <div class="flex items-center gap-2 text-xs">
        <Badge variant="outline" class="border-amber-500/60 text-[0.625rem] text-amber-700 dark:text-amber-300">Pending</Badge>
        <span class="text-muted-foreground">
          <template v-if="draft.line">{{ draft.side === 'LEFT' ? 'Old' : 'New' }} line{{ draft.start_line ? `s ${draft.start_line}–${draft.line}` : ` ${draft.line}` }}</template>
          <template v-else>Whole file</template> · submitted with your review
        </span>
      </div>
    </DefineHeader>
    <EditableComment
      v-if="actions"
      kind="draft"
      :id="actions.indexOf(draft)"
      :target="draft"
      :save="async (base, next) => { await actions!.update(props.draft, base, next); return next }"
      :remove="() => actions!.remove(props.draft)"
      class="space-y-1"
    >
      <template #header><ReuseHeader /></template>
      <MarkdownBody :content="draft.body" />
    </EditableComment>
    <div v-else class="space-y-1">
      <ReuseHeader />
      <MarkdownBody :content="draft.body" />
    </div>
  </div>
</template>
