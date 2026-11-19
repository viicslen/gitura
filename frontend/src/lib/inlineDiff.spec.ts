import { describe, it, expect } from 'vitest'
import { inlineDiff, markRanges, pairChanges, type Range } from './inlineDiff'
import { highlightLine } from './highlight'

const pick = (s: string, ranges: Range[] | undefined) => (ranges ?? []).map(([a, b]) => s.slice(a, b))

describe('inlineDiff', () => {
  it('marks only the words that changed, joining changes split by whitespace', () => {
    const a = '    public function handle(AbstractResponseType $response): bool'
    const b = '    public function handle(SubmissionResult|ValidationResult $result): bool'
    const d = inlineDiff(a, b)
    expect(pick(a, d.a)).toEqual(['AbstractResponseType', 'response'])
    expect(pick(b, d.b)).toEqual(['SubmissionResult|ValidationResult', 'result'])
    expect(d.similarity).toBeGreaterThan(0.4)
  })

  it('marks a pure insertion on one side only', () => {
    const d = inlineDiff('foo(a)', 'foo(a, b)')
    expect(d.a).toEqual([])
    expect(pick('foo(a, b)', d.b)).toEqual([', b'])
  })
})

describe('pairChanges', () => {
  it('pairs similar lines and leaves unrelated rewrites unmarked', () => {
    const dels = ['use DTS\\eBaySDK\\Trading\\Types\\ErrorType;', 'use MyListerHub\\Core\\Concerns\\Actions\\AsAction;']
    const adds = ['use App\\Actions\\Listing\\HandleListingErrors;', 'use MyListerHub\\Core\\Concerns\\Actions\\AsAction;']
    const [l, r] = pairChanges(dels, adds)
    expect(l).toEqual([undefined, []])
    expect(r).toEqual([undefined, []])
  })
})

describe('markRanges', () => {
  it('splits marks around highlighter tags and counts entities as one character', () => {
    const line = 'if (a < b) return "x"'
    const html = highlightLine(line, 'typescript')
    const start = line.indexOf('<')
    const out = markRanges(html, [[start, line.length]], 'm')
    expect(out.replace(/<mark class="m">|<\/mark>/g, '')).toBe(html)
    const marked = [...out.matchAll(/<mark class="m">(.*?)<\/mark>/g)].map((m) => m[1].replace(/<[^>]+>/g, '')).join('')
    expect(marked).toBe('&lt; b) return &quot;x&quot;')
  })
})
