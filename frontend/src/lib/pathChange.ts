export interface PathChange {
  kind: 'moved' | 'renamed' | 'moved and renamed'
  /** Unchanged leading folders, including the trailing slash. */
  prefix: string
  from: string
  to: string
  /** Unchanged trailing part (folders and/or file name), including its leading slash. */
  suffix: string
}

/** Splits a rename into its unchanged ends and the changed middle, by path segment. */
export function describePathChange(oldPath: string, newPath: string): PathChange {
  const a = oldPath.split('/')
  const b = newPath.split('/')
  let head = 0
  while (head < Math.min(a.length, b.length) - 1 && a[head] === b[head]) head++
  let tail = 0
  while (tail < Math.min(a.length, b.length) - head && a[a.length - 1 - tail] === b[b.length - 1 - tail]) tail++

  const sameName = a[a.length - 1] === b[b.length - 1]
  const sameDir = a.slice(0, -1).join('/') === b.slice(0, -1).join('/')
  return {
    kind: sameName ? 'moved' : sameDir ? 'renamed' : 'moved and renamed',
    prefix: head ? `${a.slice(0, head).join('/')}/` : '',
    from: a.slice(head, a.length - tail).join('/'),
    to: b.slice(head, b.length - tail).join('/'),
    suffix: tail ? `${head < a.length - tail || head < b.length - tail ? '/' : ''}${a.slice(a.length - tail).join('/')}` : '',
  }
}
