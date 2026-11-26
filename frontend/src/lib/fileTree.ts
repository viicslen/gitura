/** A folder's `key` tells it apart from a folder at the same path in another repository's tree. */
export type FileTreeNode<T> =
  | { type: 'dir'; path: string; key: string; name: string; children: FileTreeNode<T>[] }
  | { type: 'file'; name: string; file: T }

export type FileTreeRow<T> =
  | { type: 'dir'; path: string; key: string; name: string; depth: number; expanded: boolean }
  | { type: 'file'; name: string; file: T; depth: number }

interface Keyed {
  filename: string
  repo?: string
  status?: string
}

/** The repository holding a row, as the backend reports it: '' for the main repository. */
export const repoOf = (f: Keyed): string => f.repo ?? ''

/** Whether a row is a submodule's pointer change, which a regular file can share its repository and path with. */
export const isPointer = (f: Keyed): boolean => f.status === 'submodule'

/** Identifies a file by repository, path and whether it is a pointer row; a main-repository file's key is its path, as review threads name it. */
export const keyOf = (repo: string, path: string, pointer = false): string =>
  pointer ? `${repo}\u0000${path}\u0000pointer` : repo ? `${repo}\u0000${path}` : path

/** A file's identity: a main-repository file and a submodule file can share a path, as can a pointer row and the file replacing it. */
export const fileKey = (f: Keyed): string => keyOf(repoOf(f), f.filename, isPointer(f))

/** Identifies a folder or file across the tree and its flattened rows. */
export const treeKey = (n: { type: 'dir'; key: string } | { type: 'file'; file: Keyed }): string =>
  n.type === 'dir' ? `dir:${n.key}` : fileKey(n.file)

/**
 * Groups files by folder, keeping the input order, and merges folders whose only
 * child is another folder into one node (`frontend/src`), as JetBrains does.
 * With `base`, the files all sit under that folder and the tree starts inside it.
 * Folder keys are their paths, qualified by `scope` when given.
 */
export function buildFileTree<T extends { filename: string }>(files: T[], base = '', scope = ''): FileTreeNode<T>[] {
  const root: FileTreeNode<T> & { type: 'dir' } = { type: 'dir', path: base, key: '', name: '', children: [] }
  for (const file of files) {
    const parts = (base ? file.filename.slice(base.length + 1) : file.filename).split('/')
    let dir = root
    for (const part of parts.slice(0, -1)) {
      const path = dir.path ? `${dir.path}/${part}` : part
      let next = dir.children.find((c): c is typeof root => c.type === 'dir' && c.path === path)
      if (!next) {
        next = { type: 'dir', path, key: keyOf(scope, path), name: part, children: [] }
        dir.children.push(next)
      }
      dir = next
    }
    dir.children.push({ type: 'file', name: parts[parts.length - 1], file })
  }

  const compress = (node: FileTreeNode<T>): FileTreeNode<T> => {
    if (node.type === 'file') return node
    let dir = node
    while (dir.children.length === 1 && dir.children[0].type === 'dir') {
      const only = dir.children[0]
      dir = { ...only, name: `${dir.name}/${only.name}` }
    }
    return { ...dir, children: dir.children.map(compress) }
  }
  return root.children.map(compress)
}

/** The rows to render, skipping the contents of collapsed folders. */
export function flattenFileTree<T>(nodes: FileTreeNode<T>[], collapsed: Set<string>, depth = 0): FileTreeRow<T>[] {
  return nodes.flatMap((n): FileTreeRow<T>[] => {
    if (n.type === 'file') return [{ type: 'file', name: n.name, file: n.file, depth }]
    const expanded = !collapsed.has(n.key)
    return [
      { type: 'dir', path: n.path, key: n.key, name: n.name, depth, expanded },
      ...(expanded ? flattenFileTree(n.children, collapsed, depth + 1) : []),
    ]
  })
}
