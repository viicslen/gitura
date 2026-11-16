import { describe, it, expect } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DiffFileSidebar from './DiffFileSidebar.vue'
import type * as model from '@/bindings/gitura/internal/model'

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

describe('DiffFileSidebar folder menu', () => {
  it('marks every file in a right-clicked folder as viewed', async () => {
    const wrapper = mount(DiffFileSidebar, { props: { files, selectedPath: null }, attachTo: document.body })
    await wrapper.find('[role="treeitem"][aria-expanded]').trigger('contextmenu')
    await flushPromises()
    const item = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find((el) => el.textContent?.includes('Mark folder as viewed'))
    item!.click()
    await flushPromises()
    expect(wrapper.emitted('set-viewed')).toEqual([[['src/a.ts', 'src/b.ts'], true]])
    wrapper.unmount()
  })
})

describe('DiffFileSidebar cascade', () => {
  it('springs in the shown rows in order, then only the rows of a folder being opened', async () => {
    const wrapper = mount(DiffFileSidebar, { props: { files, selectedPath: null, active: true }, attachTo: document.body })
    const rising = () => wrapper.findAll('.wow-rise').map((row) => (row.element as HTMLElement).style.getPropertyValue('--i'))
    expect(rising()).toEqual(['0', '1', '2', '3'])

    const folder = wrapper.find('[role="treeitem"][aria-expanded]')
    await folder.trigger('click')
    expect(rising()).toEqual(['0', '1', '2', '3'])
    await folder.trigger('click')
    expect(rising()).toEqual(['0', '1'])
    wrapper.unmount()
  })
})
