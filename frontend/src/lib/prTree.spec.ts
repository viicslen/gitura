import { describe, expect, it } from 'vitest'
import { prTree } from './prTree'
import type * as model from '@/bindings/gitura/internal/model'

function pr(number: number, base: string, head: string, extra: Partial<model.PRListItem> = {}): model.PRListItem {
  return {
    number,
    title: `#${number}`,
    owner: 'o',
    repo: 'r',
    author_login: 'me',
    created_at: '',
    updated_at: '',
    html_url: `https://github.com/o/r/pull/${number}`,
    is_draft: false,
    state: 'open',
    is_author: true,
    is_assignee: false,
    is_reviewer: false,
    base_ref: base,
    head_ref: head,
    head_repo: 'o/r',
    ...extra,
  }
}

const shape = (items: model.PRListItem[]) => prTree(items).map(({ pr, depth }) => `${pr.number}@${depth}`)

describe('prTree', () => {
  it('nests a stack under its base PR, keeping list order among siblings', () => {
    const items = [pr(3, 'feat1', 'feat3'), pr(9, 'main', 'other'), pr(2, 'feat1', 'feat2'), pr(1, 'main', 'feat1'), pr(4, 'feat2', 'feat4')]
    expect(shape(items)).toEqual(['9@0', '1@0', '3@1', '2@1', '4@2'])
  })

  it('leaves a PR at the top when its base PR is filtered out or in another repository', () => {
    expect(shape([pr(2, 'feat1', 'feat2'), pr(1, 'main', 'feat1', { repo: 'other', head_repo: 'o/other' })])).toEqual(['2@0', '1@0'])
  })

  it('does not stack on a branch that lives in a fork', () => {
    expect(shape([pr(1, 'main', 'feat1', { head_repo: 'fork/r' }), pr(2, 'feat1', 'feat2')])).toEqual(['1@0', '2@0'])
  })

  it('stacks a PR opened against a fork branch under the PR from that branch', () => {
    const parent = pr(1, 'main', 'feat1', { head_repo: 'alice/r' })
    const child = pr(2, 'feat1', 'feat2', { owner: 'alice', head_repo: 'alice/r', html_url: 'https://github.com/alice/r/pull/2' })
    expect(prTree([parent, child]).map(({ pr, depth, parent }) => [pr.number, depth, parent?.number])).toEqual([[1, 0, undefined], [2, 1, 1]])
  })

  it('leaves a PR at the top when more than one PR is opened from its base branch', () => {
    expect(shape([pr(3, 'feat1', 'feat3'), pr(1, 'main', 'feat1'), pr(2, 'release', 'feat1')])).toEqual(['3@0', '1@0', '2@0'])
  })

  it('keeps a branch ambiguous when filters hide all but one of the PRs opened from it', () => {
    const items = [pr(3, 'feat1', 'feat3'), pr(1, 'main', 'feat1'), pr(2, 'release', 'feat1')]
    expect(prTree(items.slice(0, 2), items).map(({ pr, depth }) => `${pr.number}@${depth}`)).toEqual(['3@0', '1@0'])
  })

  it('shows PRs whose bases loop, once each', () => {
    expect(shape([pr(1, 'b', 'a'), pr(2, 'a', 'b')])).toEqual(['1@0', '2@1'])
  })

  it('keeps every PR flat when branch refs are missing', () => {
    expect(shape([pr(1, '', ''), pr(2, '', '')])).toEqual(['1@0', '2@0'])
  })
})
