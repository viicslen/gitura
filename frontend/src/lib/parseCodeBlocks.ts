/**
 * parseCodeBlocks — splits a markdown comment body into alternating text and
 * fenced code block segments.
 *
 * Rules:
 *  - Any fenced block (``` or ~~~) produces a segment with type='code'.
 *  - Blocks with the language tag "run" (e.g. ```run) set isRun=true.
 *  - All other content is returned as type='text' segments.
 */

export interface TextSegment {
  type: 'text'
  content: string
}

export interface CodeSegment {
  type: 'code'
  lang: string
  content: string
  /** true when the fence tag is exactly "run" */
  isRun: boolean
}

export type BodySegment = TextSegment | CodeSegment

/**
 * Splits a markdown string into text and code-fence segments.
 * Preserves original whitespace within each segment.
 */
export function parseCodeBlocks(markdown: string): BodySegment[] {
  const segments: BodySegment[] = []
  // Matches opening fences of the form ``` or ~~~ with an optional language tag.
  // Captures: [1] fence char repeated ≥3 times, [2] optional language tag
  const fenceOpen = /^(`{3,}|~{3,})([^\n]*)\n/

  let remaining = markdown
  let textBuffer = ''

  while (remaining.length > 0) {
    const match = fenceOpen.exec(remaining)
    if (!match || match.index === undefined) {
      // No more fences — rest is plain text.
      textBuffer += remaining
      remaining = ''
      break
    }

    // Everything before this fence is text.
    textBuffer += remaining.slice(0, match.index)

    const fenceChars = match[1]   // e.g. "```" or "~~~"
    const langTag   = match[2].trim() // e.g. "ts", "run", ""
    const afterOpen = remaining.slice(match.index + match[0].length)

    // Find the matching closing fence (same number of chars, optional trailing spaces).
    const closeRe = new RegExp(`^${escapeRegExp(fenceChars)}[ \\t]*$`, 'm')
    const closeMatch = closeRe.exec(afterOpen)

    if (!closeMatch) {
      // Unclosed fence — treat remainder as text.
      textBuffer += match[0] + afterOpen
      remaining = ''
      break
    }

    // Flush text buffer.
    if (textBuffer) {
      segments.push({ type: 'text', content: textBuffer })
      textBuffer = ''
    }

    const codeContent = afterOpen.slice(0, closeMatch.index)
    segments.push({
      type: 'code',
      lang: langTag,
      content: codeContent.replace(/\n$/, ''), // strip single trailing newline
      isRun: langTag.toLowerCase() === 'run',
    })

    remaining = afterOpen.slice(closeMatch.index + closeMatch[0].length)
    // Consume the newline after the closing fence if present.
    if (remaining.startsWith('\n')) remaining = remaining.slice(1)
  }

  if (textBuffer) {
    segments.push({ type: 'text', content: textBuffer })
  }

  return segments
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
