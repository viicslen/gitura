import { useEventListener } from '@vueuse/core'
import { reducedMotion } from '@/components/motion/wow'
import { KEYBINDING_ACTIONS, chordOf, kindOf, type KeybindingAction, type Keybindings } from './useKeybindings'

export interface VimActions {
  search: () => void
  back: () => void
  tab: (step: 1 | -1) => void
  bindings: () => Keybindings
}

type VimAction = Exclude<Extract<KeybindingAction, `vim${string}`>, 'vimLeave'>

const TYPING = 'input, textarea, select, [contenteditable=""], [contenteditable="true"]'
const PANE = '[data-vim-pane]'
const ITEM = '[data-vim-item]'
// Where focus lands in a pane entered for the first time, best first.
const ENTRY = ['[aria-current="page"]', '[aria-selected="true"]', '[role="treeitem"][tabindex="0"]', ITEM]
const MODIFIERS = ['Shift', 'Control', 'Alt', 'Meta']
const LINE = 64

const visible = (el: Element): boolean => el.getClientRects().length > 0 && !el.closest('[inert]')
const scrollable = (el: Element): boolean => el.scrollHeight > el.clientHeight && /auto|scroll/.test(getComputedStyle(el).overflowY)

function scrollerOf(el: Element): Element | null {
  // Focus on a pane itself means its content scroller, which sits inside it.
  if (el.matches(PANE)) for (const n of el.querySelectorAll('*')) if (scrollable(n)) return n
  for (let n: Element | null = el; n; n = n.parentElement) if (scrollable(n)) return n
  return null
}

let sending = false
/** Presses `key` on `el` as the widgets there expect; true when one of them handled it. */
function press(el: Element, key: string): boolean {
  sending = true
  try {
    return !el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }))
  } finally {
    sending = false
  }
}

/**
 * Vim-style navigation for the whole app, on the `vim*` keys from Settings → Keybindings. Down, up, left, right,
 * first and last are passed to the focused widget as arrow, Home and End keys, so trees, lists, tabs and the thread
 * pager move as they do with those keys. When nothing takes them, focus steps through the pane's `data-vim-item`
 * rows, or else the pane scrolls. Pane keys move between `data-vim-pane` regions in document order. Ignored while
 * typing (but for the leave key) and inside menus; a key bound to a view shortcut takes precedence.
 */
