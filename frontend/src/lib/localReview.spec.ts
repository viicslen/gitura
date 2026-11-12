import { describe, expect, it } from 'vitest'
import { formatLocalReview } from './localReview'
import type * as model from '@/bindings/gitura/internal/model'

describe('formatLocalReview', () => {
  it('formats single-line, ranged and old-side comments', () => {
    const comments = [
      { path: 'a.go', line: 3, side: 'RIGHT', body: 'rename this ' },
      { path: 'b.ts', line: 9, side: 'RIGHT', start_line: 7, start_side: 'RIGHT', body: 'extract' },
      { path: 'c.md', line: 2, side: 'LEFT', body: 'why removed?' },
    ] as model.DraftCommentDTO[]
    expect(formatLocalReview(comments)).toBe(
      'Review comments on the local changes in this repository:\n\n' +
        '### a.go:3\n\nrename this\n\n' +
        '### b.ts:7-9\n\nextract\n\n' +
        '### c.md:2 (old)\n\nwhy removed?\n',
    )
  })
})
