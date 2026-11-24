import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'
import type { UnsentComments } from '@/lib/diffLayout'
import DiffFileView from './DiffFileView.vue'
import * as model from '@/bindings/gitura/internal/model'

const ctx = (n: number) => ({ type: 'context', old_no: n, new_no: n, content: `line ${n}` })
const file = model.PRFileDTO.createFrom({ filename: 'a.ts', status: 'modified', additions: 0, deletions: 0, changes: 0, is_binary: false })
const diff = model.ParsedDiffDTO.createFrom({
  filename: 'a.ts',
  status: 'modified',
  is_binary: false,
  total_additions: 0,
  total_deletions: 0,
  hunks: [{ header: '@@', old_start: 1, old_lines: 3, new_start: 1, new_lines: 3, lines: [ctx(1), ctx(2), ctx(3)] }],
})

describe('DiffFileView', () => {
  it('keeps the comment target fixed while a post is in flight', async () => {
    let resolve!: (ok: boolean) => void
    const postComment = vi.fn(() => new Promise<boolean>((r) => (resolve = r)))
    const wrapper = mount(DiffFileView, {
      props: { file, diff, commentable: true, threads: [], drafts: [], showThreads: true,
      wrapLines: true, postComment, unsentComments: reactive(new Map()) as UnsentComments },
      attachTo: document.body,
    })
    const press = async (n: number) => {
      await wrapper.find(`button[aria-label="Comment on new line ${n}"]`).trigger('mousedown', { button: 0 })
      window.dispatchEvent(new MouseEvent('mouseup'))
      await flushPromises()
    }

    await press(1)
    await wrapper.find('textarea').setValue('feedback')
    await wrapper.find('[aria-label="Add as single comment"]').trigger('click')
    await press(3)

    expect(postComment).toHaveBeenCalledWith(expect.objectContaining({ line: 1, side: 'RIGHT' }), 'immediate')
    resolve(false)
    await flushPromises()
    const selected = wrapper.findAll('button[aria-label^="Comment on new line"]').filter((b) =>
      b.element.parentElement?.className.includes('bg-primary/15'),
    )
    expect(selected.map((b) => b.text())).toEqual(['1'])
    wrapper.unmount()
  })

  it('extends a comment range in its hunk after an earlier hunk is dropped', async () => {
    const hunk = (start: number) => ({ header: '@@', old_start: start, old_lines: 3, new_start: start, new_lines: 3, lines: [ctx(start), ctx(start + 1), ctx(start + 2)] })
    const two = model.ParsedDiffDTO.createFrom({ ...diff, hunks: [hunk(1), hunk(10)] })
    const wrapper = mount(DiffFileView, {
      props: { file, diff: two, commentable: true, threads: [], drafts: [], showThreads: true,
        wrapLines: true, postComment: vi.fn(), unsentComments: reactive(new Map()) as UnsentComments },
      attachTo: document.body,
    })
    const press = async (n: number, shiftKey = false) => {
      await wrapper.find(`button[aria-label="Comment on new line ${n}"]`).trigger('mousedown', { button: 0, shiftKey })
      window.dispatchEvent(new MouseEvent('mouseup'))
      await flushPromises()
    }

    await press(11)
    await wrapper.setProps({ diff: model.ParsedDiffDTO.createFrom({ ...diff, hunks: [hunk(10)] }) })
    await press(12, true)

    const selected = wrapper.findAll('button[aria-label^="Comment on new line"]').filter((b) =>
      b.element.parentElement?.className.includes('bg-primary/15'),
    )
    expect(selected.map((b) => b.text())).toEqual(['11', '12'])
    wrapper.unmount()
  })

  it('extends a restored comment range in its hunk after an earlier hunk is dropped', async () => {
    const hunk = (start: number) => ({ header: '@@', old_start: start, old_lines: 3, new_start: start, new_lines: 3, lines: [ctx(start), ctx(start + 1), ctx(start + 2)] })
    const props = { file, commentable: true, threads: [], drafts: [], showThreads: true, wrapLines: true,
      postComment: vi.fn(), unsentComments: reactive(new Map()) as UnsentComments }
    const first = mount(DiffFileView, { props: { ...props, diff: model.ParsedDiffDTO.createFrom({ ...diff, hunks: [hunk(1), hunk(10)] }) }, attachTo: document.body })
    await first.find('button[aria-label="Comment on new line 11"]').trigger('mousedown', { button: 0 })
    window.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()
    await first.find('textarea').setValue('half a thought')
    first.unmount()

    const second = mount(DiffFileView, { props: { ...props, diff: model.ParsedDiffDTO.createFrom({ ...diff, hunks: [hunk(10)] }) }, attachTo: document.body })
    await second.find('button[aria-label="Comment on new line 12"]').trigger('mousedown', { button: 0, shiftKey: true })
    window.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()

    const selected = second.findAll('button[aria-label^="Comment on new line"]').filter((b) =>
      b.element.parentElement?.className.includes('bg-primary/15'),
    )
    expect(selected.map((b) => b.text())).toEqual(['11', '12'])
    second.unmount()
  })

  it('keeps a half-written comment when the view is left and comes back', async () => {
    const unsentComments: UnsentComments = reactive(new Map())
    const props = { file, diff, commentable: true, threads: [], drafts: [], showThreads: true, wrapLines: true, postComment: vi.fn(), unsentComments }
    const first = mount(DiffFileView, { props, attachTo: document.body })
    await first.find('button[aria-label="Comment on new line 2"]').trigger('mousedown', { button: 0 })
    window.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()
    await first.find('textarea').setValue('half a thought')
    first.unmount()

    const second = mount(DiffFileView, { props, attachTo: document.body })
    await flushPromises()
    expect((second.find('textarea').element as HTMLTextAreaElement).value).toBe('half a thought')
    second.unmount()
  })

  it('does not restore a comment that was still sending when the view was left', async () => {
    let resolve!: (ok: boolean) => void
    const unsentComments: UnsentComments = reactive(new Map())
    const postComment = vi.fn(() => new Promise<boolean>((r) => (resolve = r)))
    const props = { file, diff, commentable: true, threads: [], drafts: [], showThreads: true, wrapLines: true, postComment, unsentComments }
    const first = mount(DiffFileView, { props, attachTo: document.body })
    await first.find('button[aria-label="Comment on new line 2"]').trigger('mousedown', { button: 0 })
    window.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()
    await first.find('textarea').setValue('sent once')
    await first.find('[aria-label="Add as single comment"]').trigger('click')
    first.unmount()
    resolve(true)
    await flushPromises()

    expect(unsentComments.size).toBe(0)
    const second = mount(DiffFileView, { props, attachTo: document.body })
    await flushPromises()
    expect(second.find('textarea').exists()).toBe(false)
    second.unmount()
  })

  it('brings a comment back when it fails to send after the view was left', async () => {
    let resolve!: (ok: boolean) => void
    const unsentComments: UnsentComments = reactive(new Map())
    const postComment = vi.fn(() => new Promise<boolean>((r) => (resolve = r)))
    const props = { file, diff, commentable: true, threads: [], drafts: [], showThreads: true, wrapLines: true, postComment, unsentComments }
    const first = mount(DiffFileView, { props, attachTo: document.body })
    await first.find('button[aria-label="Comment on new line 2"]').trigger('mousedown', { button: 0 })
    window.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()
    await first.find('textarea').setValue('try again')
    await first.find('[aria-label="Add as single comment"]').trigger('click')
    first.unmount()
    resolve(false)
    await flushPromises()

    const second = mount(DiffFileView, { props, attachTo: document.body })
    await flushPromises()
    expect((second.find('textarea').element as HTMLTextAreaElement).value).toBe('try again')
    second.unmount()
  })

  it('keeps a failed comment when the file was reopened before the send failed', async () => {
    let resolve!: (ok: boolean) => void
    const unsentComments: UnsentComments = reactive(new Map())
    const postComment = vi.fn(() => new Promise<boolean>((r) => (resolve = r)))
    const props = { file, diff, commentable: true, threads: [], drafts: [], showThreads: true, wrapLines: true, postComment, unsentComments }
    const first = mount(DiffFileView, { props, attachTo: document.body })
    await first.find('button[aria-label="Comment on new line 2"]').trigger('mousedown', { button: 0 })
    window.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()
    await first.find('textarea').setValue('retry me')
    await first.find('[aria-label="Add as single comment"]').trigger('click')
    first.unmount()
    const second = mount(DiffFileView, { props, attachTo: document.body })
    await flushPromises()
    const locked = second.find('textarea').element as HTMLTextAreaElement
    expect(locked.value).toBe('retry me')
    expect(locked.disabled).toBe(true)
    resolve(false)
    await flushPromises()
    expect(locked.disabled).toBe(false)
    second.unmount()

    const third = mount(DiffFileView, { props, attachTo: document.body })
    await flushPromises()
    expect((third.find('textarea').element as HTMLTextAreaElement).value).toBe('retry me')
    third.unmount()
  })

  it('locks a restored comment that is sending in every view of the file', async () => {
    let resolve!: (ok: boolean) => void
    const unsentComments: UnsentComments = reactive(new Map())
    unsentComments.set(file.filename, { target: { side: 'RIGHT', hunk: 0, line: 2 }, body: 'saved earlier' })
    const postComment = vi.fn(() => new Promise<boolean>((r) => (resolve = r)))
    const props = { file, diff, commentable: true, threads: [], drafts: [], showThreads: true, wrapLines: true, postComment, unsentComments }
    const first = mount(DiffFileView, { props, attachTo: document.body })
    await flushPromises()
    await first.find('[aria-label="Add as single comment"]').trigger('click')
    first.unmount()

    const second = mount(DiffFileView, { props, attachTo: document.body })
    await flushPromises()
    expect(second.find('[aria-label="Add as single comment"]').attributes('disabled')).toBeDefined()
    expect(second.find('[aria-label="Add inline comment"]').attributes('aria-busy')).toBe('true')
    expect(second.text()).toContain('Sending…')
    await second.find('[aria-label="Add as single comment"]').trigger('click')
    expect(postComment).toHaveBeenCalledTimes(1)
    await second.find('button[aria-label="Comment on new line 3"]').trigger('mousedown', { button: 0 })
    window.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()
    expect((second.find('textarea').element as HTMLTextAreaElement).value).toBe('saved earlier')

    resolve(true)
    await flushPromises()
    expect(unsentComments.size).toBe(0)
    expect(second.find('textarea').exists()).toBe(false)
    second.unmount()
  })

  it('animates in only the inline rows added after the file is shown', async () => {
    const unsentComments: UnsentComments = reactive(new Map())
    unsentComments.set(file.filename, { target: { side: 'RIGHT', hunk: 0, line: 2 }, body: 'saved earlier' })
    const props = { file, diff, commentable: true, threads: [], drafts: [], showThreads: true, wrapLines: true, postComment: vi.fn(), unsentComments }
    const wrapper = mount(DiffFileView, { props, attachTo: document.body })
    await flushPromises()
    const formRow = () => wrapper.findAll('transition-stub').find((t) => t.find('textarea').exists())
    expect(formRow()?.attributes('appear')).toBe('false')

    await wrapper.find('button[aria-label="Comment on new line 3"]').trigger('mousedown', { button: 0 })
    window.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()
    expect(formRow()?.attributes('appear')).toBe('true')
    wrapper.unmount()
  })

  it('does not re-render the diff while typing a comment', async () => {
    let paneUpdates = 0
    const props = { file, diff, commentable: true, threads: [], drafts: [], showThreads: true, wrapLines: true, postComment: vi.fn(), unsentComments: reactive(new Map()) as UnsentComments }
    const wrapper = mount(DiffFileView, {
      props,
      attachTo: document.body,
      global: { mixins: [{ updated(this: { $options: { __name?: string } }) { if (this.$options.__name === 'DiffPane') paneUpdates++ } }] },
    })
    await wrapper.find('button[aria-label="Comment on new line 2"]').trigger('mousedown', { button: 0 })
    window.dispatchEvent(new MouseEvent('mouseup'))
    await flushPromises()
    paneUpdates = 0

    await wrapper.find('textarea').setValue('a')
    await wrapper.find('textarea').setValue('ab')

    expect(paneUpdates).toBe(0)
    wrapper.unmount()
  })

  describe('split scrolling', () => {
    // jsdom lays nothing out: each pane gets a scroll range by hand.
    async function panes(base: number, head: number, shown = diff) {
      const wrapper = mount(DiffFileView, {
        props: { file, diff: shown, commentable: true, threads: [], drafts: [], showThreads: true, wrapLines: true, postComment: vi.fn(), unsentComments: reactive(new Map()) as UnsentComments },
        attachTo: document.body,
      })
      // A new layout gets measured, which sets up the points the panes sync through.
      await wrapper.setProps({ diff: model.ParsedDiffDTO.createFrom({ ...shown, hunks: [...shown.hunks] }) })
      await flushPromises()
      const [left, right] = wrapper.findAll('.\\@container').map((w) => w.element)
      if (!left || !right) throw new Error('no split panes')
      for (const [el, height] of [[left, base], [right, head]] as const) {
        Object.defineProperty(el, 'clientHeight', { value: 400 })
        Object.defineProperty(el, 'scrollHeight', { value: height })
        Object.defineProperty(el, 'scrollTop', { value: 0, writable: true })
      }
      return { wrapper, left, right }
    }

    it('wheels the head when the base under the pointer has nowhere to go', async () => {
      const { wrapper, left, right } = await panes(400, 2000)
      left.dispatchEvent(new WheelEvent('wheel', { deltaY: 120 }))
      expect(right.scrollTop).toBe(120)
      wrapper.unmount()
    })

    it('jumps through changes in the base when the head fits, and to the last one on request', async () => {
      // Old lines 1-20 kept, 21 deleted, 22-40 kept, 41 deleted: the base's rows 20 and 40.
      const kept = (from: number, to: number, shift: number) =>
        Array.from({ length: to - from + 1 }, (_, i) => ({ type: 'context', old_no: from + i, new_no: from + i - shift, content: `line ${from + i}` }))
      const gone = (n: number) => ({ type: 'delete', old_no: n, new_no: 0, content: `gone ${n}` })
      const lines = [...kept(1, 20, 0), gone(21), ...kept(22, 40, 1), gone(41)]
      const deletions = model.ParsedDiffDTO.createFrom({ ...diff, hunks: [{ header: '@@', old_start: 1, old_lines: 41, new_start: 1, new_lines: 39, lines }] })
      const { wrapper, left } = await panes(2000, 400, deletions)
      const rows = left.firstElementChild?.children ?? []
      Array.from(rows).forEach((row, i) => Object.defineProperty(row, 'offsetTop', { value: i * 20 }))

      // A quarter of the 400px pane above each change.
      expect(wrapper.vm.jumpChange(1)).toBe(true)
      expect(left.scrollTop).toBe(300)
      expect(wrapper.vm.jumpChange(1)).toBe(true)
      expect(left.scrollTop).toBe(700)
      expect(wrapper.vm.jumpChange(1)).toBe(false)
      left.scrollTop = 0
      expect(wrapper.vm.jumpChange(1, true)).toBe(true)
      expect(left.scrollTop).toBe(700)
      wrapper.unmount()
    })

    it('follows the base to its end when keys scrolled it from outside', async () => {
      const { wrapper, left, right } = await panes(1000, 2000)
      left.scrollTop = 600
      left.dispatchEvent(new Event('scroll'))
      expect(right.scrollTop).toBe(1600)
      wrapper.unmount()
    })
  })

  it('focuses the head pane so keys scroll the diff', () => {
    const wrapper = mount(DiffFileView, {
      props: { file, diff, commentable: true, threads: [], drafts: [], showThreads: true,
        wrapLines: true, postComment: vi.fn(), unsentComments: reactive(new Map()) as UnsentComments },
      attachTo: document.body,
    })
    wrapper.vm.focus()
    expect(document.activeElement).toBe(wrapper.find('[data-vim-scroll]').element)
    wrapper.unmount()
  })
})
