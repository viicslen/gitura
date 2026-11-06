import { describe, it, expect } from 'vitest'
import { buildSplitLayout, diffLineKeys, mapOffset, syncScrollTop, type PaneItem } from './diffLayout'
import type { model } from '@/wailsjs/go/models'

const ctx = (o: number, n: number) => ({ type: 'context', old_no: o, new_no: n, content: `c${o}` })
const del = (o: number) => ({ type: 'delete', old_no: o, new_no: 0, content: `d${o}` })
const add = (n: number) => ({ type: 'add', old_no: 0, new_no: n, content: `a${n}` })

const hunk = (old_start: number, old_lines: number, new_start: number, new_lines: number, lines: object[]) =>
  ({ header: '', old_start, old_lines, new_start, new_lines, lines }) as model.DiffHunkDTO

const shape = (items: PaneItem[]) => items.map((i) => (i.type === 'line' ? i.no : i.type))

describe('buildSplitLayout', () => {
  // old: 10 c, 11 d, 12 d, 13 c   new: 10 c, 11 a, 12 c, then pure insert 13 a
  const h = hunk(10, 4, 10, 4, [ctx(10, 10), del(11), del(12), add(11), ctx(13, 12), add(13)])

  it('keeps each side unpadded and marks insertion points on the empty side', () => {
    const { left, right, blocks } = buildSplitLayout([h])
    expect(shape(left)).toEqual(['gap', 10, 11, 12, 13, 'mark'])
    expect(shape(right)).toEqual(['gap', 10, 11, 12, 13])
    expect(blocks.map((b) => b.kind)).toEqual(['modified', 'added'])
    expect(blocks[0].left).toEqual([{ item: 2, at: 'top' }, { item: 3, at: 'bottom' }])
    expect(blocks[0].right).toEqual([{ item: 2, at: 'top' }, { item: 2, at: 'bottom' }])
    expect(blocks[1].left).toEqual([{ item: 5, at: 'top' }, { item: 5, at: 'bottom' }])
  })

  it('pairs gaps, unchanged lines and block edges as sync points in order', () => {
    const { sync } = buildSplitLayout([h])
    const pairs = sync.map((p) => `${p.left.item}${p.left.at[0]}-${p.right.item}${p.right.at[0]}`)
    expect(pairs).toEqual([
      '0t-0t', '0b-0b', // gap
      '1t-1t', '1b-1b', // 10 | 10
      '2t-2t', '3b-2b', // modified block
      '4t-3t', '4b-3b', // 13 | 12
      '5t-4t', '5b-4b', // insertion
    ])
  })

  it('counts unchanged lines between hunks, including next to a pure deletion', () => {
    const { left } = buildSplitLayout([hunk(4, 2, 3, 0, [del(4), del(5)]), hunk(10, 1, 8, 1, [ctx(10, 8)])])
    expect(left.flatMap((i) => (i.type === 'gap' ? [i.hidden] : []))).toEqual([3, 4])
  })

  it('tags lines with their hunk for comment ranges', () => {
    const { right } = buildSplitLayout([hunk(1, 1, 1, 1, [ctx(1, 1)]), hunk(5, 1, 5, 1, [ctx(5, 5)])])
    expect(right.flatMap((i) => (i.type === 'line' ? [i.hunk] : []))).toEqual([0, 1])
  })
})

describe('mapOffset', () => {
  // Left 0–100 unchanged, then a 100px block on the left that is 20px on the right.
  const points: [number, number][] = [[0, 0], [100, 100], [200, 120], [300, 220]]

  it('moves 1:1 through unchanged stretches', () => {
    expect(mapOffset(points, 50)).toBe(50)
    expect(mapOffset(points, 250)).toBe(170)
  })

  it('moves the smaller side proportionally slower through a block', () => {
    expect(mapOffset(points, 150)).toBe(110)
  })

  it('jumps over a stretch that has no height on the driving side', () => {
    expect(mapOffset([[0, 0], [100, 100], [100, 180], [200, 280]], 100)).toBe(180)
  })

  it('continues 1:1 past the ends', () => {
    expect(mapOffset(points, 320)).toBe(240)
    expect(mapOffset([], 7)).toBe(7)
  })
})

describe('syncScrollTop', () => {
  // Left: 1000px unchanged, a 400px block the right side doesn't have, then 1000px unchanged.
  const leftToRight: [number, number][] = [[0, 0], [1000, 1000], [1400, 1000], [2400, 2000]]
  const rightToLeft = leftToRight.map(([l, r]): [number, number] => [r, l])
  const sweep = (points: [number, number][], max: number, followerMax: number) =>
    Array.from({ length: max + 1 }, (_, t) => syncScrollTop(points, t, max, followerMax, 600))

  it('never moves the follower backwards, whichever pane drives', () => {
    for (const ys of [sweep(leftToRight, 1800, 1400), sweep(rightToLeft, 1400, 1800)]) {
      expect(ys.every((y, i) => i === 0 || y >= ys[i - 1])).toBe(true)
    }
  })

  it('starts and ends both panes together', () => {
    const ys = sweep(leftToRight, 1800, 1400)
    expect([ys[0], ys[1800]]).toEqual([0, 1400])
  })

  it('lines up the middle of both viewports away from the ends', () => {
    // Left top 500 puts left 800 mid-viewport, which is unchanged and maps to right 800.
    expect(syncScrollTop(leftToRight, 500, 1800, 1400, 600)).toBe(500)
    // Mid-viewport inside the left-only block, the right holds at the block's insertion point.
    expect(syncScrollTop(leftToRight, 900, 1800, 1400, 600)).toBe(700)
    expect(syncScrollTop(leftToRight, 1000, 1800, 1400, 600)).toBe(700)
  })
})

describe('diffLineKeys', () => {
  it('maps lines to their sides', () => {
    expect([...diffLineKeys([hunk(1, 2, 1, 2, [ctx(1, 1), del(2), add(2)])])].sort()).toEqual(
      ['LEFT:1', 'LEFT:2', 'RIGHT:1', 'RIGHT:2'],
    )
  })
})
