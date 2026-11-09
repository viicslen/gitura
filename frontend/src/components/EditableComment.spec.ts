import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive, ref } from 'vue'

const update = vi.fn()
const del = vi.fn()
vi.mock('@/wailsjs/go/main/App', () => ({
  UpdateBodyIfUnchanged: (...a: unknown[]) => update(...a),
  DeleteComment: (...a: unknown[]) => del(...a),
}))

import EditableComment from './EditableComment.vue'

const mountAs = (
  viewer: string,
  target: { body: string; author_login: string },
  id = 1,
  kind: 'review_comment' | 'issue_comment' | 'review' = 'review_comment',
  commentDeleted = vi.fn(),
) =>
  mount(EditableComment, {
    props: { kind, id, target },
    slots: { default: '<p>view</p>' },
    global: { provide: { viewerLogin: ref(viewer), commentDeleted } },
    attachTo: document.body,
  })

describe('EditableComment', () => {
  it('offers editing only on the viewer’s own comments', () => {
    expect(mountAs('dev', { body: 'x', author_login: 'other' }).find('[aria-label="Edit comment"]').exists()).toBe(false)
    expect(mountAs('dev', { body: 'x', author_login: 'dev' }).find('button[aria-label="Edit comment"]').exists()).toBe(true)
  })

  it('saves against the body the edit started from and writes the result back', async () => {
    update.mockImplementation(async (_k, _id, _expected, next: string) => next)
    const target = reactive({ body: 'old', author_login: 'dev' })
    const w = mountAs('dev', target, 2)

    await w.find('button[aria-label="Edit comment"]').trigger('click')
    await w.find('textarea').setValue('new')
    await w.findAll('button').find((b) => b.text() === 'Save')?.trigger('click')
    await flushPromises()

    expect(update).toHaveBeenCalledWith('review_comment', 2, 'old', 'new')
    expect(target.body).toBe('new')
    expect(w.find('textarea').exists()).toBe(false)
  })

  it('keeps the editor open with the error when saving fails', async () => {
    update.mockRejectedValue('conflict: this comment changed on GitHub')
    const w = mountAs('dev', { body: 'old', author_login: 'dev' }, 3)

    await w.find('button[aria-label="Edit comment"]').trigger('click')
    await w.find('textarea').setValue('new')
    await w.findAll('button').find((b) => b.text() === 'Save')?.trigger('click')
    await flushPromises()

    expect((w.find('textarea').element as HTMLTextAreaElement).value).toBe('new')
    expect(w.text()).toContain('conflict')
  })

  it('deletes after confirmation and tells the page to reload', async () => {
    del.mockResolvedValue(undefined)
    const onDeleted = vi.fn()
    const w = mountAs('dev', { body: 'x', author_login: 'dev' }, 4, 'issue_comment', onDeleted)

    await w.find('button[aria-label="Delete comment"]').trigger('click')
    await flushPromises()
    const confirm = Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Delete')
    confirm?.click()
    await flushPromises()

    expect(del).toHaveBeenCalledWith('issue_comment', 4)
    expect(onDeleted).toHaveBeenCalledOnce()
    w.unmount()
  })

  it('routes draft edits and deletes through the given handlers, not GitHub', async () => {
    update.mockClear()
    del.mockClear()
    const save = vi.fn(async (_base: string, next: string) => next)
    const remove = vi.fn(async () => {})
    const w = mount(EditableComment, {
      props: { kind: 'draft', id: 0, target: { body: 'old' }, save, remove },
      slots: { default: '<p>view</p>' },
      attachTo: document.body,
    })

    await w.find('button[aria-label="Edit comment"]').trigger('click')
    await w.find('textarea').setValue('new')
    await w.findAll('button').find((b) => b.text() === 'Save')?.trigger('click')
    await flushPromises()
    expect(save).toHaveBeenCalledWith('old', 'new')

    await w.find('button[aria-label="Delete comment"]').trigger('click')
    await flushPromises()
    Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Delete')?.click()
    await flushPromises()
    expect(remove).toHaveBeenCalledOnce()
    expect(update).not.toHaveBeenCalled()
    expect(del).not.toHaveBeenCalled()
    w.unmount()
  })

  it('offers no delete on a submitted review', () => {
    const w = mountAs('dev', { body: 'x', author_login: 'dev' }, 5, 'review')
    expect(w.find('button[aria-label="Delete comment"]').exists()).toBe(false)
    w.unmount()
  })
})
