import { describe, it, expect } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DiffFileSidebar from './DiffFileSidebar.vue'
import type * as model from '@/bindings/gitura/internal/model'
import { fileKey } from '@/lib/fileTree'

const files = ['src/a.ts', 'src/b.ts', 'README.md'].map(
  (filename) => ({ filename, status: 'modified', additions: 1, deletions: 0, viewed: false }) as model.PRFileDTO,
)

describe('DiffFileSidebar tree keyboard', () => {
  it('has one tab stop and moves with up/down, leaving left/right to the page', async () => {
    const wrapper = mount(DiffFileSidebar, { props: { files, selectedKey: 'src/b.ts' }, attachTo: document.body })
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
    const wrapper = mount(DiffFileSidebar, { props: { files, selectedKey: null }, attachTo: document.body })
    await wrapper.find('[role="treeitem"][aria-expanded]').trigger('contextmenu')
    await flushPromises()
    const item = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find((el) => el.textContent?.includes('Mark folder as viewed'))
    item!.click()
    await flushPromises()
    expect(wrapper.emitted('set-viewed')).toEqual([[['src/a.ts', 'src/b.ts'], true]])
    wrapper.unmount()
  })

  it("keeps a main-repository folder's action out of a submodule beneath it", async () => {
    const nested = [
      { filename: 'third_party/zlib.h', status: 'modified', additions: 1, deletions: 0, viewed: false },
      { filename: 'third_party/msgpack', status: 'submodule', submodule: 'third_party/msgpack', additions: 1, deletions: 1, viewed: false },
      { filename: 'third_party/msgpack/pack.c', repo: 'third_party/msgpack', submodule: 'third_party/msgpack', status: 'modified', additions: 2, deletions: 0, viewed: false },
      { filename: 'third_party/msgpack/src/unpack.c', repo: 'third_party/msgpack', submodule: 'third_party/msgpack', status: 'modified', additions: 2, deletions: 0, viewed: false },
    ] as model.PRFileDTO[]
    const wrapper = mount(DiffFileSidebar, { props: { files: nested, selectedKey: null }, attachTo: document.body })
    const markFolder = async (title: string) => {
      await wrapper.find(`[role="treeitem"][aria-expanded][title="${title}"]`).trigger('contextmenu')
      await flushPromises()
      Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find((el) => el.textContent?.includes('Mark folder as viewed'))?.click()
      await flushPromises()
    }
    await markFolder('third_party')
    await markFolder('third_party/msgpack/src')
    expect(wrapper.emitted('set-viewed')).toEqual([[['third_party/zlib.h'], true], [[fileKey(nested[3])], true]])
    wrapper.unmount()
  })
})

