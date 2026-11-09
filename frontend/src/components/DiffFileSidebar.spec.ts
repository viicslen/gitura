import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DiffFileSidebar from './DiffFileSidebar.vue'
import type { model } from '@/wailsjs/go/models'

const files = ['src/a.ts', 'src/b.ts', 'README.md'].map(
  (filename) => ({ filename, status: 'modified', additions: 1, deletions: 0, viewed: false }) as model.PRFileDTO,
)

describe('DiffFileSidebar tree keyboard', () => {
  it('has one tab stop and moves with up/down, leaving left/right to the page', async () => {
    const wrapper = mount(DiffFileSidebar, { props: { files, selectedPath: 'src/b.ts' }, attachTo: document.body })
    const items = () => wrapper.findAll('[role="treeitem"]')
    expect(items().map((i) => i.attributes('tabindex'))).toEqual(['-1', '-1', '0', '-1'])

    ;(items()[2].element as HTMLElement).focus()
    await items()[2].trigger('keydown', { key: 'ArrowUp' })
    expect(document.activeElement).toBe(items()[1].element)

    await items()[0].trigger('keydown', { key: 'ArrowLeft' })
    expect(items()).toHaveLength(4)
    wrapper.unmount()
  })
})
