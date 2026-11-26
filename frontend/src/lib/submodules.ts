import type * as model from '@/bindings/gitura/internal/model'
import { buildFileTree, isPointer, repoOf, type FileTreeNode } from './fileTree'

type File = model.PRFileDTO

/** "Added" or "Removed" for a submodule the change adds or removes: its pointer's range starts or ends at no commit. */
export function submoduleChange(pointer: File): 'Added' | 'Removed' | null {
  const [from, to = ''] = (pointer.submodule_range ?? '').split('..')
  const none = (sha: string) => /^0+$/.test(sha)
  if (none(from)) return 'Added'
  return none(to.replace(/-dirty$/, '')) ? 'Removed' : null
}

export interface SubmoduleGroup {
  pointer: File
  /** Every changed file inside the submodule, including ones the filters hide. */
  files: File[]
  /** The rows to list: the pointer first, then the shown files as their own tree. */
  nodes: FileTreeNode<File>[]
}

/** Main-repository files as one tree, then a group per submodule; `shown` is what the filters leave. */
export function groupSubmodules(files: File[], shown: File[] = files): { main: FileTreeNode<File>[]; groups: SubmoduleGroup[] } {
  const groups = files.filter(isPointer).map((pointer): SubmoduleGroup => {
    const sub = pointer.filename
    const inside = (f: File) => repoOf(f) === sub && !isPointer(f)
    const pointerRow: FileTreeNode<File>[] = shown.includes(pointer) ? [{ type: 'file', name: 'Pointer change', file: pointer }] : []
    return { pointer, files: files.filter(inside), nodes: [...pointerRow, ...buildFileTree(shown.filter(inside), sub, groupKey(sub))] }
  })
  return { main: buildFileTree(shown.filter((f) => !f.submodule)), groups: groups.filter((g) => g.nodes.length) }
}

/** A group's folder key in the tree, apart from a main-repository folder at the same path so each folds on its own. */
// The NUL keeps it out of reach of any real path, which cannot contain one.
export const groupKey = (sub: string): string => `\u0000${sub}`

/** One tree for both: each group becomes a folder keyed by `groupKey`. */
export const groupedTree = ({ main, groups }: ReturnType<typeof groupSubmodules>): FileTreeNode<File>[] => [
  ...main,
  ...groups.map((g): FileTreeNode<File> => {
    const key = groupKey(g.pointer.filename)
    return { type: 'dir', path: key, key, name: g.pointer.filename, children: g.nodes }
  }),
]

/** Main-repository files, then each submodule's pointer followed by its files, so next/previous follows the list. */
export function submodulesLast(files: File[]): File[] {
  const subs = [...new Set(files.flatMap((f) => (f.submodule ? [f.submodule] : [])))]
  return [
    ...files.filter((f) => !f.submodule),
    ...subs.flatMap((sub) => files.filter((f) => f.submodule === sub).sort((a, b) => Number(isPointer(b)) - Number(isPointer(a)))),
  ]
}

export interface SubmoduleNotice {
  text: string
  /** A pull request the text points at, as "owner/repo#12". */
  pr?: string
  url?: string
  /** Text after the pull request. */
  after?: string
}

/** The notice as one line of plain text. */
export const noticeText = (n: SubmoduleNotice): string => [n.text, n.pr].filter(Boolean).join(' ') + (n.after ?? '')

/** Where a submodule's comments can go, for its header; nothing to say in a local review. */
export function submoduleRouting(pointer: File, local: boolean): SubmoduleNotice | null {
  if (local) return null
  if (pointer.submodule_pr) return { text: 'Has an open PR:', pr: pointer.submodule_pr, url: pointer.submodule_pr_url }
  if (pointer.submodule_pr_unknown) return { text: "Couldn't check for an open pull request" }
  return { text: "Review comments here are added to your review's text; a comment posted now goes to this PR's conversation" }
}

export const BACKWARDS_NOTICE = 'This bump moves the submodule backwards; the diff shows what it undoes.'
export const DIVERGED_NOTICE = "The submodule's old and new commits have diverged; the diff shows old → new directly."

/** What the diff of a file inside or of a submodule notes; `files` is every changed file, to find its submodule row. */
export function submoduleNotices(
  file: File,
  files: File[],
  local: boolean,
): { backwards: boolean; diverged: boolean; prUnknown: boolean; routing: SubmoduleNotice | null } {
  const pointer = file.submodule ? files.find((f) => f.filename === file.submodule && isPointer(f)) : undefined
  const routing = !pointer || isPointer(file)
    ? null
    : local || !pointer.submodule_pr
      ? submoduleRouting(pointer, local)
      : file.viewed_local
        ? { text: 'Outside', pr: pointer.submodule_pr, url: pointer.submodule_pr_url, after: ": comments are added to your review's text and Viewed stays in gitura" }
        : { text: 'Comments can go to', pr: pointer.submodule_pr, url: pointer.submodule_pr_url, after: ' when you submit; Viewed syncs there' }
  return {
    backwards: !!pointer?.submodule_backwards,
    diverged: !!pointer?.submodule_diverged,
    prUnknown: !!pointer?.submodule_pr_unknown,
    routing,
  }
}

/** "owner/repo#12" for a GitHub pull request or review link, else the link itself. */
export function prLabel(url: string): string {
  const m = /github\.com\/([^/]+\/[^/]+)\/pull\/(\d+)/.exec(url)
  return m ? `${m[1]}#${m[2]}` : url
}
