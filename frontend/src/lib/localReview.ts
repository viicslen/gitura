import type * as model from '@/bindings/gitura/internal/model'

/** Renders local review comments as markdown a coding agent can act on. */
export function formatLocalReview(comments: model.DraftCommentDTO[]): string {
  const sections = comments.map((c) => {
    const repo = c.submodule ? `submodule ${c.submodule}` : ''
    const sub = c.pointer ? ` (submodule pointer${repo ? `, in ${repo}` : ''})` : repo ? ` (${repo})` : ''
    if (!c.line) return `### ${c.path}${sub}\n\n${c.body.trim()}`
    const lines = c.start_line ? `${c.start_line}-${c.line}` : `${c.line}`
    const old = c.side === 'LEFT' ? ' (old)' : ''
    return `### ${c.path}:${lines}${old}${sub}\n\n${c.body.trim()}`
  })
  return ['Review comments on the local changes in this repository:', ...sections].join('\n\n') + '\n'
}
