import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DiffInlineThread from './DiffInlineThread.vue'
import { renderGitHubMarkdown } from '@/lib/markdown'

vi.mock('@/lib/markdown', async (importOriginal) => {
  const real = await importOriginal<typeof import('@/lib/markdown')>()
  return { ...real, renderGitHubMarkdown: vi.fn(real.renderGitHubMarkdown) }
})
import * as model from '@/bindings/gitura/internal/model'

const thread = model.CommentThreadDTO.createFrom({
  root_id: 1,
  review_id: 0,
  node_id: 'T1',
  resolved: false,
  outdated: false,
  path: 'a.ts',
  line: 3,
  side: 'RIGHT',
  comments: [{ id: 1, in_reply_to_id: 0, review_id: 0, body: 'hi', author_login: 'dev', author_avatar: '', diff_hunk: '', created_at: '', is_suggestion: false }],
})

describe('DiffInlineThread', () => {
  it('keeps an unsent reply across collapse and expand', async () => {
    const wrapper = mount(DiffInlineThread, { props: { thread, language: 'typescript' } })
    await wrapper.find('textarea').setValue('draft reply')

    const header = wrapper.find('button[aria-expanded]')
    await header.trigger('click')
    expect(header.attributes('aria-expanded')).toBe('false')
    await header.trigger('click')

    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('draft reply')
  })

  it('does not re-render comment markdown when the thread is collapsed', async () => {
    const wrapper = mount(DiffInlineThread, { props: { thread, language: 'typescript' } })
    vi.mocked(renderGitHubMarkdown).mockClear()

    await wrapper.find('button[aria-expanded]').trigger('click')

    expect(renderGitHubMarkdown).not.toHaveBeenCalled()
  })
})
