import { describe, it, expect } from 'vitest'
import type * as model from '@/bindings/gitura/internal/model'
import { fileKey, flattenFileTree, keyOf } from './fileTree'
import { BACKWARDS_NOTICE, DIVERGED_NOTICE, groupedTree, groupSubmodules, noticeText, prLabel, submoduleNotices, submoduleRouting, submodulesLast } from './submodules'

const file = (filename: string, extra: Partial<model.PRFileDTO> = {}) =>
  ({ filename, status: 'modified', additions: 1, deletions: 0, viewed: false, ...extra }) as model.PRFileDTO
const pointer = (path: string, extra: Partial<model.PRFileDTO> = {}) =>
  file(path, { status: 'submodule', submodule: path, submodule_range: 'aaa..bbb', ...extra })

const lib = pointer('vendor/lib')
const files = [
  file('src/main.go'),
  file('vendor/lib/src/parse/hunk.c', { repo: 'vendor/lib', submodule: 'vendor/lib' }),
  lib,
  file('vendor/lib/README.md', { repo: 'vendor/lib', submodule: 'vendor/lib' }),
  file('README.md'),
]

const render = (shown = files) =>
  flattenFileTree(groupedTree(groupSubmodules(files, shown)), new Set()).map((r) => `${'  '.repeat(r.depth)}${r.name}`)

describe('groupSubmodules', () => {
  it('lists main-repository files first, then each submodule with its pointer row before its own tree', () => {
    expect(render()).toEqual([
      'src',
      '  main.go',
      'README.md',
      'vendor/lib',
      '  Pointer change',
      '  src/parse',
      '    hunk.c',
      '  README.md',
    ])
  })

  it('keys a group apart from a main-repository folder whose name looks like a group key', () => {
    const tree = groupedTree(groupSubmodules([file('sub:lib/x.go'), pointer('lib'), file('lib/a.c', { repo: 'lib', submodule: 'lib' })]))
    const keys = tree.flatMap((n) => (n.type === 'dir' ? [n.key] : []))
    expect(new Set(keys).size).toBe(2)
  })

  it('never merges submodule folders into the parent tree', () => {
    const { main, groups } = groupSubmodules([file('vendor/other.go'), lib, file('vendor/lib/x.c', { repo: 'vendor/lib', submodule: 'vendor/lib' })])
    expect(main.map((n) => n.name)).toEqual(['vendor'])
    expect(groups[0].nodes.map((n) => n.name)).toEqual(['Pointer change', 'x.c'])
  })

  it('counts every file in a group but lists only the shown ones, and drops a group with nothing shown', () => {
    const shown = files.filter((f) => f.filename !== 'vendor/lib/README.md')
    const { groups } = groupSubmodules(files, shown)
    expect(groups[0].files).toHaveLength(2)
    expect(render(shown)).not.toContain('  README.md')
    expect(groupSubmodules(files, [files[0]]).groups).toEqual([])
  })

  it('gives a submodule nested in another its own group, holding only its own files', () => {
    const core = pointer('vendor/lib/deps/core', { repo: 'vendor/lib', submodule_pr: 'o/core#2' })
    const nested = [...files, core, file('vendor/lib/deps/core/x.c', { repo: 'vendor/lib/deps/core', submodule: 'vendor/lib/deps/core' })]
    const { groups } = groupSubmodules(nested)
    expect(groups.map((g) => [g.pointer.filename, g.files.map((f) => f.filename)])).toEqual([
      ['vendor/lib', ['vendor/lib/src/parse/hunk.c', 'vendor/lib/README.md']],
      ['vendor/lib/deps/core', ['vendor/lib/deps/core/x.c']],
    ])
    expect(groups[1].nodes.map((n) => n.name)).toEqual(['Pointer change', 'x.c'])
  })

  it("tells a parent file and a nested submodule's pointer at one path apart by their repo", () => {
    const own = file('lib/nest', { status: 'removed' })
    const nest = pointer('lib/nest', { repo: 'lib' })
    const rows = [pointer('lib'), file('lib/a.c', { repo: 'lib', submodule: 'lib' }), nest, own]
    expect(fileKey(own)).not.toBe(fileKey(nest))
    const keys = flattenFileTree(groupedTree(groupSubmodules(rows)), new Set()).flatMap((r) => (r.type === 'file' ? [fileKey(r.file)] : []))
    expect(keys).toEqual([fileKey(own), fileKey(rows[0]), fileKey(rows[1]), fileKey(nest)])
  })

  it("keeps the parent's own removed submodule under an added one in the parent repository", () => {
    const vendor = pointer('lib/vendor', { submodule_range: 'aaa..0000000' })
    const rows = [pointer('lib', { submodule_range: '0000000..bbb' }), file('lib/a.c', { repo: 'lib', submodule: 'lib' }), vendor]
    expect(fileKey(vendor)).toBe(keyOf('', 'lib/vendor', true))
    expect(groupSubmodules(rows).groups.map((g) => [g.pointer.filename, g.files.map((f) => f.filename)])).toEqual([
      ['lib', ['lib/a.c']],
      ['lib/vendor', []],
    ])
  })

  it('keeps a submodule whose files are unavailable as just its pointer', () => {
    const sdk = pointer('third_party/sdk', { submodule_unavailable: 'no access' })
    expect(groupSubmodules([sdk]).groups.map((g) => g.nodes.map((n) => n.name))).toEqual([['Pointer change']])
  })
})

