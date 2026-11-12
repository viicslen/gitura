<script setup lang="ts">
import { ref, computed, inject, reactive } from 'vue'
import type { ComputedRef } from 'vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

const props = withDefaults(defineProps<{
  target: { path: string; line: number; side: string; startLine?: number }
  /** Resolves true once the comment is saved; on false the typed text is kept. */
  post: (body: string, mode: 'draft' | 'immediate') => Promise<boolean>
  /** Sent from an earlier view of the file and still in flight. */
  sending?: boolean
  /** Owned by the diff so text survives the form moving lines; an object, not a v-model, so typing doesn't re-render the diff. */
  draft?: { body: string }
}>(), { draft: () => reactive({ body: '' }) })

const emit = defineEmits<{
  (e: 'cancel'): void
}>()

const body = computed({
  get: () => props.draft.body,
  set: (value: string) => { props.draft.body = value },
})
const submitting = ref(false)
const busy = computed(() => submitting.value || props.sending)
const hasPendingReview = inject<ComputedRef<boolean>>('hasPendingReview')
const draftsStale = inject<ComputedRef<boolean> | null>('draftsStale', null)
const localReview = inject('localReview', false)

async function submit(mode: 'draft' | 'immediate'): Promise<void> {
  if (!body.value.trim() || busy.value) return
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
  if (event.key === 'Escape' && !body.value.trim() && !busy.value) {
    emit('cancel')
  }
}
</script>

<template>
  <div
    class="rounded-md border border-border bg-background shadow-sm p-3 flex flex-col gap-2"
    role="group"
    aria-label="Add inline comment"
    :aria-busy="busy"
  >
    <Textarea
      v-model="body"
      placeholder="Leave a comment…"
      rows="3"
      class="text-sm resize-none"
      aria-label="Comment body"
      :disabled="busy"
      autofocus
      @keydown="handleKeydown"
    />
    <div class="flex flex-wrap items-center justify-end gap-2">
      <span v-if="busy" class="mr-auto text-xs text-muted-foreground" role="status">Sending…</span>
      <span v-else-if="draftsStale" class="mr-auto text-xs text-muted-foreground">
        Your pending review was started on an earlier commit. Submit or discard it to add review comments here.
      </span>
      <Button
        variant="ghost"
        size="sm"
        aria-label="Cancel comment"
        :disabled="busy"
        @click="emit('cancel')"
      >
        Cancel
      </Button>
      <Button
        v-if="!localReview"
        variant="outline"
        size="sm"
        :disabled="!body.trim() || busy"
        aria-label="Add as single comment"
        @click="submit('immediate')"
      >
        Add single comment
      </Button>
      <Button
        size="sm"
        :disabled="!body.trim() || busy || draftsStale"
        aria-label="Add to review"
        @click="submit('draft')"
      >
        {{ hasPendingReview ? 'Add to review' : 'Start a review' }}
      </Button>
    </div>
  </div>
</template>
