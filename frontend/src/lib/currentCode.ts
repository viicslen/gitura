/** A window of a file as it is now, as a context-only hunk that CommentDiffHunk can show. */
export interface CurrentCode {
  hunk: string
  /** 1-based line in the hunk holding the commented line; unset when that line is no longer in the file. */
  markedLine?: number
}

const CONTEXT = 5

/**
 * Finds where an outdated thread's line sits in the file now: the line of the same text nearest its old number,
 * since GitHub stops tracking an outdated thread's position. Without a match it shows the code around the old number.
 */
export function currentCode(file: string, diffHunk: string, line: number): CurrentCode {
  const lines = file.replace(/\r?\n$/, '').split(/\r?\n/)
  // GitHub ends a comment's diff hunk at the commented line.
  const commented = diffHunk.split('\n').filter((l) => l && !l.startsWith('@@') && !l.startsWith('\\')).at(-1)?.slice(1).trim() ?? ''
  const old = line - 1
  let at = -1
  // A line of only punctuation, such as `}`, recurs too often to find the same line again.
  if (/\w/.test(commented)) {
    lines.forEach((l, i) => {
      if (l.trim() === commented && (at < 0 || Math.abs(i - old) < Math.abs(at - old))) at = i
    })
  }
  const center = at >= 0 ? at : Math.min(Math.max(old, 0), lines.length - 1)
  const start = Math.max(0, center - CONTEXT)
  const window = lines.slice(start, center + CONTEXT + 1)
  return {
    hunk: [`@@ +${start + 1},${window.length} @@`, ...window.map((l) => ` ${l}`)].join('\n'),
    markedLine: at >= 0 ? at - start + 1 : undefined,
  }
}
