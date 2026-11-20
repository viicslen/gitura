<script setup lang="ts">
import { ref, computed, inject, reactive } from 'vue'
import type { ComputedRef } from 'vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import SplitRunButton from '@/components/run/SplitRunButton.vue'
import { commands } from '@/composables/useCommands'
import { useToggles } from '@/composables/useToggles'
import type { DraftActions } from './DiffDraftComment.vue'

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
const draftActions = inject<DraftActions | null>('draftActions', null)
const cmds = computed(() => commands.value ?? [])
const { toggles } = useToggles()

function handleRan(runs: { input: string }[]): void {
  draftActions?.ran()
  // An edit landing while the run started is kept.
  if (!toggles.value.remove_draft_after_run || runs[0].input !== body.value.trim()) return
  body.value = ''
  emit('cancel')
}

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
      <div class="mr-auto flex items-center gap-2">
        <SplitRunButton
          v-if="cmds.length > 0"
          :commands="cmds"
          kind="draft"
          :input="body.trim()"
          :disabled="!body.trim() || busy"
          label="Run"
          @ran="handleRan"
        />
        <span v-if="busy" class="text-xs text-muted-foreground animate-in fade-in-0 delay-200 fill-mode-backwards" role="status">Sending…</span>
        <span v-else-if="draftsStale" class="text-xs text-muted-foreground">
          Your pending review was started on an earlier commit. Submit or discard it to add review comments here.
        </span>
      </div>
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
