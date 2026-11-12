import { describe, it, expect, vi } from 'vitest'
import { onTaskChange, taskToggler } from './taskList'

vi.mock('vue-sonner', () => ({ toast: { error: vi.fn() } }))
const UpdateBodyIfUnchanged = vi.fn()
vi.mock('@/bindings/gitura/internal/app/app', () => ({ UpdateBodyIfUnchanged: (...a: unknown[]) => UpdateBodyIfUnchanged(...a) }))

function box(index: string, checked: boolean): HTMLInputElement {
  const el = document.createElement('input')
  el.type = 'checkbox'
  el.checked = checked
  el.dataset.taskIndex = index
  return el
}

function changeOn(el: HTMLInputElement): Event {
  const event = new Event('change')
  Object.defineProperty(event, 'target', { value: el })
  return event
}

describe('onTaskChange', () => {
  it('passes the offset index and new state to the toggle', async () => {
    const toggle = vi.fn().mockResolvedValue(undefined)
    const el = box('1', true)
    await onTaskChange(changeOn(el), toggle, 2)
    expect(toggle).toHaveBeenCalledWith(3, true)
    expect(el.disabled).toBe(false)
  })

  it('reverts the checkbox when GitHub rejects the edit', async () => {
    const el = box('0', true)
    await onTaskChange(changeOn(el), vi.fn().mockRejectedValue(new Error('403')))
    expect(el.checked).toBe(false)
  })
})

describe('taskToggler', () => {
  it('runs quick clicks on one body one after another, each on the latest body', async () => {
    UpdateBodyIfUnchanged.mockImplementation(async (_k, _id, _expected: string, next: string) => next)
    const target = { body: '- [ ] a\n- [ ] b' }
    const toggle = taskToggler('issue_comment', 7, target)
    await Promise.all([toggle(0, true), toggle(1, true)])
    expect(UpdateBodyIfUnchanged.mock.calls.map((c) => [c[2], c[3]])).toEqual([
      ['- [ ] a\n- [ ] b', '- [x] a\n- [ ] b'],
      ['- [x] a\n- [ ] b', '- [x] a\n- [x] b'],
    ])
    expect(target.body).toBe('- [x] a\n- [x] b')
  })
})
