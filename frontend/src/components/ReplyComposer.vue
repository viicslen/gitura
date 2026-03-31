<script setup lang="ts">
import { ref } from 'vue'
import type { model } from '../wailsjs/go/models'
import { ReplyToComment } from '../wailsjs/go/main/App'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

const props = defineProps<{
  threadRootId: number
}>()

const emit = defineEmits<{
  (e: 'reply-sent', comment: model.CommentDTO): void
}>()

const draft = ref('')
const submitting = ref(false)
const inlineError = ref('')

async function submit(): Promise<void> {
  if (!draft.value.trim() || submitting.value) return
  submitting.value = true
  inlineError.value = ''

  try {
    const comment = await ReplyToComment(props.threadRootId, draft.value)
    draft.value = ''
    emit('reply-sent', comment)
  } catch (err) {
    inlineError.value = String(err)
  } finally {
    submitting.value = false
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
      class="resize-none"
      :disabled="submitting"
      @keydown="handleKeydown"
    />

    <div class="flex items-center gap-2">
      <slot name="leading" />

      <div class="flex-1" />

      <p v-if="inlineError" class="text-xs text-destructive truncate">
        {{ inlineError }}
      </p>
      <span v-else class="text-xs text-muted-foreground">
        Ctrl+Enter to submit
      </span>

      <Button
        size="sm"
        :disabled="!draft.trim() || submitting"
        @click="submit"
      >
        {{ submitting ? 'Sending…' : 'Reply' }}
      </Button>
    </div>
  </div>
</template>
