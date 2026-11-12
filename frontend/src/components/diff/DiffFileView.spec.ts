import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'
import type { UnsentComments } from '@/lib/diffLayout'
import DiffFileView from './DiffFileView.vue'
import { model } from '@/wailsjs/go/models'

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
})
