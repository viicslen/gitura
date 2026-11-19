<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import { computed, inject } from 'vue'
import { toast } from 'vue-sonner'
import { createReusableTemplate } from '@vueuse/core'
import MarkdownBody from '@/components/markdown/MarkdownBody.vue'
import EditableComment from '@/components/comment/EditableComment.vue'
import SplitRunButton from '@/components/run/SplitRunButton.vue'
import { commands, defaultCommandName } from '@/composables/useCommands'
import { useToggles } from '@/composables/useToggles'
import { errorMessage } from '@/lib/utils'
import type * as model from '@/bindings/gitura/internal/model'

export type DraftActions = {
  indexOf: (draft: model.DraftCommentDTO) => number
  update: (draft: model.DraftCommentDTO, base: string, next: string) => Promise<void>
  remove: (draft: model.DraftCommentDTO) => Promise<void>
  /** A command was started on a draft. */
  ran: () => void
}

const props = defineProps<{ draft: model.DraftCommentDTO }>()
const actions = inject<DraftActions | null>('draftActions', null)
const [DefineHeader, ReuseHeader] = createReusableTemplate()
const cmds = computed(() => commands.value ?? [])
const { toggles } = useToggles()

async function handleRan(): Promise<void> {
  actions!.ran()
  if (!toggles.value.remove_draft_after_run) return
  try {
    await actions!.remove(props.draft)
  } catch (err) {
    toast.error('Could not remove the pending comment: ' + errorMessage(err))
  }
}
</script>

<template>
  <div class="relative mx-3 my-2 rounded-md border border-dashed border-amber-500/60 bg-amber-500/5 px-3 py-2 font-sans">
    <DefineHeader>
      <div class="flex flex-1 items-center gap-2 text-xs">
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
      <template #header>
        <ReuseHeader />
        <!-- Stays visible while its own menu is open, when the pointer has left the comment. -->
        <div
          v-if="cmds.length > 0"
          class="opacity-0 transition-opacity duration-faster group-hover/edit:opacity-100 group-hover/edit:duration-0 focus-within:opacity-100 focus-within:duration-0 has-[[data-state=open]]:opacity-100"
        >
          <SplitRunButton :commands="cmds" :default-command-name="defaultCommandName" :input="draft.body" @ran="handleRan" />
        </div>
      </template>
      <MarkdownBody :content="draft.body" />
    </EditableComment>
    <div v-else class="space-y-1">
      <ReuseHeader />
      <MarkdownBody :content="draft.body" />
    </div>
  </div>
</template>
