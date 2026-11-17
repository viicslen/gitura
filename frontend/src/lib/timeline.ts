import type * as model from '@/bindings/gitura/internal/model'

export type TimelineEntry =
  | { kind: 'commit'; at: string; commit: model.CommitDTO }
  | { kind: 'review'; at: string; review: model.ReviewDTO; number: number }
  | { kind: 'comment'; at: string; comment: model.IssueCommentDTO }
  | { kind: 'thread'; at: string; thread: model.CommentThreadDTO; comment: model.ReviewCommentDTO; reply: boolean }

/** Interleaves commits, reviews, general comments and every thread comment, oldest first. */
export function buildTimeline(data: model.PRTimelineDTO, threads: model.CommentThreadDTO[]): TimelineEntry[] {
  // A comment drafted in a pending review is only visible once the review is submitted.
  const published = new Map(data.reviews.map((r) => [r.review_id, r.submitted_at]))
  const entries: TimelineEntry[] = [
    ...data.commits.map((commit) => ({ kind: 'commit' as const, at: commit.committed_at, commit })),
    ...data.reviews.flatMap((review) => {
      const number = data.review_numbers?.[`${review.review_id}`]
      return number ? [{ kind: 'review' as const, at: review.submitted_at, review, number }] : []
    }),
    ...data.general_comments.map((comment) => ({ kind: 'comment' as const, at: comment.created_at, comment })),
    ...threads.flatMap((thread) =>
      thread.comments.map((comment, i) => ({
        kind: 'thread' as const,
        at: published.get(comment.review_id) || comment.created_at,
        thread,
        comment,
        reply: i > 0,
      })),
    ),
  ]
  // Stable, so on a tie a commit stays ahead of the review and comments published with it.
  return entries.sort((a, b) => Date.parse(a.at) - Date.parse(b.at))
}
