<script setup lang="ts">
import { ref, inject } from 'vue'
import type { ComputedRef } from 'vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

const props = defineProps<{
  target: { path: string; line: number; side: string; startLine?: number }
  /** Resolves true once the comment is saved; on false the typed text is kept. */
  post: (body: string, mode: 'draft' | 'immediate') => Promise<boolean>
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
}>()

const body = ref('')
const submitting = ref(false)
const hasPendingReview = inject<ComputedRef<boolean>>('hasPendingReview')
const draftsStale = inject<ComputedRef<boolean> | null>('draftsStale', null)

async function submit(mode: 'draft' | 'immediate'): Promise<void> {
  if (!body.value.trim() || submitting.value) return
  submitting.value = true
  try {
    if (await props.post(body.value.trim(), mode)) body.value = ''
  } finally {
    submitting.value = false
  }
}

function handleKeydown(event: KeyboardEvent): void {
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter' && !draftsStale?.value) {
    submit('draft')
  }
  // Escape only dismisses an empty form, so a stray keypress can't discard a written comment.
  if (event.key === 'Escape' && !body.value.trim() && !submitting.value) {
    emit('cancel')
  }
}
</script>

<template>
  <div
    class="rounded-md border border-border bg-background shadow-sm p-3 flex flex-col gap-2"
    role="group"
    aria-label="Add inline comment"
  >
    <Textarea
      v-model="body"
      placeholder="Leave a comment…"
      rows="3"
      class="text-sm resize-none"
      aria-label="Comment body"
      :disabled="submitting"
      autofocus
      @keydown="handleKeydown"
    />
    <div class="flex items-center justify-end gap-2">
      <span v-if="draftsStale" class="mr-auto text-xs text-muted-foreground">
        Your pending review was started on an earlier commit. Submit or discard it to add review comments here.
      </span>
      <Button
        variant="ghost"
        size="sm"
        aria-label="Cancel comment"
        :disabled="submitting"
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
        :disabled="!body.trim() || submitting || draftsStale"
        aria-label="Add to review"
        @click="submit('draft')"
      >
        {{ hasPendingReview ? 'Add to review' : 'Start a review' }}
      </Button>
    </div>
  </div>
</template>
