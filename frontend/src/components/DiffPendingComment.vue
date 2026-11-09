<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import { inject } from 'vue'
import MarkdownBody from './MarkdownBody.vue'
import EditableComment from './EditableComment.vue'
import type { model } from '@/wailsjs/go/models'

export type DraftActions = {
  indexOf: (draft: model.DraftCommentDTO) => number
  update: (draft: model.DraftCommentDTO, base: string, next: string) => Promise<void>
  remove: (draft: model.DraftCommentDTO) => Promise<void>
}

const props = defineProps<{ draft: model.DraftCommentDTO }>()
const actions = inject<DraftActions | null>('draftActions', null)
</script>

<template>
  <div class="mx-3 my-2 rounded-md border border-dashed border-amber-500/60 bg-amber-500/5 px-3 py-2 font-sans">
    <div class="mb-1 flex items-center gap-2 text-xs">
      <Badge variant="outline" class="border-amber-500/60 text-[0.625rem] text-amber-700 dark:text-amber-300">Pending</Badge>
      <span class="text-muted-foreground">
        {{ draft.side === 'LEFT' ? 'Old' : 'New' }} line{{ draft.start_line ? `s ${draft.start_line}–${draft.line}` : ` ${draft.line}` }} · submitted with your review
      </span>
    </div>
    <EditableComment
      v-if="actions"
      kind="draft"
      :id="actions.indexOf(draft)"
      :target="draft"
      :save="async (base, next) => { await actions!.update(props.draft, base, next); return next }"
      :remove="() => actions!.remove(props.draft)"
    >
      <MarkdownBody :content="draft.body" />
    </EditableComment>
    <MarkdownBody v-else :content="draft.body" />
  </div>
</template>
