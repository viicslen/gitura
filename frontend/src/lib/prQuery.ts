import { shallowRef } from 'vue'
import type * as model from '@/bindings/gitura/internal/model'

/** The open PRs involving the user, as last loaded by the PR list. */
export const myOpenPRs = shallowRef<model.PRListItem[]>([])

/** A PR number typed into search: with its repository when the text names one, as `owner/repo#123` or a PR URL. */
export type PRNumberQuery = { number: number; owner?: string; repo?: string }

/** Reads `123`, `#123`, `owner/repo#123` or `https://github.com/owner/repo/pull/123[/...]`; null for anything else. */
export function parsePRNumber(text: string): PRNumberQuery | null {
  const t = text.trim()
  const bare = /^#?(\d+)$/.exec(t)
  if (bare) return { number: Number(bare[1]) }
  const ref = /^([\w.-]+)\/([\w.-]+)#(\d+)$/.exec(t) ?? /^(?:https?:\/\/)?github\.com\/([\w.-]+)\/([\w.-]+)\/pull\/(\d+)(?:[/?#].*)?$/.exec(t)
  return ref ? { owner: ref[1], repo: ref[2], number: Number(ref[3]) } : null
}
