import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { effectScope, type EffectScope } from 'vue'
import { useVimKeys, type VimActions } from './useVimKeys'
import { DEFAULT_KEYBINDINGS, type Keybindings } from './useKeybindings'

const press = (key: string, init: KeyboardEventInit = {}) =>
  (document.activeElement ?? document.body).dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init }))
const focused = () => document.activeElement?.id
function byId(id: string): HTMLElement {
  const el = document.getElementById(id)
  if (!el) throw new Error(`no #${id}`)
  return el
}

describe('useVimKeys', () => {
  let scope: EffectScope
  let actions: VimActions
  let keys: Keybindings
  let on: boolean

  beforeEach(() => {
    // jsdom lays nothing out, so every element would count as hidden.
    Object.defineProperty(HTMLElement.prototype, 'getClientRects', { configurable: true, value: () => [new DOMRect()] })
    document.body.innerHTML = `
      <nav data-vim-pane id="rail"><button data-vim-item id="r1"></button><button data-vim-item id="r2" aria-current="page"></button></nav>
      <main>
        <section data-vim-pane id="list">
          <input id="filter" />
          <button data-vim-item id="a"></button><button data-vim-item id="b"></button><button data-vim-item id="c"></button>
        </section>
        <ul data-vim-pane role="tree" id="tree"><li role="treeitem" tabindex="0" id="t1"></li></ul>
      </main>`
    keys = { ...DEFAULT_KEYBINDINGS }
    on = true
    actions = { search: vi.fn(), back: vi.fn(), tab: vi.fn(), bindings: () => keys, enabled: () => on }
    scope = effectScope()
    scope.run(() => useVimKeys(actions))
  })
  afterEach(() => scope.stop())

  it('starts in the first pane with rows, then steps through its rows with j, k, gg and G', () => {
    press('j')
    expect(focused()).toBe('a')
    expect(byId('a').hasAttribute('data-vim-focus')).toBe(true)
    press('j')
    expect(byId('a').hasAttribute('data-vim-focus')).toBe(false)
    expect(byId('b').hasAttribute('data-vim-focus')).toBe(true)
    press('j')
    press('j')
    expect(focused()).toBe('c')
    press('k')
    expect(focused()).toBe('b')
    press('g')
    press('g')
    expect(focused()).toBe('a')
    press('G')
    expect(focused()).toBe('c')
  })

  it('hands hjkl to the focused widget as arrow keys and stops when it takes them', () => {
    const seen: string[] = []
    const tree = byId('tree')
    tree.addEventListener('keydown', (e) => {
      seen.push(e.key)
      if (e.key === 'ArrowDown') e.preventDefault()
    })
    byId('t1').focus()
    press('j')
    press('h')
    expect(seen).toEqual(['j', 'ArrowDown', 'h', 'ArrowLeft'])
    expect(focused()).toBe('t1')
  })

  it('enters the rows of a pane that was focused before they loaded', () => {
    const tree = byId('tree')
    byId('t1').remove()
    byId('b').focus()
    press('l', { ctrlKey: true })
    expect(focused()).toBe('tree')
    tree.innerHTML = '<li role="treeitem" tabindex="0" id="t2"></li>'
    press('j')
    expect(focused()).toBe('t2')
  })

  it('moves between panes with Ctrl+H and Ctrl+L, back to where focus was', () => {
    byId('b').focus()
    press('l', { ctrlKey: true })
    expect(focused()).toBe('t1')
    press('h', { ctrlKey: true })
    expect(focused()).toBe('b')
    press('h', { ctrlKey: true })
    expect(focused()).toBe('r2')
  })

  it('leaves text fields alone, and Escape returns from one to its pane', () => {
    byId('filter').focus()
    expect(press('j')).toBe(true)
    expect(focused()).toBe('filter')
    press('Escape')
    expect(focused()).toBe('a')
  })

  it('runs the global commands, and yields a key bound to a view shortcut', () => {
    byId('a').focus()
    press('g')
    press('T')
    press('/')
    press('q')
    expect(actions.tab).toHaveBeenCalledWith(-1)
    expect(actions.search).toHaveBeenCalledOnce()
    expect(actions.back).toHaveBeenCalledOnce()
    keys.nextFile = 'j'
    expect(press('j')).toBe(true)
    expect(focused()).toBe('a')
  })

  it('drops the first key of a sequence once the second is too late', () => {
    vi.useFakeTimers({ toFake: ['Date', 'performance'] })
    try {
      byId('a').focus()
      press('g')
      vi.advanceTimersByTime(1000)
      press('t')
      expect(actions.tab).not.toHaveBeenCalled()
      press('g')
      press('t')
      expect(actions.tab).toHaveBeenCalledWith(1)
    } finally {
      vi.useRealTimers()
    }
  })

  it('does nothing while turned off, or for a key left unbound', () => {
    byId('a').focus()
    on = false
    expect(press('j')).toBe(true)
    expect(press('/')).toBe(true)
    expect(actions.search).not.toHaveBeenCalled()
    on = true
    keys.vimDown = ''
    expect(press('j')).toBe(true)
    expect(focused()).toBe('a')
  })

  it('scrolls the scroller a pane marks, not the first one in it', () => {
    const tree = byId('tree')
    tree.innerHTML = '<div id="s1" style="overflow-y:auto"></div><div id="s2" data-vim-scroll style="overflow-y:auto"></div>'
    const scrolled: string[] = []
    for (const id of ['s1', 's2']) {
      const el = byId(id)
      Object.defineProperty(el, 'scrollHeight', { value: 1000 })
      Object.defineProperty(el, 'clientHeight', { value: 400 })
      el.scrollBy = () => scrolled.push(id)
    }
    tree.tabIndex = -1
    tree.focus()
    press('d', { ctrlKey: true })
    expect(scrolled).toEqual(['s2'])
  })

  it('follows rebound keys, sequences and chords included', () => {
    keys = { ...keys, vimDown: 'e', vimFirst: 'z z', vimNextPane: 'Alt+L', vimLeave: 'Ctrl+[' }
    byId('a').focus()
    expect(press('j')).toBe(true)
    press('e')
    expect(focused()).toBe('b')
    press('z')
    press('z')
    expect(focused()).toBe('a')
    press('l', { altKey: true })
    expect(focused()).toBe('t1')
    byId('filter').focus()
    expect(press('Escape')).toBe(true)
    press('[', { ctrlKey: true })
    expect(focused()).toBe('a')
  })
})
