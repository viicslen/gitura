import { ref } from 'vue'
import { GetKeybindings, SetKeybindings } from '@/bindings/gitura/internal/app/app'
import { errorMessage } from '@/lib/utils'

export const KEYBINDING_ACTIONS = [
  { id: 'prevFile', label: 'Previous file', view: 'Files', default: '[' },
  { id: 'nextFile', label: 'Next file', view: 'Files', default: ']' },
  { id: 'prevThread', label: 'Previous thread', view: 'Conversation', default: 'ArrowLeft' },
  { id: 'nextThread', label: 'Next thread', view: 'Conversation', default: 'ArrowRight' },
  { id: 'quickOpen', label: 'Open a pull request', view: 'Anywhere', default: 'Ctrl+K', chord: true },
] as const

export type KeybindingAction = (typeof KEYBINDING_ACTIONS)[number]['id']
type Keybindings = Record<KeybindingAction, string>

const defaults = Object.fromEntries(KEYBINDING_ACTIONS.map((a) => [a.id, a.default])) as Keybindings
const bindings = ref<Keybindings>({ ...defaults })
let loaded: Promise<void> | null = null

const KEY_LABELS: Record<string, string> = { ArrowLeft: '←', ArrowRight: '→', ArrowUp: '↑', ArrowDown: '↓', ' ': 'Space' }

export function keyLabel(key: string): string {
  return KEY_LABELS[key] ?? (key.length === 1 ? key.toUpperCase() : key)
}

/** The key from a keydown with its held modifiers, as stored for a chord action: `Ctrl+P`, `Ctrl+Shift+K`. */
export function chordOf(event: KeyboardEvent): string {
  const mods = [event.ctrlKey && 'Ctrl', event.altKey && 'Alt', event.shiftKey && 'Shift', event.metaKey && 'Meta'].filter(Boolean)
  return [...mods, event.key.length === 1 ? event.key.toUpperCase() : event.key].join('+')
}

async function save(next: Keybindings): Promise<string | null> {
  try {
    await SetKeybindings(next)
    bindings.value = next
    return null
  } catch (e: unknown) {
    return errorMessage(e)
  }
}

/**
 * Binds the key from a keydown to an action. Resolves to an error message instead when the key can't be used:
 * view shortcuts never fire with a modifier held, chord actions work anywhere (even while typing) so they need
 * Ctrl, Alt or Meta, and two actions in the same view can't share a key.
 */
export async function bindKey(action: KeybindingAction, event: KeyboardEvent): Promise<string | null> {
  const def = KEYBINDING_ACTIONS.find((a) => a.id === action)
  const chord = def && 'chord' in def
  if (chord && !(event.ctrlKey || event.metaKey || event.altKey)) return 'Hold Ctrl, Alt or Meta with the key.'
  if (!chord && (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey)) return 'Use a single key without modifiers.'
  const key = chord ? chordOf(event) : event.key
  const clash = KEYBINDING_ACTIONS.find((a) => a.id !== action && a.view === def?.view && bindings.value[a.id] === key)
  if (clash) return `${keyLabel(key)} is already used for ${clash.label}.`
  return save({ ...bindings.value, [action]: key })
}

/** Shortcut keys from settings.toml, falling back to defaults until loaded or when loading fails. */
export function useKeybindings() {
  loaded ??= GetKeybindings()
    .then((saved) => {
      const known = Object.entries(saved ?? {}).filter(([id]) => id in defaults)
      bindings.value = { ...defaults, ...Object.fromEntries(known) }
    })
    .catch(() => {})
  return {
    bindings,
    reset: () => save({ ...defaults }),
  }
}
