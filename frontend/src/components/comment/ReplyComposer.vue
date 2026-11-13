<script lang="ts">
import { errorMessage } from '@/lib/utils'
import { reactive } from 'vue'

// Per thread, shared by every composer showing it (Conversation and Files tabs), so hiding,
// collapsing or switching tabs keeps unsent text and a thread can't be sent twice at once.
const replyDrafts = reactive<Record<number, string>>({})
const sendingThreads = reactive<Record<number, boolean>>({})
</script>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type * as model from '@/bindings/gitura/internal/model'
import { ReplyToComment } from '@/bindings/gitura/internal/app/app'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import BusyLabel from '@/components/motion/BusyLabel.vue'
import { crossfade } from '@/components/motion/motion'

const props = defineProps<{
  threadRootId: number
}>()

const emit = defineEmits<{
  (e: 'reply-sent', comment: model.ReviewCommentDTO): void
}>()

const draft = computed({
  get: () => replyDrafts[props.threadRootId] ?? '',
  set: (value: string) => {
    if (value) replyDrafts[props.threadRootId] = value
    else delete replyDrafts[props.threadRootId]
  },
})
const submitting = computed(() => !!sendingThreads[props.threadRootId])
const inlineError = ref('')

async function submit(): Promise<void> {
  if (!draft.value.trim() || submitting.value) return
  const rootId = props.threadRootId
  const sent = draft.value
  sendingThreads[rootId] = true

  try {
    const comment = await ReplyToComment(rootId, sent)
    if (replyDrafts[rootId] === sent) delete replyDrafts[rootId]
    inlineError.value = ''
    emit('reply-sent', comment)
  } catch (err) {
    inlineError.value = errorMessage(err)
  } finally {
    delete sendingThreads[rootId]
  }
}

function handleKeydown(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    submit()
  }
}
</script>

<template>
  <div class="space-y-2">
    <Textarea
      v-model="draft"
      placeholder="Write a reply…"
      rows="3"
      class="resize-none "
      :disabled="submitting"
      @keydown="handleKeydown"
    />

    <div class="flex flex-wrap items-center gap-2">
      <slot name="leading" />

      <div class="flex-1" />

      <span class="grid min-w-0 justify-items-end">
        <Transition v-bind="crossfade">
          <p v-if="inlineError" :key="inlineError" class="[grid-area:1/1] max-w-full text-xs text-destructive truncate">
            {{ inlineError }}
          </p>
          <span v-else class="[grid-area:1/1] text-xs text-muted-foreground">
            Ctrl+Enter to submit
          </span>
        </Transition>
      </span>

      <Button
        size="sm"
        class="ml-auto"
        :disabled="!draft.trim() || submitting"
        @click="submit"
      >
        <BusyLabel :busy="submitting">Reply</BusyLabel>
      </Button>
    </div>
  </div>
</template>
