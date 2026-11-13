<script setup lang="ts">
/**
 * RunOutput — the input and streamed output of one run.
 *
 * Its own component so a streamed chunk re-renders only this block, never the
 * run list around it, which measures its rows to animate them.
 */
import { Loader2 } from '@lucide/vue'
import { AnsiUp } from 'ansi_up'
import type { Run } from '@/composables/useRuns'

defineProps<{ run: Run }>()

const ansiUp = new AnsiUp()

function ansiToHtml(text: string): string {
  return ansiUp.ansi_to_html(text)
}
</script>

<template>
  <div class="bg-muted/30 border-t border-border px-4 py-3 space-y-2">
    <div v-if="run.input" class="space-y-1">
      <p class="text-xs text-muted-foreground font-medium uppercase tracking-wide">Input</p>
      <pre class="text-xs font-mono bg-muted rounded px-2 py-1.5 overflow-x-auto whitespace-pre-wrap line-clamp-3">{{ run.input }}</pre>
    </div>

    <div v-if="run.stdout" class="space-y-1">
      <p class="text-xs text-muted-foreground font-medium uppercase tracking-wide">Output</p>
      <pre class="text-xs font-mono bg-muted rounded px-2 py-1.5 overflow-x-auto max-h-32 overflow-y-auto whitespace-pre-wrap" v-html="ansiToHtml(run.stdout)" />
    </div>

    <div v-if="run.stderr" class="space-y-1">
      <p class="text-xs text-muted-foreground font-medium uppercase tracking-wide">Stderr</p>
      <pre
        class="text-xs font-mono rounded px-2 py-1.5 overflow-x-auto max-h-24 overflow-y-auto whitespace-pre-wrap"
        :class="run.exit_code !== 0 ? 'bg-destructive/10 text-destructive' : 'bg-muted text-muted-foreground'"
        v-html="ansiToHtml(run.stderr)"
      />
    </div>

    <div v-if="run.running && !run.stdout && !run.stderr" class="flex items-center gap-2 text-xs text-muted-foreground">
      <Loader2 class="h-3 w-3 animate-spin" />
      Running…
    </div>
  </div>
</template>
