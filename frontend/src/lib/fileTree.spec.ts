import { describe, it, expect } from 'vitest'
import { buildFileTree, flattenFileTree } from './fileTree'

const files = ['README.md', 'frontend/src/components/A.vue', 'frontend/src/components/B.vue', 'frontend/src/lib/c.ts', 'internal/github/d.go']
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
      'internal/github',
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
      'internal/github',
      '  d.go',
    ])
  })
})
