<script setup lang="ts">
import { ref, inject } from 'vue'
import type { ComputedRef } from 'vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

const props = defineProps<{
  target: { path: string; line: number; side: string; startLine?: number }
}>()

const emit = defineEmits<{
  (e: 'submit', body: string, mode: 'draft' | 'immediate'): void
  (e: 'cancel'): void
}>()

const body = ref('')
const submitting = ref(false)
const hasPendingReview = inject<ComputedRef<boolean>>('hasPendingReview')

async function submit(mode: 'draft' | 'immediate'): Promise<void> {
  if (!body.value.trim() || submitting.value) return
  submitting.value = true
  try {
    emit('submit', body.value.trim(), mode)
    body.value = ''
  } finally {
    submitting.value = false
  }
}

function handleKeydown(event: KeyboardEvent): void {
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    submit('draft')
  }
  if (event.key === 'Escape') {
    emit('cancel')
  }
}
</script>

<template>
  <div
    class="rounded-md border border-border bg-background shadow-sm p-3 flex flex-col gap-2"
    aria-label="Add inline comment"
  >
    <Textarea
      v-model="body"
      placeholder="Leave a comment…"
      rows="3"
      class="text-sm resize-none"
      aria-label="Comment body"
      autofocus
      @keydown="handleKeydown"
    />
    <div class="flex items-center justify-end gap-2">
      <Button
        variant="ghost"
        size="sm"
        aria-label="Cancel comment"
        @click="emit('cancel')"
      >
        Cancel
      </Button>
      <Button
        variant="outline"
        size="sm"
        :disabled="!body.trim() || submitting"
        aria-label="Add as single comment"
        @click="submit('immediate')"
      >
        Add single comment
      </Button>
      <Button
        size="sm"
        :disabled="!body.trim() || submitting"
        aria-label="Add to review"
        @click="submit('draft')"
      >
        {{ hasPendingReview ? 'Add to review' : 'Start a review' }}
      </Button>
    </div>
  </div>
</template>
