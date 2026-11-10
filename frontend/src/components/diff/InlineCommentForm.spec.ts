import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import InlineCommentForm from './InlineCommentForm.vue'

const target = { path: 'a.ts', line: 3, side: 'RIGHT' }
const text = (w: ReturnType<typeof mount>) => (w.find('textarea').element as HTMLTextAreaElement).value

describe('InlineCommentForm', () => {
  it('keeps the typed comment when posting fails and clears it on success', async () => {
    const post = vi.fn().mockResolvedValueOnce(false).mockResolvedValueOnce(true)
    const wrapper = mount(InlineCommentForm, { props: { target, post } })
    await wrapper.find('textarea').setValue('important feedback')

    await wrapper.find('[aria-label="Add as single comment"]').trigger('click')
    await flushPromises()
    expect(post).toHaveBeenLastCalledWith('important feedback', 'immediate')
    expect(text(wrapper)).toBe('important feedback')

    await wrapper.find('[aria-label="Add as single comment"]').trigger('click')
    await flushPromises()
    expect(text(wrapper)).toBe('')
  })

  it('locks the form while a post is in flight', async () => {
    let resolve!: (ok: boolean) => void
    const post = vi.fn(() => new Promise<boolean>((r) => (resolve = r)))
    const wrapper = mount(InlineCommentForm, { props: { target, post } })
    await wrapper.find('textarea').setValue('first version')

    await wrapper.find('[aria-label="Add as single comment"]').trigger('click')
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).disabled).toBe(true)
    expect(wrapper.find('[aria-label="Cancel comment"]').attributes('disabled')).toBeDefined()

    resolve(false)
    await flushPromises()
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).disabled).toBe(false)
    expect(text(wrapper)).toBe('first version')
  })

  it('ignores Escape while the comment has text', async () => {
    const wrapper = mount(InlineCommentForm, { props: { target, post: vi.fn() } })
    await wrapper.find('textarea').setValue('keep me')
    await wrapper.find('textarea').trigger('keydown', { key: 'Escape' })
    expect(wrapper.emitted('cancel')).toBeUndefined()

    await wrapper.find('textarea').setValue('')
    await wrapper.find('textarea').trigger('keydown', { key: 'Escape' })
    expect(wrapper.emitted('cancel')).toHaveLength(1)
  })
})
