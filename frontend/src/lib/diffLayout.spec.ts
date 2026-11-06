import { describe, it, expect } from 'vitest'
import { buildDiffRows, diffLineKeys, type DiffRow } from './diffLayout'
import type { model } from '@/wailsjs/go/models'

const ctx = (o: number, n: number) => ({ type: 'context', old_no: o, new_no: n, content: `c${o}` })
const del = (o: number) => ({ type: 'delete', old_no: o, new_no: 0, content: `d${o}` })
const add = (n: number) => ({ type: 'add', old_no: 0, new_no: n, content: `a${n}` })

const hunk = (old_start: number, old_lines: number, new_start: number, new_lines: number, lines: object[]) =>
  ({ header: '', old_start, old_lines, new_start, new_lines, lines }) as model.DiffHunkDTO

const segments = (rows: DiffRow[]) => rows.flatMap((r) => (r.type === 'segment' ? [r.segment] : []))

describe('buildDiffRows', () => {
  // old: 10 c, 11 d, 12 d, 13 c   new: 10 c, 11 a, 12 c, then pure insert 13 a
  const h = hunk(10, 4, 10, 4, [ctx(10, 10), del(11), del(12), add(11), ctx(13, 12), add(13)])

  it('keeps sides unpadded and classifies change blocks', () => {
    const [seg] = segments(buildDiffRows([h], new Set()))
    expect(seg.left.map((l) => l.no)).toEqual([10, 11, 12, 13])
    expect(seg.right.map((l) => l.no)).toEqual([10, 11, 12, 13])
    expect(seg.blocks).toEqual([
      { kind: 'modified', left: [1, 3], right: [1, 2] },
      { kind: 'added', left: [4, 4], right: [3, 4] },
    ])
    expect(seg.left[1].block).toBe(0)
    expect(seg.left[0].block).toBe(-1)
  })

  it('splits a segment after an anchored line on either side', () => {
    const segs = segments(buildDiffRows([h], new Set(['LEFT:12', 'RIGHT:99'])))
    expect(segs).toHaveLength(2)
    expect(segs[0].anchors).toEqual(['LEFT:12'])
    expect(segs[0].left.map((l) => l.no)).toEqual([10, 11, 12])
    expect(segs[1].left.map((l) => l.no)).toEqual([13])
    expect(segs[1].blocks[0].left).toEqual([1, 1])
  })

  it('emits gaps for unchanged lines between and before hunks', () => {
    const rows = buildDiffRows([hunk(5, 1, 5, 1, [ctx(5, 5)]), hunk(20, 1, 20, 1, [ctx(20, 20)])], new Set())
    expect(rows.filter((r) => r.type === 'gap').map((r) => (r as { hidden: number }).hidden)).toEqual([4, 14])
  })

  it('handles added and removed files', () => {
    expect(buildDiffRows([hunk(0, 0, 1, 2, [add(1), add(2)])], new Set())[0].type).toBe('segment')
    const [seg] = segments(buildDiffRows([hunk(1, 2, 0, 0, [del(1), del(2)])], new Set()))
    expect(seg.blocks[0].kind).toBe('deleted')
  })
})

describe('diffLineKeys', () => {
  it('maps lines to their sides', () => {
    expect([...diffLineKeys([hunk(1, 2, 1, 2, [ctx(1, 1), del(2), add(2)])])].sort()).toEqual(
      ['LEFT:1', 'LEFT:2', 'RIGHT:1', 'RIGHT:2'],
    )
  })
})

describe('buildDiffRows block splitting', () => {
  it('breaks inside a change block so inline rows sit under their line', () => {
    const added = hunk(0, 0, 1, 4, [add(1), add(2), add(3), add(4)])
    const segs = segments(buildDiffRows([added], new Set(['RIGHT:2'])))
    expect(segs.map((s) => s.right.map((l) => l.no))).toEqual([[1, 2], [3, 4]])
    expect(segs[0].anchors).toEqual(['RIGHT:2'])
    expect(segs.map((s) => s.blocks)).toEqual([
      [{ kind: 'added', left: [0, 0], right: [0, 2] }],
      [{ kind: 'added', left: [0, 0], right: [0, 2] }],
    ])
    expect(new Set(segs.map((s) => s.key)).size).toBe(2)
  })

  it('pairs deleted and added rows when splitting a modified block', () => {
    const m = hunk(1, 2, 1, 3, [del(1), del(2), add(1), add(2), add(3)])
    const segs = segments(buildDiffRows([m], new Set(['RIGHT:1'])))
    expect(segs[0].left.map((l) => l.no)).toEqual([1])
    expect(segs[0].right.map((l) => l.no)).toEqual([1])
    expect(segs[1].blocks[0]).toEqual({ kind: 'modified', left: [0, 1], right: [0, 2] })
  })
})

describe('buildDiffRows gaps around empty sides', () => {
  it('counts unchanged lines correctly next to a pure deletion', () => {
    // old 1..3 unchanged, 4..5 deleted (new side "+3,0"), old 6..9 unchanged, hunk at old 10 / new 8
    const rows = buildDiffRows(
      [hunk(4, 2, 3, 0, [del(4), del(5)]), hunk(10, 1, 8, 1, [ctx(10, 8)])],
      new Set(),
    )
    expect(rows.filter((r) => r.type === 'gap').map((r) => (r as { hidden: number }).hidden)).toEqual([3, 4])
  })
})