export function useVimKeys(actions: VimActions): void {
  let pending = ''
  let pointer: Element | null = null
  const lastFocus = new WeakMap<Element, HTMLElement>()

  useEventListener(document, 'pointerdown', (e: PointerEvent) => (pointer = e.target instanceof Element ? e.target : null), { passive: true })
  useEventListener(document, 'focusin', (e: FocusEvent) => {
    const el = e.target instanceof HTMLElement ? e.target : null
    const pane = el && !el.matches(TYPING) ? el.closest(PANE) : null
    if (el && pane) lastFocus.set(pane, el)
  })

  const entry = (pane: Element): HTMLElement | undefined => {
    for (const selector of ENTRY) {
      const found = Array.from(pane.querySelectorAll<HTMLElement>(selector)).find(visible)
      if (found) return found
    }
  }

  function enter(pane: HTMLElement): void {
    const last = lastFocus.get(pane)
    const target = (last?.isConnected && pane.contains(last) && visible(last) ? last : null) ?? entry(pane) ?? pane
    if (target === pane && !pane.hasAttribute('tabindex')) pane.tabIndex = -1
    target.focus()
  }

  function scroll(from: Element, top: number | 'start' | 'end'): void {
    const el = scrollerOf(from)
    if (!el) return
    if (top === 'start' || top === 'end') el.scrollTo({ top: top === 'start' ? 0 : el.scrollHeight, behavior: reducedMotion() ? 'auto' : 'smooth' })
    else el.scrollBy({ top, behavior: Math.abs(top) > LINE && !reducedMotion() ? 'smooth' : 'auto' })
  }

  function half(focus: HTMLElement | null, dir: 1 | -1): void {
    const from = focus ?? (pointer?.isConnected ? pointer : null)
    if (from) scroll(from, dir * 0.5 * (scrollerOf(from)?.clientHeight ?? 0))
  }

  function move(focus: HTMLElement | null, key: 'ArrowDown' | 'ArrowUp' | 'ArrowLeft' | 'ArrowRight' | 'Home' | 'End'): void {
    const vertical = key === 'ArrowDown' || key === 'ArrowUp'
    const edge = key === 'Home' ? 'start' : key === 'End' ? 'end' : null
    if (!focus) {
      // Nothing focused: scroll what was last clicked, or start in the page's first pane with rows.
      if (pointer?.isConnected) return vertical || edge ? scroll(pointer, edge ?? (key === 'ArrowDown' ? LINE : -LINE)) : undefined
      const panes = Array.from(document.querySelectorAll<HTMLElement>(PANE)).filter((p) => visible(p) && p.closest('main'))
      const first = panes.find((p) => entry(p)) ?? panes[0]
      if (first) enter(first)
      return
    }
    if (press(focus, key)) return
    const item = focus.closest<HTMLElement>(ITEM)
    if (item && (vertical || edge)) {
      const items = Array.from((item.closest(PANE) ?? document).querySelectorAll<HTMLElement>(ITEM)).filter(visible)
      const i = items.indexOf(item)
      const next = key === 'Home' ? items[0] : key === 'End' ? items[items.length - 1] : items[i + (key === 'ArrowDown' ? 1 : -1)]
      next?.focus()
      return
    }
    if (vertical || edge) scroll(focus, edge ?? (key === 'ArrowDown' ? LINE : -LINE))
  }

  function switchPane(focus: HTMLElement | null, step: 1 | -1): void {
    const scope = focus?.closest('[role="dialog"]') ?? document
    const panes = Array.from(scope.querySelectorAll<HTMLElement>(PANE)).filter(visible)
    const here = focus?.closest<HTMLElement>(PANE)
    const i = here ? panes.indexOf(here) : -1
    const next = i < 0 ? (step > 0 ? panes[0] : panes[panes.length - 1]) : panes[i + step]
    if (next) enter(next)
  }

  type Command = { run: (focus: HTMLElement | null) => void; global?: boolean }
  const commands: Record<VimAction, Command> = {
    vimDown: { run: (f) => move(f, 'ArrowDown') },
    vimUp: { run: (f) => move(f, 'ArrowUp') },
    vimLeft: { run: (f) => move(f, 'ArrowLeft') },
    vimRight: { run: (f) => move(f, 'ArrowRight') },
    vimFirst: { run: (f) => move(f, 'Home') },
    vimLast: { run: (f) => move(f, 'End') },
    vimHalfDown: { run: (f) => half(f, 1) },
    vimHalfUp: { run: (f) => half(f, -1) },
    vimPrevPane: { run: (f) => switchPane(f, -1) },
    vimNextPane: { run: (f) => switchPane(f, 1) },
    vimNextView: { run: () => actions.tab(1), global: true },
    vimPrevView: { run: () => actions.tab(-1), global: true },
    vimSearch: { run: () => actions.search(), global: true },
    vimBack: { run: () => actions.back(), global: true },
  }
  const ids = Object.keys(commands) as VimAction[]
  const viewKeys = KEYBINDING_ACTIONS.filter((a) => kindOf(a.id) === 'key').map((a) => a.id)

  useEventListener(window, 'keydown', (event: KeyboardEvent) => {
    if (sending || event.defaultPrevented || MODIFIERS.includes(event.key)) return
    const keys = actions.bindings()
    const held = event.ctrlKey || event.metaKey || event.altKey
    const pressed = held ? chordOf(event) : event.key
    const active = document.activeElement
    const focus = active instanceof HTMLElement && active !== document.body ? active : null
    if (focus?.closest(TYPING)) {
      // Leaves the field for its pane, like leaving insert mode; dialogs keep Escape for closing.
      if (pressed !== keys.vimLeave || focus.closest('[role="dialog"]')) return
      event.preventDefault()
      const pane = focus.closest<HTMLElement>(PANE)
      focus.blur()
      if (pane) enter(pane)
      return
    }
    if (focus?.closest('[role="menu"]')) return
    if (!held && viewKeys.some((id) => keys[id] === pressed)) return

    const combo = pending && !held ? `${pending} ${pressed}` : pressed
    pending = ''
    const find = (key: string) => ids.find((id) => keys[id] === key)
    let id = find(combo)
    if (!id && !held && ids.some((v) => keys[v].startsWith(`${combo} `))) {
      pending = combo
      event.preventDefault()
      return
    }
    // A stray first key of a sequence doesn't swallow the key after it.
    if (!id && combo !== pressed) id = find(pressed)
    if (!id || (commands[id].global && focus?.closest('[role="dialog"]'))) return
    event.preventDefault()
    commands[id].run(focus)
  })
}
