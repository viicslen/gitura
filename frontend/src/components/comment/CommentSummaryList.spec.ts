import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import * as model from '@/bindings/gitura/internal/model'
import CommentSummaryList from './CommentSummaryList.vue'

const thread = (id: number) =>
  model.CommentThreadDTO.createFrom({
    root_id: id,
    review_id: 0,
    node_id: `T${id}`,
    resolved: false,
    outdated: false,
    path: 'a.ts',
    line: id,
    side: 'RIGHT',
    comments: [{ id, in_reply_to_id: 0, review_id: 0, body: `thread ${id}`, author_login: 'dev', author_avatar: '', diff_hunk: '', created_at: '', is_suggestion: false }],
  })

describe('CommentSummaryList', () => {
  it('selects the first or last thread on Home and End', async () => {
    const threads = [thread(1), thread(2), thread(3)]
    const wrapper = mount(CommentSummaryList, {
      props: { threads, total: 3, currentIndex: 1, showResolved: false, commands: [] },
      attachTo: document.body,
    })
    const row = wrapper.get('#thread-item-1')
    await row.trigger('keydown', { key: 'End' })
    await row.trigger('keydown', { key: 'Home' })
    expect(wrapper.emitted('select')).toEqual([[2], [0]])
    expect(document.activeElement?.id).toBe('thread-item-0')
    wrapper.unmount()
  })
})
