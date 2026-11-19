import type * as model from '@/bindings/gitura/internal/model'

type Authored = { author_login: string; body: string }

const SEPARATOR = '\n\n---\n\n'

/** Comments as one text, each under its author, for a command run on a whole conversation. */
export function conversationInput(comments: Authored[]): string {
  return comments
    .filter((c) => c.body.trim())
    .map((c) => `@${c.author_login}:\n${c.body.trim()}`)
    .join(SEPARATOR)
}

export function threadInput(thread: model.CommentThreadDTO): string {
  return `${thread.path}${thread.line ? `:${thread.line}` : ''}\n\n${conversationInput(thread.comments)}`
}

/** A review's body followed by every inline thread it opened. */
export function reviewInput(review: model.ReviewDTO, threads: model.CommentThreadDTO[]): string {
  return [conversationInput([review]), ...threads.filter((t) => t.review_id === review.review_id).map(threadInput)]
    .filter(Boolean)
    .join(SEPARATOR)
}

/** Everything said on the PR: description, general comments, review bodies and every inline thread. */
export function prInput(overview: model.PROverviewDTO, threads: model.CommentThreadDTO[]): string {
  const { title, body } = overview.summary
  return [
    `# ${title}${body.trim() ? `\n\n${body.trim()}` : ''}`,
    conversationInput([...overview.general_comments, ...overview.reviews]),
    ...threads.map(threadInput),
  ]
    .filter(Boolean)
    .join(SEPARATOR)
}
