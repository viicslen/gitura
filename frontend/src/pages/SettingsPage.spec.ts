import { describe, it, expect, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SettingsPage from './SettingsPage.vue'

const saves = vi.hoisted(() => [] as Array<() => void>)
vi.mock('@/bindings/gitura/internal/app/app', async (importOriginal) => {
  const real = await importOriginal<Record<string, unknown>>()
  const stubs = Object.fromEntries(Object.keys(real).map((name) => [name, vi.fn(async () => null)]))
  // Each save waits until the test answers it, so it can answer after the recording restarted.
  return { ...stubs, SetKeybindings: vi.fn(() => new Promise<void>((resolve) => saves.push(resolve))) }
})

describe('SettingsPage keybindings', () => {
  it('keeps a restarted recording when the earlier save answers late', async () => {
    const wrapper = mount(SettingsPage, { attachTo: document.body, global: { stubs: { DialogTitle: true } } })
    await flushPromises()
    await wrapper.get('[role="tab"][id$="keybindings"]').trigger('mousedown', { button: 0 })
    await flushPromises()
    const button = () => wrapper.get('[aria-label="Change shortcut for Previous file"]')

    await button().trigger('click')
    await button().trigger('keydown', { key: 'x' })
    await button().trigger('click')
    expect(button().classes()).toContain('wow-glow')

    saves.shift()?.()
    await flushPromises()
    expect(button().classes()).toContain('wow-glow')
    wrapper.unmount()
  })
})
