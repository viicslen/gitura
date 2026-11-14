import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import CountUp from './CountUp.vue'
import { confetti, reducedMotion, viewTransition } from './wow'

describe('showpiece motion without a motion-capable window', () => {
  it('counts as reduced motion when the window cannot say', () => {
    expect(reducedMotion()).toBe(true)
  })

  it('still makes the change a view transition would have wrapped', async () => {
    const change = vi.fn()
    await viewTransition(change)
    expect(change).toHaveBeenCalledOnce()
  })

  it('shows the final number at once and adds no confetti canvas', () => {
    expect(mount(CountUp, { props: { value: 42 } }).text()).toBe('42')
    confetti()
    expect(document.querySelector('canvas')).toBeNull()
  })
})
