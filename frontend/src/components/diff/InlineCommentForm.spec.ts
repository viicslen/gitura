import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const run = vi.fn(async (_names: string[], input: string) => [{ input }])
vi.mock('@/bindings/gitura/internal/app/app', () => ({
  RunCommands: (...a: [string[], string]) => run(...a),
  SetToggles: vi.fn(async () => {}),
}))

import InlineCommentForm from './InlineCommentForm.vue'
import { commands } from '@/composables/useCommands'
import { setToggle } from '@/composables/useToggles'
import type * as model from '@/bindings/gitura/internal/model'

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

  describe('Run', () => {
    async function runTyped(remove: boolean) {
      commands.value = [{ name: 'fix', command: 'echo' } as model.CommandDTO]
      await setToggle('remove_draft_after_run', remove)
      const post = vi.fn()
      const wrapper = mount(InlineCommentForm, { props: { target, post } })
      await wrapper.find('textarea').setValue('  try this  ')
      await wrapper.find('button[aria-busy]').trigger('click')
      await flushPromises()
      expect(run).toHaveBeenLastCalledWith(['fix'], 'try this', expect.anything())
      expect(post).not.toHaveBeenCalled()
      return wrapper
    }

    it('runs the typed comment without posting it, and keeps the text by default', async () => {
      const wrapper = await runTyped(false)
      expect(text(wrapper)).toBe('  try this  ')
      expect(wrapper.emitted('cancel')).toBeUndefined()
    })

    it('discards the comment after the run when the setting is on', async () => {
      const wrapper = await runTyped(true)
      expect(text(wrapper)).toBe('')
      expect(wrapper.emitted('cancel')).toHaveLength(1)
    })
  })
})
