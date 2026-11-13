<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { X, ChevronDown, Loader2, CheckCircle2, XCircle, Terminal, Trash2, Square } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Badge } from '@/components/ui/badge'
import { useRuns } from '@/composables/useRuns'
import type { Run } from '@/composables/useRuns'
import RunOutput from './RunOutput.vue'

const props = defineProps<{
  open: boolean
  /** Root ID of the currently-viewed thread; when set, enables the "This thread" scope. */
  currentThreadRootId?: number
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { runs, runsForThread, clearHistory, cancelRun } = useRuns()

// Scope toggle: 'all' shows every run; 'thread' shows only runs for the current thread.
type Scope = 'all' | 'thread'
const scope = ref<Scope>('all')

// When the thread changes, keep the scope stable (user choice persists).
// When scope is 'thread' but there's no current thread, fall back to 'all' display.
const displayedRuns = computed<readonly Run[]>(() => {
  if (scope.value === 'thread' && props.currentThreadRootId) {
    return runsForThread(props.currentThreadRootId).value
  }
  return runs.value
})

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
const hasDisplayedRuns = computed(() => displayedRuns.value.length > 0)

// The panel's content stays mounted until the closing height transition has finished.
const rendered = ref(props.open)
watch(() => props.open, (open) => { if (open) rendered.value = true })
function onHeightEnd(e: TransitionEvent): void {
  if (e.propertyName === 'height' && !props.open) rendered.value = false
}

// Moving between threads can be a held key: the thread-scoped list then changes without motion.
const listMotion = ref(true)
watch(() => props.currentThreadRootId, () => {
  if (scope.value !== 'thread') return
  listMotion.value = false
  void nextTick(() => { listMotion.value = true })
})

const rowMotionProps = computed(() => listMotion.value
  ? {
      // Spelled differently from the enter and leave classes: Vue strips the move classes when the move ends.
      moveClass: 'transition-[transform,opacity] ease-in-out',
      enterActiveClass: 'transition-[opacity,transform]',
      enterFromClass: 'opacity-0',
      leaveActiveClass: 'absolute inset-x-0 transition-[opacity,transform] duration-fast',
      leaveToClass: 'opacity-0',
    }
  : { css: false })

const swapEnter = 'animate-in fade-in-0 animation-duration-base'
const swapLeave = 'animate-out fade-out-0 animation-duration-fast'
</script>

<template>
  <div
    class="shrink-0 overflow-hidden transition-[height]"
    :class="open ? 'h-[260px] duration-slow' : 'h-0 duration-base'"
    :inert="!open"
    @transitionend.self="onHeightEnd"
  >
    <div v-if="rendered" class="flex h-[260px] flex-col border-t border-border bg-background">
      <!-- Panel header -->
      <div class="flex items-center gap-2 px-4 py-2 border-b border-border shrink-0">
        <Terminal class="h-4 w-4 text-muted-foreground" aria-hidden="true" />
        <span class="text-sm font-medium">Run History</span>
        <Badge
          variant="secondary"
          class="text-xs px-1.5 py-0 tabular-nums transition-[opacity,visibility]"
          :class="hasRuns ? 'duration-fast' : 'invisible opacity-0 duration-faster'"
        >
          {{ runs.length }}
        </Badge>

        <!-- Scope toggle (only shown when a thread is active) -->
        <Transition :enter-active-class="swapEnter" :leave-active-class="swapLeave">
          <div
            v-if="currentThreadRootId"
            class="relative ml-1 grid grid-cols-2 rounded-md border border-border text-xs overflow-hidden"
          >
            <span
              class="absolute inset-y-0 left-0 w-1/2 bg-muted transition-transform ease-in-out"
              :class="scope === 'thread' && 'translate-x-full'"
              aria-hidden="true"
            />
            <button
              class="relative px-2 py-0.5 transition-colors"
              :class="scope === 'all' ? 'text-foreground' : 'text-muted-foreground hover:text-foreground'"
              :aria-pressed="scope === 'all'"
              @click="scope = 'all'"
            >All</button>
            <button
              class="relative px-2 py-0.5 transition-colors border-l border-border"
              :class="scope === 'thread' ? 'text-foreground' : 'text-muted-foreground hover:text-foreground'"
              :aria-pressed="scope === 'thread'"
              @click="scope = 'thread'"
            >This thread</button>
          </div>
        </Transition>

        <div class="flex-1" />
        <!-- The fade sits on a wrapper: the button zeroes its own transitions while it has keyboard focus -->
        <span
          class="flex transition-[opacity,visibility]"
          :class="hasRuns ? 'duration-fast' : 'invisible opacity-0 duration-faster'"
        >
          <Button
            variant="ghost"
            size="sm"
            class="h-6 gap-1 px-2 text-xs text-muted-foreground"
            aria-label="Clear run history"
            @click="clearHistory"
          >
            <Trash2 class="h-3 w-3" />
            Clear
          </Button>
        </span>
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
        <div class="relative">
          <Transition
            :css="listMotion"
            :enter-active-class="swapEnter"
            :leave-active-class="`absolute inset-x-0 top-0 ${swapLeave}`"
          >
            <!-- Empty state -->
            <div
              v-if="!hasDisplayedRuns"
              :key="scope"
              class="flex flex-col items-center justify-center py-8 text-muted-foreground"
            >
              <Terminal class="h-6 w-6 opacity-30 mb-2" aria-hidden="true" />
              <p class="text-xs">
                <template v-if="scope === 'thread'">No runs for this thread yet.</template>
                <template v-else>No runs yet. Run a command from a comment to see results here.</template>
              </p>
            </div>

            <TransitionGroup v-else key="runs" tag="div" class="relative divide-y divide-border" v-bind="rowMotionProps">
              <div v-for="run in displayedRuns" :key="run.run_id">
                <!-- Run summary row -->
                <button
                  class="w-full flex items-center gap-3 px-4 py-2.5 text-left hover:bg-muted/40 transition-colors duration-faster hover:duration-0 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  :aria-expanded="expandedRunID === run.run_id"
                  @click="toggleExpand(run.run_id)"
                >
                  <!-- Status icon -->
                  <span class="grid size-4 shrink-0 *:col-start-1 *:row-start-1">
                    <Transition :enter-active-class="`${swapEnter} zoom-in-75`" :leave-active-class="`${swapLeave} zoom-out-75`">
                      <span v-if="statusIcon(run) === 'running'" key="running">
                        <Loader2 class="h-4 w-4 animate-spin text-muted-foreground" />
                      </span>
                      <CheckCircle2
                        v-else-if="statusIcon(run) === 'success'"
                        key="success"
                        class="h-4 w-4 text-green-600 dark:text-green-400"
                      />
                      <Square
                        v-else-if="statusIcon(run) === 'cancelled'"
                        key="cancelled"
                        class="h-4 w-4 text-muted-foreground"
                      />
                      <XCircle v-else key="error" class="h-4 w-4 text-destructive" />
                    </Transition>
                  </span>

                  <!-- Command name -->
                  <span class="text-sm font-medium truncate flex-1">{{ run.command_name }}</span>

                  <!-- Exit code badge (when finished) -->
                  <Transition :enter-active-class="swapEnter">
                    <Badge
                      v-if="!run.running"
                      :variant="run.cancelled || run.exit_code === 0 ? 'secondary' : 'destructive'"
                      class="text-xs shrink-0"
                      :class="run.cancelled && 'text-muted-foreground'"
                    >
                      {{ run.cancelled ? 'cancelled' : `exit ${run.exit_code}` }}
                    </Badge>
                  </Transition>

                  <!-- Stop button while running, then the duration, in one cell -->
                  <span class="grid min-w-11 shrink-0 items-center justify-items-end *:col-start-1 *:row-start-1">
                    <Transition :enter-active-class="swapEnter" :leave-active-class="swapLeave">
                      <Button
                        v-if="run.running"
                        key="stop"
                        variant="ghost"
                        size="icon"
                        class="h-5 w-5 text-muted-foreground hover:text-destructive"
                        aria-label="Stop this run"
                        @click.stop="cancelRun(run.run_id)"
                      >
                        <Square class="h-3 w-3" />
                      </Button>
                      <span v-else key="duration" class="text-xs text-muted-foreground tabular-nums">
                        {{ formatDuration(run) }}
                      </span>
                    </Transition>
                  </span>

                  <!-- Expand icon -->
                  <ChevronDown
                    class="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform"
                    :class="expandedRunID === run.run_id ? 'rotate-180 duration-base' : 'duration-fast'"
                  />
                </button>

                <!-- Expanded output -->
                <Transition
                  enter-active-class="transition-[grid-template-rows] duration-base"
                  enter-from-class="grid-rows-[0fr]!"
                  leave-active-class="transition-[grid-template-rows] duration-fast"
                  leave-to-class="grid-rows-[0fr]!"
                >
                  <div v-if="expandedRunID === run.run_id" class="grid grid-rows-[1fr]">
                    <div class="min-h-0 overflow-hidden">
                      <RunOutput :run="run" />
                    </div>
                  </div>
                </Transition>
              </div>
            </TransitionGroup>
          </Transition>
        </div>
      </ScrollArea>
    </div>
  </div>
</template>
