<script setup lang="ts">
import { ref } from 'vue'
import type { model } from '../wailsjs/go/models'
import { CommitSuggestion } from '../wailsjs/go/main/App'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { CheckCircle2, GitCommit, Loader2 } from 'lucide-vue-next'

const props = defineProps<{
  comment: model.CommentDTO
}>()

const emit = defineEmits<{
  (e: 'committed', result: model.SuggestionCommitResult): void
}>()

// Parse the suggestion block lines for display.
function parseSuggestionLines(body: string): string[] {
  const lines = body.split('\n')
  const result: string[] = []
  let inside = false
  for (const line of lines) {
    const stripped = line.replace(/\r$/, '')
    if (!inside && stripped.startsWith('```suggestion')) {
      inside = true
      continue
    }
    if (inside && stripped === '```') break
    if (inside) result.push(stripped)
  }
  return result
}

const suggestionLines = parseSuggestionLines(props.comment.body)

// UI state
const showInput = ref(false)
const commitMessage = ref('Apply suggestion from review')
const submitting = ref(false)
const error = ref('')
const committed = ref<model.SuggestionCommitResult | null>(null)

function openInput() {
  showInput.value = true
  error.value = ''
}

async function handleCommit() {
  if (!commitMessage.value.trim()) {
    error.value = 'Commit message is required.'
    return
  }
  submitting.value = true
  error.value = ''
  try {
    const result = await CommitSuggestion(props.comment.id, commitMessage.value.trim())
    committed.value = result
    showInput.value = false
    emit('committed', result)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    if (msg.includes('github:conflict')) {
      error.value = 'Conflict: the file has changed since this suggestion was made. Please re-review the latest version.'
    } else if (msg.startsWith('validation:')) {
      error.value = msg.replace('validation:', '').trim()
    } else {
      error.value = msg.replace('github:', '').trim() || 'Failed to commit suggestion.'
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div
    v-if="comment.is_suggestion"
    class="rounded-md border border-border bg-muted/30 overflow-hidden"
    aria-label="Suggestion block"
  >
    <!-- Suggestion diff preview -->
    <div class="px-3 pt-2 pb-1">
      <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Suggested change</span>
    </div>
    <pre
      class="font-mono text-xs overflow-x-auto px-3 pb-3 whitespace-pre"
      aria-label="Suggested code"
    ><template v-for="(line, i) in suggestionLines" :key="i"><span class="block bg-green-500/15 text-green-800 dark:text-green-300">+{{ line }}</span></template></pre>

    <!-- Success state -->
    <div
      v-if="committed"
      class="flex items-center gap-2 px-3 py-2 border-t border-border bg-muted/20 text-sm text-green-700 dark:text-green-400"
    >
      <CheckCircle2 class="h-4 w-4 shrink-0" aria-hidden="true" />
      <span>Committed: </span>
      <a
        :href="committed.html_url"
        target="_blank"
        rel="noopener noreferrer"
        class="font-mono underline hover:no-underline"
        aria-label="View commit on GitHub"
      >{{ committed.commit_sha.slice(0, 7) }}</a>
    </div>

    <!-- Commit form -->
    <div v-else class="px-3 py-2 border-t border-border space-y-2">
      <div v-if="!showInput">
        <Button
          size="sm"
          variant="outline"
          class="gap-2"
          aria-label="Accept suggestion and commit"
          @click="openInput"
        >
          <GitCommit class="h-4 w-4" aria-hidden="true" />
          Accept Suggestion
        </Button>
      </div>

      <div v-else class="space-y-2">
        <div class="flex gap-2">
          <Input
            v-model="commitMessage"
            placeholder="Commit message…"
            class="text-sm h-8 flex-1"
            aria-label="Commit message"
            :disabled="submitting"
            @keydown.enter="handleCommit"
          />
          <Button
            size="sm"
            :disabled="submitting || !commitMessage.trim()"
            aria-label="Confirm and commit suggestion"
            @click="handleCommit"
          >
            <Loader2 v-if="submitting" class="h-4 w-4 animate-spin" aria-hidden="true" />
            <span v-else>Commit</span>
          </Button>
          <Button
            size="sm"
            variant="ghost"
            :disabled="submitting"
            aria-label="Cancel committing suggestion"
            @click="showInput = false; error = ''"
          >
            Cancel
          </Button>
        </div>
        <p v-if="error" class="text-xs text-destructive" role="alert">{{ error }}</p>
      </div>
    </div>
  </div>
</template>
