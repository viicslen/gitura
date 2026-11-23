import { describe, it, expect } from 'vitest'
import { currentCode } from './currentCode'

const file = Array.from({ length: 30 }, (_, i) => `line ${i + 1}`).join('\n') + '\n'
const hunk = '@@ -8,3 +8,3 @@\n line 8\n-old 9\n+line 10'

describe('currentCode', () => {
  it('marks the commented line where it sits now, with context around it', () => {
    const shifted = 'inserted\n'.repeat(4) + file
    const { hunk: out, markedLine } = currentCode(shifted, hunk, 10)
    const rows = out.split('\n')
    expect(rows[0]).toBe('@@ +9,11 @@')
    expect(rows[markedLine ?? 0]).toBe(' line 10')
  })

  it('picks the match nearest the old line when the text repeats', () => {
    const repeated = 'x\nreturn;\nx\nx\nx\nx\nx\nx\nx\nreturn;\nx\n'
    const { hunk: out, markedLine } = currentCode(repeated, '@@ -1 +1 @@\n return;', 9)
    expect(out.split('\n')[0]).toBe('@@ +5,7 @@')
    expect(markedLine).toBe(6)
  })

  it('does not match a line of only punctuation', () => {
    const { markedLine } = currentCode('a\n}\nb', '@@ -1 +1 @@\n }', 2)
    expect(markedLine).toBeUndefined()
  })

  it('shows the code around the old line when the commented line is gone', () => {
    const { hunk: out, markedLine } = currentCode(file, '@@ -1 +1 @@\n+removed', 20)
    expect(markedLine).toBeUndefined()
    expect(out.split('\n')[0]).toBe('@@ +15,11 @@')
  })

  it('stays inside a file shorter than the old line', () => {
    const { hunk: out } = currentCode('a\nb', '@@ -1 +1 @@\n+gone', 50)
    expect(out).toBe('@@ +1,2 @@\n a\n b')
  })
})
