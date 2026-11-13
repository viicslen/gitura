export type FileTreeNode<T> =
  | { type: 'dir'; path: string; name: string; children: FileTreeNode<T>[] }
  | { type: 'file'; name: string; file: T }

export type FileTreeRow<T> =
  | { type: 'dir'; path: string; name: string; depth: number; expanded: boolean }
  | { type: 'file'; name: string; file: T; depth: number }

/** Identifies a folder or file across the tree and its flattened rows. */
export const treeKey = (n: { type: 'dir'; path: string } | { type: 'file'; file: { filename: string } }): string =>
  n.type === 'dir' ? `dir:${n.path}` : n.file.filename

/**
 * Groups files by folder, keeping the input order, and merges folders whose only
 * child is another folder into one node (`frontend/src`), as JetBrains does.
 */
export function buildFileTree<T extends { filename: string }>(files: T[]): FileTreeNode<T>[] {
  const root: FileTreeNode<T> & { type: 'dir' } = { type: 'dir', path: '', name: '', children: [] }
  for (const file of files) {
    const parts = file.filename.split('/')
    let dir = root
    for (const part of parts.slice(0, -1)) {
      const path = dir.path ? `${dir.path}/${part}` : part
      let next = dir.children.find((c): c is typeof root => c.type === 'dir' && c.path === path)
      if (!next) {
        next = { type: 'dir', path, name: part, children: [] }
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
    const expanded = !collapsed.has(n.path)
    return [
      { type: 'dir', path: n.path, name: n.name, depth, expanded },
      ...(expanded ? flattenFileTree(n.children, collapsed, depth + 1) : []),
    ]
  })
}
