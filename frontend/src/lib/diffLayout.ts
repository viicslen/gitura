import type * as model from '@/bindings/gitura/internal/model'
import { pairChanges, type Range } from './inlineDiff'

export type Side = 'LEFT' | 'RIGHT'

/** Where a new inline comment is being written. */
export interface CommentTarget {
  side: Side
  hunk: number
  line: number
  startLine?: number
  /** The head the form was opened on, so a comment placed on older code is refused instead of landing on moved lines. */
  headSha?: string
  /** Likewise the submodule range shown, for a file inside a submodule. */
  range?: string
}

/**
 * Half-written inline comments by file path, kept while the user moves between files.
 * Must be reactive: views of the same file follow an entry's `sending` state.
 */
export type UnsentComments = Map<string, { target: CommentTarget; body: string; sending?: boolean }>
export type BlockKind = 'added' | 'deleted' | 'modified'

/** One row of a pane: a code line, a collapsed run of unchanged lines, or the insertion point of a block with no lines on this side. */
export type PaneItem =
  | { type: 'line'; key: string; no: number; content: string; hunk: number; block: number; changes?: Range[] }
  | { type: 'gap'; key: string; hidden: number }
  | { type: 'mark'; key: string; block: number }

/** A pane item's edge: 'bottom' is the bottom of the code line itself, above any inline rows under it. */
export interface Edge {
  item: number
  at: 'top' | 'bottom'
}

export interface SplitBlock {
  kind: BlockKind
  left: [Edge, Edge]
  right: [Edge, Edge]
}

/** Positions on the two panes that must line up when scrolling in sync. */
export interface SyncPoint {
  left: Edge
  right: Edge
}

export interface SplitLayout {
  left: PaneItem[]
  right: PaneItem[]
  blocks: SplitBlock[]
  sync: SyncPoint[]
}

export interface LineSelection {
  side: Side
  from: number
  to: number
}

export function anchorKey(side: Side, line: number): string {
  return `${side}:${line}`
}

/**
 * Lays hunks out JetBrains-style: each side lists only its own lines, and change blocks
 * are mapped across by connectors. Sync points pair up unchanged lines, gaps and block
 * edges so the panes can scroll at different speeds through blocks of different sizes.
 */
export function buildSplitLayout(hunks: model.DiffHunkDTO[]): SplitLayout {
  const left: PaneItem[] = []
  const right: PaneItem[] = []
  const blocks: SplitBlock[] = []
  const sync: SyncPoint[] = []
  const top = (item: number): Edge => ({ item, at: 'top' })
  const bottom = (item: number): Edge => ({ item, at: 'bottom' })
  const pairEdges = (l: number, r: number): void => {
    sync.push({ left: top(l), right: top(r) }, { left: bottom(l), right: bottom(r) })
  }

  hunks.forEach((hunk, hi) => {
    const prev = hunks[hi - 1]
    // A side with zero lines in a hunk reports the line *before* the hunk as its start.
    const first = (start: number, count: number) => (count === 0 ? start + 1 : start)
    const hidden = prev
      ? first(hunk.old_start, hunk.old_lines) - (first(prev.old_start, prev.old_lines) + prev.old_lines)
      : first(hunk.old_start, hunk.old_lines) - 1
    if (hidden > 0) {
      left.push({ type: 'gap', key: `gap-${hi}`, hidden })
      right.push({ type: 'gap', key: `gap-${hi}`, hidden })
      pairEdges(left.length - 1, right.length - 1)
    }

    const lines = hunk.lines ?? []
    const line = (l: DiffLine, side: Side, block: number, changes?: Range[]): PaneItem => {
      const no = side === 'LEFT' ? l.old_no : l.new_no
      const content = side === 'LEFT' ? l.old_content ?? l.content : l.content
      return { type: 'line', key: `${side}:${no}`, no, content, hunk: hi, block, ...(changes && { changes }) }
    }

    let i = 0
    while (i < lines.length) {
      if (lines[i].type === 'context') {
        const l = lines[i++]
        left.push(line(l, 'LEFT', -1))
        right.push(line(l, 'RIGHT', -1))
        pairEdges(left.length - 1, right.length - 1)
        continue
      }
      const dels: model.DiffLineDTO[] = []
      const adds: model.DiffLineDTO[] = []
      while (i < lines.length && lines[i].type !== 'context') {
        const l = lines[i++]
        ;(l.type === 'delete' ? dels : adds).push(l)
      }
      const b = blocks.length
      const [delChanges, addChanges] = pairChanges(dels.map((l) => l.content), adds.map((l) => l.content))
      const fill = (pane: PaneItem[], side: Side, src: model.DiffLineDTO[], changes: (Range[] | undefined)[]): [Edge, Edge] => {
        const start = pane.length
        if (src.length) src.forEach((l, k) => pane.push(line(l, side, b, changes[k])))
        else pane.push({ type: 'mark', key: `mark-${b}`, block: b })
        return [top(start), bottom(pane.length - 1)]
      }
      const l = fill(left, 'LEFT', dels, delChanges)
      const r = fill(right, 'RIGHT', adds, addChanges)
      blocks.push({ kind: !dels.length ? 'added' : !adds.length ? 'deleted' : 'modified', left: l, right: r })
      sync.push({ left: l[0], right: r[0] }, { left: l[1], right: r[1] })
    }
  })

  return { left, right, blocks, sync }
}

