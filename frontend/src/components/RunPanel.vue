<script setup lang="ts">
import { computed, ref } from 'vue'
import { X, ChevronDown, ChevronUp, Loader2, CheckCircle2, XCircle, Terminal, Trash2, Square } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Badge } from '@/components/ui/badge'
import { useRuns } from '@/composables/useRuns'
import type { Run } from '@/composables/useRuns'
import { AnsiUp } from 'ansi_up'

const ansiUp = new AnsiUp()

function ansiToHtml(text: string): string {
  return ansiUp.ansi_to_html(text)
}

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { runs, clearHistory, cancelRun } = useRuns()

// Which run is expanded (showing stdout/stderr)
const expandedRunID = ref<string | null>(null)

function toggleExpand(runID: string): void {
  expandedRunID.value = expandedRunID.value === runID ? null : runID
}

function statusIcon(run: Run): 'running' | 'success' | 'error' | 'cancelled' {
  if (run.running) return 'running'
  if (run.cancelled) return 'cancelled'
  if (run.exit_code === 0) return 'success'
  return 'error'
}

function formatDuration(run: Run): string {
  if (run.running || !run.started_at || !run.finished_at) return ''
  const start = new Date(run.started_at).getTime()
  const end = new Date(run.finished_at).getTime()
  const ms = end - start
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

const hasRuns = computed(() => runs.value.length > 0)
</script>

<template>
  <!-- Slide-up panel from bottom of screen -->
  <Transition
    enter-active-class="transition-all duration-200 ease-out"
    enter-from-class="translate-y-full opacity-0"
    enter-to-class="translate-y-0 opacity-100"
    leave-active-class="transition-all duration-150 ease-in"
    leave-from-class="translate-y-0 opacity-100"
    leave-to-class="translate-y-full opacity-0"
  >
    <div
      v-if="open"
      class="flex flex-col border-t border-border bg-background"
      style="height: 260px;"
    >
      <!-- Panel header -->
      <div class="flex items-center gap-2 px-4 py-2 border-b border-border shrink-0">
        <Terminal class="h-4 w-4 text-muted-foreground" aria-hidden="true" />
        <span class="text-sm font-medium">Run History</span>
        <Badge v-if="runs.length > 0" variant="secondary" class="text-xs px-1.5 py-0">
          {{ runs.length }}
        </Badge>
        <div class="flex-1" />
        <Button
          v-if="hasRuns"
          variant="ghost"
          size="sm"
          class="h-6 gap-1 px-2 text-xs text-muted-foreground"
          aria-label="Clear run history"
          @click="clearHistory"
        >
          <Trash2 class="h-3 w-3" />
          Clear
        </Button>
        <Button
          variant="ghost"
          size="icon"
          class="h-6 w-6"
          aria-label="Close run history"
          @click="emit('close')"
        >
          <X class="h-3.5 w-3.5" />
        </Button>
      </div>

      <!-- Run list -->
      <ScrollArea class="flex-1 min-h-0">
        <!-- Empty state -->
        <div
          v-if="!hasRuns"
          class="flex flex-col items-center justify-center h-full py-8 text-muted-foreground"
        >
          <Terminal class="h-6 w-6 opacity-30 mb-2" aria-hidden="true" />
          <p class="text-xs">No runs yet. Run a command from a comment to see results here.</p>
        </div>

        <div v-else class="divide-y divide-border">
          <div
            v-for="run in runs"
            :key="run.run_id"
            class="group"
          >
            <!-- Run summary row -->
            <button
              class="w-full flex items-center gap-3 px-4 py-2.5 text-left hover:bg-muted/40 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              :aria-expanded="expandedRunID === run.run_id"
              @click="toggleExpand(run.run_id)"
            >
              <!-- Status icon -->
              <Loader2
                v-if="statusIcon(run) === 'running'"
                class="h-4 w-4 shrink-0 animate-spin text-muted-foreground"
              />
              <CheckCircle2
                v-else-if="statusIcon(run) === 'success'"
                class="h-4 w-4 shrink-0 text-green-600 dark:text-green-400"
              />
              <Square
                v-else-if="statusIcon(run) === 'cancelled'"
                class="h-4 w-4 shrink-0 text-muted-foreground"
              />
              <XCircle
                v-else
                class="h-4 w-4 shrink-0 text-destructive"
              />

              <!-- Command name -->
              <span class="text-sm font-medium truncate flex-1">{{ run.command_name }}</span>

              <!-- Exit code badge (when finished) -->
              <Badge
                v-if="!run.running && !run.cancelled"
                :variant="run.exit_code === 0 ? 'secondary' : 'destructive'"
                class="text-xs shrink-0"
              >
                exit {{ run.exit_code }}
              </Badge>
              <Badge
                v-else-if="run.cancelled"
                variant="secondary"
                class="text-xs shrink-0 text-muted-foreground"
              >
                cancelled
              </Badge>

              <!-- Stop button (only while running) -->
              <Button
                v-if="run.running"
                variant="ghost"
                size="icon"
                class="h-5 w-5 shrink-0 text-muted-foreground hover:text-destructive"
                aria-label="Stop this run"
                @click.stop="cancelRun(run.run_id)"
              >
                <Square class="h-3 w-3" />
              </Button>

              <!-- Duration -->
              <span v-if="formatDuration(run)" class="text-xs text-muted-foreground shrink-0">
                {{ formatDuration(run) }}
              </span>

              <!-- Expand icon -->
              <ChevronUp v-if="expandedRunID === run.run_id" class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <ChevronDown v-else class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            </button>

            <!-- Expanded output -->
            <div
              v-if="expandedRunID === run.run_id"
              class="bg-muted/30 border-t border-border px-4 py-3 space-y-2"
            >
              <!-- Input used -->
              <div v-if="run.input" class="space-y-1">
                <p class="text-xs text-muted-foreground font-medium uppercase tracking-wide">Input</p>
                <pre class="text-xs font-mono bg-muted rounded px-2 py-1.5 overflow-x-auto whitespace-pre-wrap line-clamp-3">{{ run.input }}</pre>
              </div>

              <!-- stdout -->
              <div v-if="run.stdout" class="space-y-1">
                <p class="text-xs text-muted-foreground font-medium uppercase tracking-wide">Output</p>
                <pre class="text-xs font-mono bg-muted rounded px-2 py-1.5 overflow-x-auto max-h-32 overflow-y-auto whitespace-pre-wrap" v-html="ansiToHtml(run.stdout)" />
              </div>

              <!-- stderr -->
              <div v-if="run.stderr" class="space-y-1">
                <p class="text-xs text-muted-foreground font-medium uppercase tracking-wide">Stderr</p>
                <pre
                  class="text-xs font-mono rounded px-2 py-1.5 overflow-x-auto max-h-24 overflow-y-auto whitespace-pre-wrap"
                  :class="run.exit_code !== 0 ? 'bg-destructive/10 text-destructive' : 'bg-muted text-muted-foreground'"
                  v-html="ansiToHtml(run.stderr)"
                />
              </div>

              <!-- Running state (no output yet) -->
              <div v-if="run.running && !run.stdout && !run.stderr" class="flex items-center gap-2 text-xs text-muted-foreground">
                <Loader2 class="h-3 w-3 animate-spin" />
                Running…
              </div>
            </div>
          </div>
        </div>
      </ScrollArea>
    </div>
  </Transition>
</template>
