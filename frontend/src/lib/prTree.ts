import type * as model from '@/bindings/gitura/internal/model'

export type PRTreeRow = { pr: model.PRListItem; depth: number; parent?: model.PRListItem }

/**
 * Orders items so each PR follows the PR whose head branch it merges into, one level deeper.
 * Roots and siblings keep their order in items; a PR whose parent is not in items is a root.
 * `loaded` is every PR the filters chose from: a branch several of them are opened from stays ambiguous when filters hide all but one.
 */
export function prTree(items: model.PRListItem[], loaded: model.PRListItem[] = items): PRTreeRow[] {
  const shown = new Set(items)
  const branch = (repo: string, ref: string) => `${repo.toLowerCase()}:${ref}`
  // null marks a branch more than one PR is opened from: which one a child stacks on is unknowable.
  const byHead = new Map<string, model.PRListItem | null>()
  for (const pr of loaded) {
    if (!pr.head_ref || !pr.head_repo) continue
    const key = branch(pr.head_repo, pr.head_ref)
    byHead.set(key, byHead.has(key) ? null : pr)
  }
  const children = new Map<model.PRListItem, model.PRListItem[]>()
  const roots: model.PRListItem[] = []
  for (const pr of items) {
    const parent = pr.base_ref ? byHead.get(branch(`${pr.owner}/${pr.repo}`, pr.base_ref)) : undefined
    if (parent && parent !== pr && shown.has(parent)) children.set(parent, [...(children.get(parent) ?? []), pr])
    else roots.push(pr)
  }

  const rows: PRTreeRow[] = []
  const placed = new Set<model.PRListItem>()
  const visit = (pr: model.PRListItem, depth: number, parent?: model.PRListItem) => {
    if (placed.has(pr)) return
    placed.add(pr)
    rows.push({ pr, depth, parent })
    children.get(pr)?.forEach((child) => visit(child, depth + 1, pr))
  }
  roots.forEach((pr) => visit(pr, 0))
  // PRs whose bases form a loop have no root; they still show, starting from the first one listed.
  items.forEach((pr) => visit(pr, 0))
  return rows
}
