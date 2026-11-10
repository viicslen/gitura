import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { GetKeybindings, SetKeybindings } from '@/wailsjs/go/app/App'
import { bindKey, useKeybindings } from './useKeybindings'

vi.mock('@/wailsjs/go/app/App', () => ({
  GetKeybindings: vi.fn(async () => ({ nextFile: 'n', bogus: 'x' })),
  SetKeybindings: vi.fn(async () => {}),
}))

const key = (k: string, init: KeyboardEventInit = {}) => new KeyboardEvent('keydown', { key: k, ...init })

describe('useKeybindings', () => {
  const { bindings, reset } = useKeybindings()

  it('merges saved bindings over defaults, dropping unknown actions', async () => {
    await flushPromises()
    expect(GetKeybindings).toHaveBeenCalledOnce()
    expect(bindings.value).toEqual({ prevFile: '[', nextFile: 'n', prevThread: 'ArrowLeft', nextThread: 'ArrowRight' })
  })

  describe('bindKey', () => {
    beforeEach(async () => { await reset() })

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
