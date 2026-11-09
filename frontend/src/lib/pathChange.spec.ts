import { describe, it, expect } from 'vitest'
import { describePathChange } from './pathChange'

describe('describePathChange', () => {
  it('marks the changed folder of a move', () => {
    expect(describePathChange(
      'domain/Inventory/Docs/research/open-library.md',
      'domain/ProductDrafts/Docs/research/open-library.md',
    )).toEqual({ kind: 'moved', prefix: 'domain/', from: 'Inventory', to: 'ProductDrafts', suffix: '/Docs/research/open-library.md' })
  })

  it('marks the changed file name of a rename', () => {
    expect(describePathChange('src/a/old.ts', 'src/a/new.ts'))
      .toEqual({ kind: 'renamed', prefix: 'src/a/', from: 'old.ts', to: 'new.ts', suffix: '' })
  })

  it('handles a move that also renames, and a move between depths', () => {
    expect(describePathChange('a/x.ts', 'b/y.ts').kind).toBe('moved and renamed')
    expect(describePathChange('x.ts', 'lib/x.ts'))
      .toEqual({ kind: 'moved', prefix: '', from: '', to: 'lib', suffix: '/x.ts' })
  })
})
