import { nextTick } from 'vue'
import { toast } from 'vue-sonner'
import { UpdateBodyIfUnchanged } from '@/wailsjs/go/app/App'
import { setTaskListItem } from '@/lib/markdown'

export type TaskToggle = (index: number, checked: boolean) => Promise<void>
export type TaskSource = 'pr_body' | 'issue_comment' | 'review_comment' | 'review'

// One queue per body, so a second click edits the body the first click produced.
const queues = new WeakMap<object, Promise<unknown>>()

/** Toggles a checkbox in `target`'s body on GitHub and writes the new body back onto it. */
export function taskToggler(kind: TaskSource, id: number, target: { body: string }): TaskToggle {
  return (index, checked) => {
    const run = (queues.get(target) ?? Promise.resolve()).catch(() => {}).then(async () => {
      const next = setTaskListItem(target.body, index, checked)
      if (next === null) throw new Error('this checkbox changed; refresh to see the latest')
      target.body = await UpdateBodyIfUnchanged(kind, id, target.body, next)
    })
    queues.set(target, run)
    return run
  }
}

/** `change` handler for rendered markdown; `offset` shifts indices when a body is rendered in parts. */
export async function onTaskChange(event: Event, toggle?: TaskToggle, offset = 0): Promise<void> {
  const box = event.target
  if (!toggle || !(box instanceof HTMLInputElement) || box.dataset.taskIndex === undefined) return
  const container = box.closest('.markdown-body')
  const index = box.dataset.taskIndex
  box.disabled = true
  try {
    await toggle(Number(index) + offset, box.checked)
    // The saved body re-renders and replaces the checkbox; keep keyboard focus on it.
    await nextTick()
    container?.querySelector<HTMLInputElement>(`input[data-task-index="${index}"]`)?.focus()
  } catch (err) {
    box.checked = !box.checked
    toast.error('Could not update checkbox: ' + String(err))
  } finally {
    box.disabled = false
  }
}
