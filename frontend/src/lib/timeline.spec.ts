import { describe, it, expect } from 'vitest'
import { buildTimeline } from './timeline'
import type * as model from '@/bindings/gitura/internal/model'

const commit = (sha: string, at: string) => ({ sha, committed_at: at }) as model.CommitDTO
const review = (id: number, at: string) => ({ review_id: id, submitted_at: at }) as model.ReviewDTO
const comment = (id: number, reviewID: number, at: string) => ({ id, review_id: reviewID, created_at: at }) as model.ReviewCommentDTO

describe('buildTimeline', () => {
  it('orders by time, places drafted comments at their review, and hides unnumbered reviews', () => {
    const data = {
      commits: [commit('a', '2026-01-01T10:00:00Z'), commit('b', '2026-01-03T10:00:00Z')],
      // Review 2 holds only a reply, so it has no number.
      reviews: [review(1, '2026-01-04T10:00:00Z'), review(2, '2026-01-02T10:00:00Z')],
      general_comments: [{ id: 9, created_at: '2026-01-05T10:00:00Z' } as model.IssueCommentDTO],
      review_numbers: { 1: 1 },
    } as model.PRTimelineDTO
    // Drafted before commit b, but published with review 1 after it.
    const thread = { root_id: 10, comments: [comment(10, 1, '2026-01-02T12:00:00Z'), comment(11, 2, '2026-01-01T00:00:00Z')] } as model.CommentThreadDTO

    const order = buildTimeline(data, [thread]).map((e) =>
      e.kind === 'commit' ? e.commit.sha : e.kind === 'review' ? `r${e.review.review_id}` : e.kind === 'comment' ? `c${e.comment.id}` : `t${e.comment.id}${e.reply ? '-reply' : ''}`,
    )
    expect(order).toEqual(['a', 't11-reply', 'b', 'r1', 't10', 'c9'])
  })
})
