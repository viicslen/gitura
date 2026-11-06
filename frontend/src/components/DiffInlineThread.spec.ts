import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DiffInlineThread from './DiffInlineThread.vue'
import { model } from '@/wailsjs/go/models'

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
})