/**
 * Maps `y` through piecewise-linear correspondence points sorted by their first
 * coordinate; outside the points it continues 1:1 from the nearest one.
 */
export function mapOffset(points: [number, number][], y: number): number {
  const n = points.length
  if (!n) return y
  if (y <= points[0][0]) return points[0][1] + (y - points[0][0])
  if (y >= points[n - 1][0]) return points[n - 1][1] + (y - points[n - 1][0])
  let lo = 0
  let hi = n - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (points[mid][0] <= y) lo = mid
    else hi = mid - 1
  }
  const [m0, s0] = points[lo]
  const [m1, s1] = points[lo + 1]
  return s0 + ((y - m0) / (m1 - m0)) * (s1 - s0)
}

/**
 * Where the follower pane should scroll when the driving pane is at `top`. Lines at
 * the middle of both viewports line up; over the first and last half-viewport of
 * scrolling the follower eases to its own top and bottom, so both panes reach their
 * ends together. The result never decreases as `top` grows, so the follower can't
 * bounce back while the driver passes lines the follower doesn't have.
 */
export function syncScrollTop(
  points: [number, number][],
  top: number,
  maxTop: number,
  followerMaxTop: number,
  viewport: number,
): number {
  const half = viewport / 2
  const middle = (t: number) => Math.max(0, Math.min(followerMaxTop, mapOffset(points, t + half) - half))
  if (maxTop <= 0) return 0
  const edge = Math.min(half, maxTop / 2)
  if (top <= edge) return middle(edge) * (Math.max(0, top) / edge)
  if (top >= maxTop - edge) {
    const from = middle(maxTop - edge)
    return from + (followerMaxTop - from) * (Math.min(top, maxTop) - (maxTop - edge)) / edge
  }
  return middle(top)
}

/** An unchanged line whose two sides differ only in whitespace carries the old side's text too. */
type DiffLine = model.DiffLineDTO & { old_content?: string }

const MAX_PAIRING_CELLS = 1_000_000

/**
 * Like `git diff -w`: a deleted and an added line that differ only in whitespace become one
 * unchanged line, and hunks left with no changes are dropped, unless every hunk is.
 */
export function hideWhitespaceChanges(hunks: model.DiffHunkDTO[]): model.DiffHunkDTO[] {
  const bare = (l: model.DiffLineDTO) => l.content.replace(/\s+/g, '')
  const all: model.DiffHunkDTO[] = []
  for (const hunk of hunks) {
    const src = hunk.lines ?? []
    const lines: DiffLine[] = []
    // Not push(...xs): a block can exceed the engine's argument limit.
    const append = (xs: DiffLine[]) => {
      for (const x of xs) lines.push(x)
    }
    let i = 0
    while (i < src.length) {
      if (src[i].type === 'context') {
        lines.push(src[i++])
        continue
      }
      const dels: model.DiffLineDTO[] = []
      const adds: model.DiffLineDTO[] = []
      while (i < src.length && src[i].type !== 'context') {
        const l = src[i++]
        ;(l.type === 'delete' ? dels : adds).push(l)
      }
      // shortcut: a huge change block is shown as is rather than paired, upgrade to Myers if that bites.
      if (dels.length * adds.length > MAX_PAIRING_CELLS) {
        append(dels)
        append(adds)
        continue
      }
      // lcs[d][a]: longest whitespace-equal pairing of dels[d..] with adds[a..].
      const a0 = dels.map(bare)
      const b0 = adds.map(bare)
      const lcs = Array.from({ length: dels.length + 1 }, () => new Array<number>(adds.length + 1).fill(0))
      for (let d = dels.length - 1; d >= 0; d--) {
        for (let a = adds.length - 1; a >= 0; a--) {
          lcs[d][a] = a0[d] === b0[a] ? lcs[d + 1][a + 1] + 1 : Math.max(lcs[d + 1][a], lcs[d][a + 1])
        }
      }
      let d = 0
      let a = 0
      const pendingAdds: model.DiffLineDTO[] = []
      while (d < dels.length || a < adds.length) {
        if (d < dels.length && a < adds.length && a0[d] === b0[a] && lcs[d][a] === lcs[d + 1][a + 1] + 1) {
          append(pendingAdds.splice(0))
          lines.push({ ...adds[a++], type: 'context' as model.DiffLineType, old_no: dels[d].old_no, old_content: dels[d++].content })
        } else if (d < dels.length && (a === adds.length || lcs[d + 1][a] >= lcs[d][a + 1])) {
          lines.push(dels[d++])
        } else {
          pendingAdds.push(adds[a++])
        }
      }
      append(pendingAdds)
    }
    all.push({ ...hunk, lines })
  }
  const changed = all.filter((h) => h.lines.some((l) => l.type !== 'context'))
  // A whitespace-only file keeps its lines, so comments on them stay in place.
  return changed.length ? changed : all
}

/** Every anchor key that maps to a visible line in the diff. */
export function diffLineKeys(hunks: model.DiffHunkDTO[]): Set<string> {
  const keys = new Set<string>()
  for (const hunk of hunks) {
    for (const l of hunk.lines ?? []) {
      if (l.type !== 'add') keys.add(anchorKey('LEFT', l.old_no))
      if (l.type !== 'delete') keys.add(anchorKey('RIGHT', l.new_no))
    }
  }
  return keys
}
