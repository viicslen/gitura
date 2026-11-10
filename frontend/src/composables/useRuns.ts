/**
 * useRuns — module-scoped composable for tracking command execution history.
 *
 * Runs survive component remounts but are cleared on full app restart (in-memory only).
 * The composable subscribes to Wails events:
 *   - "command:run:pending"  → adds a running entry
 *   - "command:run:output"   → appends raw stdout/stderr chunks to the entry
 *   - "command:run:complete" → merges the completed result into the existing entry
 */
import { ref, readonly, computed } from 'vue'
import type { ComputedRef } from 'vue'
import { EventsOn } from '@/wailsjs/runtime/runtime'
import { CancelRun } from '@/wailsjs/go/app/App'
import type { model } from '@/wailsjs/go/models'

export type Run = model.RunResult

type RunOutputEvent = {
  run_id: string
  stream: 'stdout' | 'stderr'
  chunk: string
}

type BufferedOutput = {
  stdout: string
  stderr: string
}

const runs = ref<Run[]>([])
let subscribed = false
const outputBuffers = new Map<string, BufferedOutput>()
let flushTimer: ReturnType<typeof setTimeout> | null = null

const flushIntervalMS = 50

function flushOutputBuffers(): void {
  flushTimer = null

  if (outputBuffers.size === 0) {
    return
  }

  for (const [runID, buffered] of outputBuffers) {
    const idx = runs.value.findIndex((r) => r.run_id === runID)
    if (idx === -1) {
      outputBuffers.delete(runID)
      continue
    }

    if (buffered.stdout.length > 0) {
      runs.value[idx].stdout = (runs.value[idx].stdout ?? '') + buffered.stdout
    }
    if (buffered.stderr.length > 0) {
      runs.value[idx].stderr = (runs.value[idx].stderr ?? '') + buffered.stderr
    }

    outputBuffers.delete(runID)
  }
}

function scheduleFlush(): void {
  if (flushTimer !== null) {
    return
  }
  flushTimer = setTimeout(flushOutputBuffers, flushIntervalMS)
}

function subscribe(): void {
  if (subscribed) return
  subscribed = true

  EventsOn('command:run:pending', (result: Run) => {
    runs.value.unshift({ ...result })
  })

  EventsOn('command:run:complete', (result: Run) => {
    outputBuffers.delete(result.run_id)

    const idx = runs.value.findIndex((r) => r.run_id === result.run_id)
    if (idx !== -1) {
      runs.value[idx] = { ...result }
    } else {
      // Safety net: backend fired complete without a prior pending event.
      runs.value.unshift({ ...result })
    }
  })

  EventsOn('command:run:output', (payload: RunOutputEvent) => {
    const idx = runs.value.findIndex((r) => r.run_id === payload.run_id)
    if (idx === -1) {
      return
    }

    const buffered = outputBuffers.get(payload.run_id) ?? { stdout: '', stderr: '' }
    if (payload.stream === 'stdout') {
      buffered.stdout += payload.chunk
    } else {
      buffered.stderr += payload.chunk
    }
    outputBuffers.set(payload.run_id, buffered)
    scheduleFlush()
  })
}

// Auto-subscribe when this module is first imported.
subscribe()

export function useRuns() {
  return {
    runs: readonly(runs),
    /** Returns a computed ref of runs associated with the given thread root ID. */
    runsForThread(rootId: number): ComputedRef<Run[]> {
      return computed(() => runs.value.filter((r) => r.thread_root_id === rootId))
    },
    clearHistory(): void {
      runs.value = []
      outputBuffers.clear()
      if (flushTimer !== null) {
        clearTimeout(flushTimer)
        flushTimer = null
      }
    },
    async cancelRun(runID: string): Promise<void> {
      await CancelRun(runID)
    },
  }
}
