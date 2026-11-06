import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
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
      props: { file, diff, commentable: true, threads: [], drafts: [], showThreads: true, postComment },
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
})
