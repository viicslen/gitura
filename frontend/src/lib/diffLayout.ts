import type { model } from '@/wailsjs/go/models'

export type Side = 'LEFT' | 'RIGHT'
export type BlockKind = 'added' | 'deleted' | 'modified'

export interface PaneLine {
  no: number
  content: string
  /** Index into the segment's blocks; -1 for unchanged lines. */
  block: number
}

export interface ChangeBlock {
  kind: BlockKind
  /** Half-open line-index ranges within the segment's panes. */
  left: [number, number]
  right: [number, number]
}

export interface DiffSegment {
  key: string
  /** Index of the hunk this segment came from; a comment range must stay within one hunk. */
  hunk: number
  left: PaneLine[]
  right: PaneLine[]
  blocks: ChangeBlock[]
  /** Anchor keys whose inline rows (threads, drafts, comment form) render right after this segment. */
  anchors: string[]
}

export interface LineSelection {
  side: Side
  from: number
  to: number
}

export type DiffRow =
  | { type: 'gap'; key: string; hidden: number }
  | { type: 'segment'; segment: DiffSegment }

export function anchorKey(side: Side, line: number): string {
  return `${side}:${line}`
}

/**
 * Lays hunks out JetBrains-style: each side lists its lines without filler rows and
 * change blocks are mapped across by connectors. Both sides only re-align at segment
 * ends, so a segment is closed after any line whose anchor key is in `breakAt` to give
 * full-width inline rows a common baseline.
 */
export function buildDiffRows(hunks: model.DiffHunkDTO[], breakAt: Set<string>): DiffRow[] {
  const rows: DiffRow[] = []

  hunks.forEach((hunk, hi) => {
    const prev = hunks[hi - 1]
    // A side with zero lines in a hunk reports the line *before* the hunk as its start.
    const first = (start: number, count: number) => (count === 0 ? start + 1 : start)
    const hidden = prev
      ? first(hunk.old_start, hunk.old_lines) - (first(prev.old_start, prev.old_lines) + prev.old_lines)
      : first(hunk.old_start, hunk.old_lines) - 1
    if (hidden > 0) rows.push({ type: 'gap', key: `gap-${hi}`, hidden })

    let seg: DiffSegment | null = null
    const close = (): void => {
      if (seg && (seg.left.length || seg.right.length)) {
        seg.key = `${hi}:${seg.left[0]?.no ?? ''}:${seg.right[0]?.no ?? ''}`
        rows.push({ type: 'segment', segment: seg })
      }
      seg = null
    }

    const lines = hunk.lines ?? []
    let i = 0
    // Each unit is one context line, or one row of a change block (its j-th deleted and j-th added line).
    while (i < lines.length) {
      if (lines[i].type === 'context') {
        const l = lines[i++]
        place([l], [l], false)
        continue
      }
      const dels: model.DiffLineDTO[] = []
      const adds: model.DiffLineDTO[] = []
      while (i < lines.length && lines[i].type !== 'context') {
        const l = lines[i++]
        ;(l.type === 'delete' ? dels : adds).push(l)
      }
      for (let j = 0; j < Math.max(dels.length, adds.length); j++) {
        place(dels.slice(j, j + 1), adds.slice(j, j + 1), true, j === 0)
      }
    }
    close()

    function place(left: model.DiffLineDTO[], right: model.DiffLineDTO[], change: boolean, newBlock = false): void {
      seg ??= { key: '', hunk: hi, left: [], right: [], blocks: [], anchors: [] }
      const s: DiffSegment = seg
      let block = -1
      if (change) {
        const last = s.blocks[s.blocks.length - 1]
        const continues = !newBlock && last && last.left[1] === s.left.length && last.right[1] === s.right.length
        if (!continues) s.blocks.push({ kind: 'modified', left: [s.left.length, s.left.length], right: [s.right.length, s.right.length] })
        block = s.blocks.length - 1
        const b = s.blocks[block]
        b.left[1] += left.length
        b.right[1] += right.length
        b.kind = b.left[0] === b.left[1] ? 'added' : b.right[0] === b.right[1] ? 'deleted' : 'modified'
      }
      const keys: string[] = []
      for (const l of left) {
        s.left.push({ no: l.old_no, content: l.content, block })
        keys.push(anchorKey('LEFT', l.old_no))
      }
      for (const l of right) {
        s.right.push({ no: l.new_no, content: l.content, block })
        keys.push(anchorKey('RIGHT', l.new_no))
      }
      const hits = keys.filter((k) => breakAt.has(k))
      if (hits.length) {
        s.anchors.push(...new Set(hits))
        close()
      }
    }
  })

  return rows
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
