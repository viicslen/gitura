import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { GetKeybindings, SetKeybindings } from '@/bindings/gitura/internal/app/app'
import { bindKey, useKeybindings } from './useKeybindings'

vi.mock('@/bindings/gitura/internal/app/app', () => ({
  GetKeybindings: vi.fn(async () => ({ nextFile: 'n', bogus: 'x' })),
  SetKeybindings: vi.fn(async () => {}),
}))

const key = (k: string, init: KeyboardEventInit = {}) => new KeyboardEvent('keydown', { key: k, ...init })

describe('useKeybindings', () => {
  // Don't call useKeybindings() in a describe body: it loads once, at collection, and Vitest clears mock history before each test.
  it('merges saved bindings over defaults, dropping unknown actions', async () => {
    const { bindings } = useKeybindings()
    await flushPromises()
    expect(GetKeybindings).toHaveBeenCalledOnce()
    expect(bindings.value).toEqual({ prevFile: '[', nextFile: 'n', prevThread: 'ArrowLeft', nextThread: 'ArrowRight' })
  })

  describe('bindKey', () => {
    let bindings: ReturnType<typeof useKeybindings>['bindings']
    beforeEach(async () => {
      const keybindings = useKeybindings()
      bindings = keybindings.bindings
      await keybindings.reset()
    })

    it('saves a rebind, rejecting modifiers and same-view clashes but allowing cross-view reuse', async () => {
      expect(await bindKey('nextFile', key('j'))).toBeNull()
      expect(bindings.value.nextFile).toBe('j')
      expect(SetKeybindings).toHaveBeenLastCalledWith(expect.objectContaining({ nextFile: 'j' }))
      expect(await bindKey('nextFile', key('k', { ctrlKey: true }))).toMatch(/modifiers/)
      expect(await bindKey('prevFile', key('j'))).toMatch(/Next file/)
      expect(bindings.value.prevFile).toBe('[')
      expect(await bindKey('nextThread', key('j'))).toBeNull()
    })

    it('keeps the old binding when saving fails', async () => {
      vi.mocked(SetKeybindings).mockRejectedValueOnce(new Error('settings: save: disk full'))
      expect(await bindKey('nextFile', key('j'))).toMatch(/disk full/)
      expect(bindings.value.nextFile).toBe(']')
    })
  })
})
