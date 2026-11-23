import { describe, it, expect } from 'vitest'
import { scrollToNext } from './scrollJump'

describe('scrollToNext', () => {
  const pane = (scrollTop = 0) => ({ scrollTop, scrollHeight: 1000, clientHeight: 400 })

  it('steps through the tops, skipping one already in place', () => {
    const el = pane()
    const tops = [0, 300, 500, 900]
    expect(scrollToNext(el, tops, 1)).toBe(true)
    expect(el.scrollTop).toBe(300)
    expect(scrollToNext(el, tops, 1)).toBe(true)
    expect(el.scrollTop).toBe(500)
    expect(scrollToNext(el, tops, -1)).toBe(true)
    expect(el.scrollTop).toBe(300)
  })

  it('reports the end once the rest can’t be scrolled to', () => {
    const el = pane(600)
    expect(scrollToNext(el, [100, 700, 900], 1)).toBe(false)
    expect(el.scrollTop).toBe(600)
    expect(scrollToNext(pane(0), [100, 700], -1, 50)).toBe(false)
  })

  it('leaves the margin above the target', () => {
    const el = pane()
    scrollToNext(el, [300], 1, 100)
    expect(el.scrollTop).toBe(200)
  })
})
