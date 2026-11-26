import { describe, it, expect } from 'vitest'
import { buildFileTree, fileKey, flattenFileTree, keyOf } from './fileTree'

const files = ['README.md', 'frontend/src/components/A.vue', 'frontend/src/components/B.vue', 'frontend/src/lib/c.ts', 'internal/ghapi/d.go']
  .map((filename) => ({ filename }))

const render = (collapsed: string[] = []) =>
  flattenFileTree(buildFileTree(files), new Set(collapsed)).map((r) => `${'  '.repeat(r.depth)}${r.name}`)

describe('file tree', () => {
  it('groups by folder in input order and merges single-folder chains', () => {
    expect(render()).toEqual([
      'README.md',
      'frontend/src',
      '  components',
      '    A.vue',
      '    B.vue',
      '  lib',
      '    c.ts',
      'internal/ghapi',
      '  d.go',
    ])
  })

  it('hides the contents of collapsed folders', () => {
    expect(render(['frontend/src/components'])).toEqual([
      'README.md',
      'frontend/src',
      '  components',
      '  lib',
      '    c.ts',
      'internal/ghapi',
      '  d.go',
    ])
  })
})

describe('file tree under a base folder', () => {
  it('starts inside the base and keeps full paths on its folders', () => {
    const tree = buildFileTree([{ filename: 'vendor/lib/src/a.c' }, { filename: 'vendor/lib/b.c' }], 'vendor/lib')
    expect(tree.map((n) => (n.type === 'dir' ? n.path : n.file.filename))).toEqual(['vendor/lib/src', 'vendor/lib/b.c'])
  })

  it('qualifies folder keys with a scope, so a folder at the same path in the main tree keeps its own', () => {
    const files = [{ filename: 'lib/src/a.c' }, { filename: 'lib/b.c' }]
    const keys = (nodes: ReturnType<typeof buildFileTree>) => flattenFileTree(nodes, new Set()).flatMap((r) => (r.type === 'dir' ? [r.key] : []))
    expect(keys(buildFileTree(files, 'lib', 'sub:lib'))).toEqual(['sub:lib\u0000lib/src'])
    expect(keys(buildFileTree(files))).toEqual(['lib', 'lib/src'])
  })
})

describe('file keys', () => {
  it('tell a pointer row from a regular file at the same repository and path', () => {
    const rows = [
      { filename: 'lib', status: 'submodule' },
      { filename: 'lib', status: 'added' },
      { filename: 'lib/nest', status: 'submodule', repo: 'lib' },
      { filename: 'lib/nest', status: 'removed', repo: 'lib' },
    ]
    expect(new Set(rows.map(fileKey)).size).toBe(rows.length)
    expect(fileKey(rows[1])).toBe('lib')
    expect(fileKey(rows[0])).toBe(keyOf('', 'lib', true))
  })
})
