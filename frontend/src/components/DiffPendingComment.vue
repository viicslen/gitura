<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import MarkdownBody from './MarkdownBody.vue'
import type { model } from '@/wailsjs/go/models'

defineProps<{ draft: model.DraftCommentDTO }>()
</script>

<template>
  <div class="mx-3 my-2 rounded-md border border-dashed border-amber-500/60 bg-amber-500/5 px-3 py-2 font-sans">
    <div class="mb-1 flex items-center gap-2 text-xs">
      <Badge variant="outline" class="border-amber-500/60 text-[10px] text-amber-700 dark:text-amber-300">Pending</Badge>
      <span class="text-muted-foreground">
        {{ draft.side === 'LEFT' ? 'Old' : 'New' }} line{{ draft.start_line ? `s ${draft.start_line}–${draft.line}` : ` ${draft.line}` }} · submitted with your review
      </span>
    </div>
    <MarkdownBody :content="draft.body" />
  </div>
</template>
