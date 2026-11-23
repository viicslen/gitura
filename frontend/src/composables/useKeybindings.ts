import { ref } from 'vue'
import { GetKeybindings, SetKeybindings } from '@/bindings/gitura/internal/app/app'
import { errorMessage } from '@/lib/utils'

/**
 * `kind` is how a key is bound: a single key without modifiers (default), a `chord` held with Ctrl, Alt or Meta,
 * a `sequence` of one or two keys typed in turn (Shift allowed, steps stored space-separated: `g g`), or `leave`,
 * which fires while typing so it must be Escape or a chord.
 */
export const KEYBINDING_ACTIONS = [
  { id: 'prevFile', label: 'Previous file', view: 'Files', default: '[' },
  { id: 'nextFile', label: 'Next file', view: 'Files', default: ']' },
  { id: 'prevThread', label: 'Previous thread', view: 'Conversation', default: 'ArrowLeft' },
  { id: 'nextThread', label: 'Next thread', view: 'Conversation', default: 'ArrowRight' },
  { id: 'quickOpen', label: 'Open a pull request', view: 'Anywhere', default: 'Ctrl+K', kind: 'chord' },
  { id: 'vimDown', label: 'Next row, or scroll down', view: 'Vim', default: 'j', kind: 'sequence' },
  { id: 'vimUp', label: 'Previous row, or scroll up', view: 'Vim', default: 'k', kind: 'sequence' },
  { id: 'vimLeft', label: 'Left: previous thread, or the tab before', view: 'Vim', default: 'h', kind: 'sequence' },
  { id: 'vimRight', label: 'Right: next thread, or the next tab', view: 'Vim', default: 'l', kind: 'sequence' },
  { id: 'vimFirst', label: 'First row, or the top', view: 'Vim', default: 'g g', kind: 'sequence' },
  { id: 'vimLast', label: 'Last row, or the bottom', view: 'Vim', default: 'G', kind: 'sequence' },
  { id: 'vimHalfDown', label: 'Scroll half a page down', view: 'Vim', default: 'Ctrl+D', kind: 'chord' },
  { id: 'vimHalfUp', label: 'Scroll half a page up', view: 'Vim', default: 'Ctrl+U', kind: 'chord' },
  { id: 'vimPrevPane', label: 'Previous pane', view: 'Vim', default: 'Ctrl+H', kind: 'chord' },
  { id: 'vimNextPane', label: 'Next pane', view: 'Vim', default: 'Ctrl+L', kind: 'chord' },
  { id: 'vimNextView', label: 'Next review view', view: 'Vim', default: 'g t', kind: 'sequence' },
  { id: 'vimPrevView', label: 'Previous review view', view: 'Vim', default: 'g T', kind: 'sequence' },
  { id: 'vimSearch', label: 'Open a pull request', view: 'Vim', default: '/', kind: 'sequence' },
  { id: 'vimBack', label: 'Back to the pull request list', view: 'Vim', default: 'q', kind: 'sequence' },
  { id: 'vimLeave', label: 'Leave a text field', view: 'Vim', default: 'Escape', kind: 'leave' },
] as const

export type KeybindingAction = (typeof KEYBINDING_ACTIONS)[number]['id']
export type Keybindings = Record<KeybindingAction, string>
type Kind = 'key' | 'chord' | 'sequence' | 'leave'

export const DEFAULT_KEYBINDINGS = Object.fromEntries(KEYBINDING_ACTIONS.map((a) => [a.id, a.default])) as Keybindings
const bindings = ref<Keybindings>({ ...DEFAULT_KEYBINDINGS })
let loaded: Promise<void> | null = null

const KEY_LABELS: Record<string, string> = { ArrowLeft: '←', ArrowRight: '→', ArrowUp: '↑', ArrowDown: '↓', ' ': 'Space', Escape: 'Esc' }

