/**
 * useRuns — module-scoped composable for tracking command execution history.
 *
 * Runs survive component remounts but are cleared on full app restart (in-memory only).
 * The composable subscribes to Wails events:
 *   - "command:run:pending"  → adds a running entry
 *   - "command:run:complete" → merges the completed result into the existing entry
 */
import { ref, readonly, computed } from 'vue'
import type { ComputedRef } from 'vue'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { CancelRun } from '../wailsjs/go/main/App'
import type { model } from '../wailsjs/go/models'

export type Run = model.RunResult

const runs = ref<Run[]>([])
let subscribed = false

function subscribe(): void {
  if (subscribed) return
  subscribed = true

  EventsOn('command:run:pending', (result: Run) => {
    runs.value.unshift({ ...result })
  })

  EventsOn('command:run:complete', (result: Run) => {
    const idx = runs.value.findIndex((r) => r.run_id === result.run_id)
    if (idx !== -1) {
      runs.value[idx] = { ...result }
    } else {
      // Safety net: backend fired complete without a prior pending event.
      runs.value.unshift({ ...result })
    }
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
    },
    async cancelRun(runID: string): Promise<void> {
      await CancelRun(runID)
    },
  }
}