describe('submodulesLast', () => {
  it('orders files like the tree: main files, then each submodule pointer-first', () => {
    expect(submodulesLast(files).map((f) => f.filename)).toEqual([
      'src/main.go',
      'README.md',
      'vendor/lib',
      'vendor/lib/src/parse/hunk.c',
      'vendor/lib/README.md',
    ])
  })
})

describe('submoduleNotices', () => {
  const inner = (p: model.PRFileDTO) => [p, file(`${p.filename}/a.c`, { repo: p.filename, submodule: p.filename })] as const

  it('says comments can go to the submodule PR when there is one', () => {
    const [p, f] = inner(pointer('lib', { submodule_pr: 'o/lib#12', submodule_pr_url: 'https://github.com/o/lib/pull/12' }))
    expect(submoduleNotices(f, [p, f], false)).toEqual({
      backwards: false,
      diverged: false,
      prUnknown: false,
      routing: { text: 'Comments can go to', pr: 'o/lib#12', url: 'https://github.com/o/lib/pull/12', after: ' when you submit; Viewed syncs there' },
    })
  })

  it('says when a file outside the submodule PR keeps its Viewed mark in gitura', () => {
    const p = pointer('lib', { submodule_pr: 'o/lib#12', submodule_pr_url: 'https://github.com/o/lib/pull/12' })
    const inPR = file('lib/a.c', { repo: 'lib', submodule: 'lib' })
    const outside = file('lib/b.c', { repo: 'lib', submodule: 'lib', viewed_local: true })
    expect(noticeText(submoduleRouting(p, false) ?? { text: '' })).toBe('Has an open PR: o/lib#12')
    expect(noticeText(submoduleNotices(inPR, [p, inPR, outside], false).routing ?? { text: '' })).toBe(
      'Comments can go to o/lib#12 when you submit; Viewed syncs there',
    )
    expect(noticeText(submoduleNotices(outside, [p, inPR, outside], false).routing ?? { text: '' })).toBe(
      "Outside o/lib#12: comments are added to your review's text and Viewed stays in gitura",
    )
  })

  it('says review comments fold into the review text without a submodule PR, and where a comment posted now goes', () => {
    const [p, f] = inner(pointer('lib'))
    expect(submoduleNotices(f, [p, f], false).routing).toEqual({
      text: "Review comments here are added to your review's text; a comment posted now goes to this PR's conversation",
    })
  })

  it('says the open-PR lookup failed instead of claiming there is no PR', () => {
    const [p, f] = inner(pointer('lib', { submodule_pr_unknown: true }))
    expect(submoduleNotices(f, [p, f], false).routing).toEqual({ text: "Couldn't check for an open pull request" })
  })

  it('says nothing about routing in a local review, on the pointer itself, or on a main-repository file', () => {
    const [p, f] = inner(pointer('lib'))
    expect(submoduleNotices(f, [p, f], true).routing).toBeNull()
    expect(submoduleNotices(p, [p, f], false).routing).toBeNull()
    expect(submoduleNotices(file('x.go'), [p, f], false)).toEqual({ backwards: false, diverged: false, prUnknown: false, routing: null })
  })

  it('flags a backwards bump on the pointer and its files, local or not', () => {
    const [p, f] = inner(pointer('lib', { submodule_backwards: true }))
    expect(submoduleNotices(p, [p, f], false).backwards).toBe(true)
    expect(submoduleNotices(f, [p, f], true).backwards).toBe(true)
    expect(BACKWARDS_NOTICE).toContain('moves the submodule backwards')
  })

  it('flags diverged commits on the pointer and its files', () => {
    const [p, f] = inner(pointer('lib', { submodule_diverged: true }))
    expect(submoduleNotices(p, [p, f], false)).toMatchObject({ diverged: true, backwards: false })
    expect(submoduleNotices(f, [p, f], false).diverged).toBe(true)
    expect(DIVERGED_NOTICE).toContain('have diverged')
  })
})

describe('prLabel', () => {
  it('names a pull request or review link as owner/repo#n', () => {
    expect(prLabel('https://github.com/o/lib/pull/12#pullrequestreview-9')).toBe('o/lib#12')
    expect(prLabel('https://example.com/x')).toBe('https://example.com/x')
  })
})