describe('DiffFileSidebar cascade', () => {
  it('springs in the shown rows in order, then only the rows of a folder being opened', async () => {
    const wrapper = mount(DiffFileSidebar, { props: { files, selectedKey: null, active: true }, attachTo: document.body })
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

describe('DiffFileSidebar submodules', () => {
  const sub = [
    ...files,
    { filename: 'lib', status: 'submodule', submodule: 'lib', submodule_range: 'a1..b2', submodule_backwards: true, submodule_pr: 'o/lib#3', submodule_pr_url: 'https://github.com/o/lib/pull/3', additions: 1, deletions: 1, viewed: false },
    { filename: 'lib/x.c', repo: 'lib', submodule: 'lib', status: 'modified', additions: 4, deletions: 2, viewed: false },
  ] as model.PRFileDTO[]

  it('groups a submodule under its own header after the main files, pointer row first', () => {
    const wrapper = mount(DiffFileSidebar, { props: { files: sub, selectedKey: null } })
    const header = wrapper.findAll('[role="treeitem"]')[4]
    expect(header.text()).toContain('lib')
    expect(header.text()).toContain('1 file')
    expect(header.text()).toContain('+4')
    expect(header.text()).toContain('Moved backwards')
    expect(header.text()).toContain('Has an open PR: o/lib#3')
    expect(wrapper.findAll('[role="treeitem"]').slice(5).map((i) => i.attributes('title'))).toEqual(['lib', 'lib/x.c'])
  })

  it('flags diverged commits and a failed open-PR lookup in the header', () => {
    const odd = sub.map((f) => (f.filename === 'lib' ? { ...f, submodule_backwards: false, submodule_diverged: true, submodule_pr: undefined, submodule_pr_unknown: true } : f))
    const header = mount(DiffFileSidebar, { props: { files: odd, selectedKey: null } }).findAll('[role="treeitem"]')[4]
    expect(header.text()).toContain('Diverged')
    expect(header.text()).toContain("Couldn't check for an open pull request")
    expect(header.text()).not.toContain("added to your review's text")
  })

  it('claims nothing about Viewed in the header, since it holds only per file', () => {
    const mixed = [...sub, { filename: 'lib/y.c', repo: 'lib', submodule: 'lib', status: 'modified', additions: 1, deletions: 0, viewed: false, viewed_local: true }] as model.PRFileDTO[]
    const header = mount(DiffFileSidebar, { props: { files: mixed, selectedKey: null } }).findAll('[role="treeitem"]')[4]
    expect(header.text()).toContain('Has an open PR: o/lib#3')
    expect(header.text()).not.toContain('Viewed')
  })

  it('shows the shield only for access problems, and lists the files of an added submodule', () => {
    const header = (extra: Partial<model.PRFileDTO>, inside = sub.slice(4)) =>
      mount(DiffFileSidebar, { props: { files: [...files, { ...sub[3], ...extra }, ...inside], selectedKey: null } }).findAll('[role="treeitem"]')[4]
    const denied = header({ submodule_unavailable: 'no access' }, [])
    expect(denied.find('.lucide-shield-off').exists()).toBe(true)
    expect(denied.text()).not.toContain('0 files')
    expect(header({}, []).text()).toContain('0 files')
    const added = header({ submodule_unavailable: 'submodule added', submodule_range: '', submodule_backwards: false })
    expect(added.text()).toContain('Submodule added')
    expect(added.text()).toContain('1 file')
    expect(added.text()).toContain('Has an open PR: o/lib#3')
    expect(added.find('.lucide-shield-off').exists()).toBe(false)
    const failed = header({ submodule_unavailable: 'failed to load' })
    expect(failed.text()).toContain('Failed to load')
    expect(failed.find('.lucide-shield-off').exists()).toBe(false)
  })

  it("says in the group header when the submodule PR's Viewed marks couldn't be read", () => {
    const header = (extra: Partial<model.PRFileDTO>) =>
      mount(DiffFileSidebar, { props: { files: [...files, { ...sub[3], ...extra }, ...sub.slice(4)], selectedKey: null } }).findAll('[role="treeitem"]')[4]
    expect(header({}).text()).not.toContain("Couldn't read Viewed marks")
    expect(header({ submodule_viewed_unknown: true }).text()).toContain("Couldn't read Viewed marks from o/lib#3")
  })

  it('shows no counts for a file whose lines are unknown, and totals the group so far', () => {
    const added = [
      { ...sub[3], submodule_range: '', submodule_unavailable: 'submodule added' },
      { filename: 'lib/x.c', repo: 'lib', submodule: 'lib', status: 'added', additions: 12, deletions: 0, viewed: false },
      { filename: 'lib/y.c', repo: 'lib', submodule: 'lib', status: 'added', additions: 0, deletions: 0, viewed: false, lines_unknown: true },
    ] as model.PRFileDTO[]
    const items = mount(DiffFileSidebar, { props: { files: added, selectedKey: null } }).findAll('[role="treeitem"]')
    expect(items[0].text()).toContain('2 files')
    expect(items[0].text()).toContain('+12so far')
    expect(items[3].attributes('title')).toBe('lib/y.c')
    expect(items[3].text()).not.toMatch(/[+-]\d/)

    const known = mount(DiffFileSidebar, { props: { files: added.map((f) => ({ ...f, lines_unknown: false })), selectedKey: null } })
    expect(known.findAll('[role="treeitem"]')[0].text()).not.toContain('so far')
  })

  it('folds a submodule apart from a main-repository folder at the same path', async () => {
    const both = [{ filename: 'lib/old.c', status: 'removed', additions: 0, deletions: 3, viewed: false }, ...sub.slice(3)] as model.PRFileDTO[]
    const wrapper = mount(DiffFileSidebar, { props: { files: both, selectedKey: null } })
    const items = () => wrapper.findAll('[role="treeitem"]')
    expect(items().map((i) => i.attributes('tabindex')).filter((t) => t === '0')).toHaveLength(1)
    await items()[2].trigger('click')
    expect(items()[2].attributes('aria-expanded')).toBe('false')
    expect(items()[0].attributes('aria-expanded')).toBe('true')
  })

  it('folds a same-path folder in one tree without touching the other, keeping one tab stop', async () => {
    const both = [
      { filename: 'lib/src/old.c', status: 'removed', additions: 0, deletions: 3, viewed: false },
      { filename: 'lib/top.c', status: 'removed', additions: 0, deletions: 1, viewed: false },
      { ...sub[3] },
      { filename: 'lib/src/new.c', repo: 'lib', submodule: 'lib', status: 'added', additions: 3, deletions: 0, viewed: false },
      { filename: 'lib/x.c', repo: 'lib', submodule: 'lib', status: 'added', additions: 1, deletions: 0, viewed: false },
    ] as model.PRFileDTO[]
    const wrapper = mount(DiffFileSidebar, { props: { files: both, selectedKey: fileKey(both[3]), active: true } })
    const folders = () => wrapper.findAll('[role="treeitem"][title="lib/src"]')
    expect(folders()).toHaveLength(2)
    expect(wrapper.findAll('[tabindex="0"]')).toHaveLength(1)

    await folders()[0].trigger('click')
    expect(folders().map((f) => f.attributes('aria-expanded'))).toEqual(['false', 'true'])
    await folders()[0].trigger('click')
    const rising = wrapper.findAll('.wow-rise').map((r) => r.element.closest('[role="treeitem"]')?.getAttribute('title'))
    expect(rising).toEqual(['lib/src/old.c'])

    await folders()[1].trigger('click')
    expect(folders().map((f) => f.attributes('aria-expanded'))).toEqual(['true', 'false'])
  })

  it('badges an added or removed submodule in its header, wrapping rather than clipping', () => {
    const header = (range: string) =>
      mount(DiffFileSidebar, { props: { files: [{ ...sub[3], submodule_range: range, submodule_backwards: false }, ...sub.slice(4)], selectedKey: null } }).findAll('[role="treeitem"]')[0]
    expect(header('0000000..b2c3d4e').text()).toContain('Added')
    expect(header('b2c3d4e..0000000').text()).toContain('Removed')
    expect(header('a1..b2-dirty').text()).not.toMatch(/Added|Removed/)
    expect(header('0000000..b2c3d4e').find('.flex-wrap').exists()).toBe(true)
  })

  it('leaves out the routing line in a local review', () => {
    const wrapper = mount(DiffFileSidebar, { props: { files: sub, selectedKey: null, local: true } })
    expect(wrapper.text()).not.toContain('Has an open PR')
  })
})

describe('DiffFileSidebar files sharing a path', () => {
  it('lists a main-repository file and a submodule file at one path as two rows, each selected and marked on its own', async () => {
    const shared = [
      { filename: 'lib/a.txt', status: 'removed', additions: 0, deletions: 1, viewed: true },
      { filename: 'lib', status: 'submodule', submodule: 'lib', additions: 1, deletions: 1, viewed: false },
      { filename: 'lib/a.txt', repo: 'lib', submodule: 'lib', status: 'added', additions: 1, deletions: 0, viewed: false },
    ] as model.PRFileDTO[]
    const wrapper = mount(DiffFileSidebar, { props: { files: shared, selectedKey: fileKey(shared[2]) }, attachTo: document.body })
    const rows = wrapper.findAll('[role="treeitem"][title="lib/a.txt"]')
    expect(rows).toHaveLength(2)
    expect(rows.map((r) => r.attributes('aria-selected'))).toEqual(['false', 'true'])
    expect(rows.map((r) => r.find('span.font-medium').classes('text-muted-foreground'))).toEqual([true, false])

    await rows[0].trigger('click')
    await rows[1].trigger('click')
    expect(wrapper.emitted('select')).toEqual([['lib/a.txt'], [fileKey(shared[2])]])
    wrapper.unmount()
  })
})
