/** Half-open `[start, end)` character range of a line. */
export type Range = [number, number]

const TOKEN = /\w+|\s+|[^\w\s]/g
// Past this many differing tokens per side the whole middle is marked instead of running the quadratic LCS.
const MAX_TOKENS = 100
// Larger blocks pair lines by position instead of trying every pair.
const MAX_PAIRS = 400
// Share of non-whitespace characters two lines must have in common to be compared word by word.
const MIN_SIMILARITY = 0.4

function changedRanges(line: string, tokens: string[], kept: boolean[]): Range[] {
  const out: Range[] = []
  let pos = 0
  tokens.forEach((t, i) => {
    const end = pos + t.length
    if (!kept[i]) {
      const last = out[out.length - 1]
      // Changes split only by whitespace read as one change.
      if (last && !line.slice(last[1], pos).trim()) last[1] = end
      else out.push([pos, end])
    }
    pos = end
  })
  return out
}

const solid = (t: string) => (/\s/.test(t[0]) ? 0 : t.length)

/** Word-level diff of two lines: the changed ranges on each side and how similar the lines are (0–1). */
export function inlineDiff(a: string, b: string): { a: Range[]; b: Range[]; similarity: number } {
  const ta = a.match(TOKEN) ?? []
  const tb = b.match(TOKEN) ?? []
  const keptA = ta.map(() => false)
  const keptB = tb.map(() => false)

  let s = 0
  while (s < ta.length && s < tb.length && ta[s] === tb[s]) keptA[s] = keptB[s++] = true
  let ea = ta.length
  let eb = tb.length
  while (ea > s && eb > s && ta[ea - 1] === tb[eb - 1]) keptA[--ea] = keptB[--eb] = true

  const n = ea - s
  const m = eb - s
  if (n && m && n <= MAX_TOKENS && m <= MAX_TOKENS) {
    // Suffix LCS lengths, so the walk below can go forward.
    const w = m + 1
    const lcs = new Uint8Array((n + 1) * w)
    for (let i = n - 1; i >= 0; i--)
      for (let j = m - 1; j >= 0; j--)
        lcs[i * w + j] = ta[s + i] === tb[s + j] ? lcs[(i + 1) * w + j + 1] + 1 : Math.max(lcs[(i + 1) * w + j], lcs[i * w + j + 1])
    for (let i = 0, j = 0; i < n && j < m; ) {
      if (ta[s + i] === tb[s + j]) keptA[s + i++] = keptB[s + j++] = true
      else if (lcs[(i + 1) * w + j] >= lcs[i * w + j + 1]) i++
      else j++
    }
  }

  const total = ta.reduce((t, x) => t + solid(x), 0) + tb.reduce((t, x) => t + solid(x), 0)
  const common = ta.reduce((t, x, i) => t + (keptA[i] ? solid(x) : 0), 0)
  return {
    a: changedRanges(a, ta, keptA),
    b: changedRanges(b, tb, keptB),
    similarity: total ? (2 * common) / total : 1,
  }
}

/**
 * Pairs each removed line with the most similar added line after the previous pair and returns
 * the changed ranges of every paired line; lines too unlike any other stay unpaired (undefined).
 */
export function pairChanges(dels: string[], adds: string[]): [(Range[] | undefined)[], (Range[] | undefined)[]] {
  const left: (Range[] | undefined)[] = dels.map(() => undefined)
  const right: (Range[] | undefined)[] = adds.map(() => undefined)
  const everyPair = dels.length * adds.length <= MAX_PAIRS
  let next = 0
  dels.forEach((d, i) => {
    let best: ReturnType<typeof inlineDiff> | null = null
    let at = -1
    for (let j = everyPair ? next : Math.max(next, i); j < (everyPair ? adds.length : Math.min(adds.length, i + 1)); j++) {
      const r = inlineDiff(d, adds[j])
      if (r.similarity >= MIN_SIMILARITY && (!best || r.similarity > best.similarity)) {
        best = r
        at = j
      }
    }
    if (!best) return
    left[i] = best.a
    right[at] = best.b
    next = at + 1
  })
  return [left, right]
}

/** Wraps the given text ranges of highlighted line HTML in `<mark>`, splitting marks around the highlighter's tags. */
export function markRanges(html: string, ranges: Range[] | undefined, cls: string): string {
  if (!ranges?.length) return html
  const open = `<mark class="${cls}">`
  let out = ''
  let pos = 0
  let r = 0
  let marking = false
  for (let i = 0; i < html.length; ) {
    if (marking && pos === ranges[r][1]) {
      out += '</mark>'
      marking = false
      r++
    }
    if (html[i] === '<') {
      const end = html.indexOf('>', i) + 1 || html.length
      out += marking ? `</mark>${html.slice(i, end)}${open}` : html.slice(i, end)
      i = end
      continue
    }
    if (!marking && r < ranges.length && pos === ranges[r][0]) {
      out += open
      marking = true
    }
    const end = (html[i] === '&' && html.indexOf(';', i) + 1) || i + 1
    out += html.slice(i, end)
    i = end
    pos++
  }
  return marking ? out + '</mark>' : out
}
