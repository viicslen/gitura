import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const replyToComment = vi.fn()
vi.mock('../wailsjs/go/main/App', () => ({ ReplyToComment: (...a: unknown[]) => replyToComment(...a) }))

import ReplyComposer from './ReplyComposer.vue'

const textarea = (w: ReturnType<typeof mount>) => w.find('textarea').element as HTMLTextAreaElement

describe('ReplyComposer', () => {
  beforeEach(() => replyToComment.mockReset())

  it('keeps an unsent reply per thread across unmount', async () => {
    const first = mount(ReplyComposer, { props: { threadRootId: 41 } })
    await first.find('textarea').setValue('half-written')
    first.unmount()

    expect(textarea(mount(ReplyComposer, { props: { threadRootId: 41 } })).value).toBe('half-written')
    expect(textarea(mount(ReplyComposer, { props: { threadRootId: 42 } })).value).toBe('')
  })

  it('blocks a second send of the same thread from another composer', async () => {
    let resolve!: (c: object) => void
    replyToComment.mockReturnValue(new Promise((r) => (resolve = r)))
    const conversation = mount(ReplyComposer, { props: { threadRootId: 7 } })
    const files = mount(ReplyComposer, { props: { threadRootId: 7 } })

    await conversation.find('textarea').setValue('first')
    await conversation.find('button').trigger('click')
    await flushPromises()
    expect(textarea(files).disabled).toBe(true)
    await files.find('button').trigger('click')
    expect(replyToComment).toHaveBeenCalledTimes(1)

    resolve({ id: 1, in_reply_to_id: 7 })
    await flushPromises()
    expect(textarea(files).disabled).toBe(false)
    expect(textarea(files).value).toBe('')
  })
})
