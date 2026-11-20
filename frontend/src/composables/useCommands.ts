import { ref } from 'vue'
import { GetCommands, GetDefaultCommands } from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'

/** What a run button sends; each kind can have its own default command. */
export const RUN_KINDS = [
  { id: 'pr', label: 'Pull request', hint: 'The whole PR: description, comments, reviews and threads.' },
  { id: 'review', label: 'Review', hint: 'A review and the inline threads it opened.' },
  { id: 'thread', label: 'Thread', hint: 'Every comment in an inline thread.' },
  { id: 'comment', label: 'Comment', hint: 'A general comment or the first comment of a thread.' },
  { id: 'reply', label: 'Reply', hint: 'A reply in a thread.' },
  { id: 'draft', label: 'Pending comment', hint: 'A comment in your review or one you are writing.' },
  { id: 'code', label: 'Code block', hint: 'A code block inside a comment.' },
] as const
export type RunKind = (typeof RUN_KINDS)[number]['id']

// Shared so changes made in the settings dialog reach the open review page. null until first loaded.
export const commands = ref<model.CommandDTO[] | null>(null)
/** Default command name per run kind; "all" covers the kinds missing here. */
export const defaultCommands = ref<Record<string, string>>({})

export function defaultCommandFor(kind: RunKind): string {
  return defaultCommands.value[kind] || defaultCommands.value.all || ''
}

export async function loadDefaultCommands(): Promise<void> {
  try {
    // A nil Go map arrives as null.
    defaultCommands.value = ((await GetDefaultCommands()) ?? {}) as Record<string, string>
  } catch {
    // Non-fatal: run buttons fall back to the first command
  }
}

export async function loadCommands(): Promise<void> {
  try {
    commands.value = await GetCommands()
  } catch {
    // Non-fatal: no run buttons are shown
  }
  await loadDefaultCommands()
}
