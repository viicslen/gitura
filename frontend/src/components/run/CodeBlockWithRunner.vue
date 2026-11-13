<script setup lang="ts">
import type * as model from '@/bindings/gitura/internal/model'
import SplitRunButton from './SplitRunButton.vue'
import { useRuns } from '@/composables/useRuns'

const props = defineProps<{
  lang: string
  content: string
  isRun: boolean
  commands: model.CommandDTO[]
  defaultCommandName: string
  threadRootId?: number
  commentId?: number
}>()

const emit = defineEmits<{
  (e: 'ran'): void
}>()

const { } = useRuns()
</script>

<template>
  <div class="relative group/codeblock">
    <!-- Code block display -->
    <pre
      class="overflow-x-auto rounded-md bg-muted px-4 py-3 text-xs font-mono leading-relaxed"
    ><code>{{ content }}</code></pre>

    <!-- SplitRunButton: always visible on all code blocks -->
    <div
      v-if="commands.length > 0"
      class="absolute top-2 right-2 transition-opacity duration-fast opacity-0 group-hover/codeblock:opacity-100 group-hover/codeblock:duration-0 focus-within:opacity-100 has-[[data-state=open]]:opacity-100"
    >
      <SplitRunButton
        :commands="commands"
        :default-command-name="defaultCommandName"
        :input="content"
        :thread-root-id="threadRootId"
        :comment-id="commentId"
        size="sm"
        @ran="emit('ran')"
      />
    </div>
  </div>
</template>