export function keyLabel(key: string): string {
  if (key.trim() && key.includes(' ')) return key.split(' ').map(keyLabel).join(' ')
  return KEY_LABELS[key] ?? key
}

export function kindOf(action: KeybindingAction): Kind {
  const def = KEYBINDING_ACTIONS.find((a) => a.id === action)
  return def && 'kind' in def ? def.kind : 'key'
}

/** The key from a keydown with its held modifiers, as stored for a chord action: `Ctrl+P`, `Ctrl+Shift+K`. */
export function chordOf(event: KeyboardEvent): string {
  const mods = [event.ctrlKey && 'Ctrl', event.altKey && 'Alt', event.shiftKey && 'Shift', event.metaKey && 'Meta'].filter(Boolean)
  return [...mods, event.key.length === 1 ? event.key.toUpperCase() : event.key].join('+')
}

/**
 * The key a keydown binds to an action, or why it can't: view shortcuts never fire with a modifier held, chord
 * actions work anywhere (even while typing) so they need Ctrl, Alt or Meta, and Space would press buttons.
 */
export function keyFor(action: KeybindingAction, event: KeyboardEvent): string | { error: string } {
  const held = event.ctrlKey || event.metaKey || event.altKey
  switch (kindOf(action)) {
    case 'chord':
      return held ? chordOf(event) : { error: 'Hold Ctrl, Alt or Meta with the key.' }
    case 'leave':
      return held ? chordOf(event) : event.key === 'Escape' ? 'Escape' : { error: 'Use Escape, or hold Ctrl, Alt or Meta with the key.' }
    case 'sequence':
      if (held) return { error: 'Use keys without Ctrl, Alt or Meta.' }
      return event.key === ' ' ? { error: 'Space presses buttons, so it can’t be used here.' } : event.key
    default:
      return held || event.shiftKey ? { error: 'Use a single key without modifiers.' } : event.key
  }
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
 * Binds `key` to an action, or unbinds it when `key` is empty. Resolves to an error message instead when another
 * action in the same view (or one that works anywhere) has it, or when one sequence would start another, so the
 * shorter could never wait for the longer.
 */
export async function setBinding(action: KeybindingAction, key: string): Promise<string | null> {
  if (!key) return save({ ...bindings.value, [action]: '' })
  const def = KEYBINDING_ACTIONS.find((a) => a.id === action)
  const others = KEYBINDING_ACTIONS.filter((a) => a.id !== action && (a.view === def?.view || a.view === 'Anywhere' || def?.view === 'Anywhere'))
  const clash = others.find((a) => bindings.value[a.id] === key)
  if (clash) return `${keyLabel(key)} is already used for ${clash.label}.`
  if (kindOf(action) === 'sequence') {
    const overlap = others.find((a) => {
      const other = bindings.value[a.id]
      return kindOf(a.id) === 'sequence' && !!other && (other.startsWith(`${key} `) || key.startsWith(`${other} `))
    })
    if (overlap) return `${keyLabel(key)} overlaps ${keyLabel(bindings.value[overlap.id])}, used for ${overlap.label}.`
  }
  return save({ ...bindings.value, [action]: key })
}

/** Binds the key from a keydown to an action; see keyFor and setBinding for why it may resolve to an error message. */
export async function bindKey(action: KeybindingAction, event: KeyboardEvent): Promise<string | null> {
  const key = keyFor(action, event)
  return typeof key === 'string' ? setBinding(action, key) : key.error
}

/** Shortcut keys from settings.toml, falling back to defaults until loaded or when loading fails. */
export function useKeybindings() {
  loaded ??= GetKeybindings()
    .then((saved) => {
      const known = Object.entries(saved ?? {}).filter(([id]) => id in DEFAULT_KEYBINDINGS)
      bindings.value = { ...DEFAULT_KEYBINDINGS, ...Object.fromEntries(known) }
    })
    .catch(() => {})
  return {
    bindings,
    reset: () => save({ ...DEFAULT_KEYBINDINGS }),
  }
}
