<script setup lang="ts">
import { computed } from 'vue'
import hljs from 'highlight.js/lib/common'

const props = defineProps<{
  diffHunk: string
  /**
   * Absolute new-file line number that anchors the comment (GitHub's thread.line).
   * This is translated internally to a hunk-relative position; the caller does
   * not need to parse the @@ header.
   */
  absoluteEndLine?: number
  /**
   * First line of a multi-line comment range (GitHub's thread.start_line).
   * When set, all lines from absoluteStartLine to absoluteEndLine are highlighted.
   * Defaults to absoluteEndLine when omitted (single-line highlight).
   */
  absoluteStartLine?: number
  /** hljs language id derived from the file extension (e.g. "typescript"). */
  language?: string
}>()

interface DiffLine {
  prefix: string
  highlighted: string  // hljs-escaped HTML for the code portion
  kind: 'add' | 'remove' | 'context' | 'header'
  /** Absolute old-file line number for this line; 0 for header and addition lines. */
  oldFileLineNum: number
  /** Absolute new-file line number for this line; 0 for header and removal lines. */
  newFileLineNum: number
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

/**
 * Parses the @@ header of a diff hunk and returns the starting line numbers for
 * both the old and new files.  For "@@ -10,7 +12,8 @@" this returns { oldStart: 10, newStart: 12 }.
 */
function parseHunkStarts(hunk: string): { oldStart: number; newStart: number } {
  const m = hunk.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/)
  return m
    ? { oldStart: parseInt(m[1], 10), newStart: parseInt(m[2], 10) }
    : { oldStart: 1, newStart: 1 }
}

const lines = computed<DiffLine[]>(() => {
  if (!props.diffHunk) return []
  const lang = props.language && hljs.getLanguage(props.language) ? props.language : 'plaintext'
  const { oldStart, newStart } = parseHunkStarts(props.diffHunk)
  let oldFileLineNum = oldStart - 1
  let newFileLineNum = newStart - 1

  return props.diffHunk.split('\n').map((raw) => {
    if (raw.startsWith('@@')) {
      return { prefix: '', highlighted: escapeHtml(raw), kind: 'header' as const, oldFileLineNum: 0, newFileLineNum: 0 }
    }
    const prefix = raw[0] === '+' || raw[0] === '-' ? raw[0] : ' '
    const code = raw.slice(1)
    const kind: DiffLine['kind'] = prefix === '+' ? 'add' : prefix === '-' ? 'remove' : 'context'
    let highlighted: string
    try {
      highlighted = hljs.highlight(code, { language: lang }).value
    } catch {
      highlighted = escapeHtml(code)
    }
    // Advance the relevant counter(s) depending on which side of the diff the line belongs to.
    let oldNum = 0
    let newNum = 0
    if (kind === 'add') {
      newNum = ++newFileLineNum
    } else if (kind === 'remove') {
      oldNum = ++oldFileLineNum
    } else {
      // context line: present in both old and new files
      oldNum = ++oldFileLineNum
      newNum = ++newFileLineNum
    }
    return { prefix, highlighted, kind, oldFileLineNum: oldNum, newFileLineNum: newNum }
  })
})

/** Returns true if the given DiffLine falls within the highlighted range. */
function isHighlighted(line: DiffLine): boolean {
  if (props.absoluteEndLine === undefined || line.newFileLineNum === 0) return false
  const rangeStart = props.absoluteStartLine ?? props.absoluteEndLine
  return line.newFileLineNum >= rangeStart && line.newFileLineNum <= props.absoluteEndLine
}
</script>

<template>
  <div aria-label="Diff context" class="rounded-md overflow-hidden border border-border text-xs font-mono">
    <pre class="overflow-x-auto m-0 p-0 leading-5"><template
      v-for="(line, i) in lines"
      :key="i"
    ><span
        :class="[
          'flex min-w-0',
          line.kind === 'header'   ? 'bg-muted text-muted-foreground'
          : line.kind === 'add'    ? 'bg-[#f0fff4] dark:bg-[#1b4721]'
          : line.kind === 'remove' ? 'bg-[#ffeef0] dark:bg-[#78191b]'
          : '',
          isHighlighted(line)
            ? 'border-l-2 border-yellow-400'
            : 'border-l-2 border-transparent',
        ]"
      ><span
          class="select-none text-muted-foreground text-right shrink-0 w-10 px-1 border-r border-border"
        >{{ line.oldFileLineNum || '' }}</span><span
          class="select-none text-muted-foreground text-right shrink-0 w-10 px-1 border-r border-border"
        >{{ line.newFileLineNum || '' }}</span><span
          :class="[
            'select-none px-2',
            line.kind === 'add'    ? 'text-green-600 dark:text-green-400'
            : line.kind === 'remove' ? 'text-red-500 dark:text-red-400'
            : 'text-muted-foreground',
          ]"
        >{{ line.prefix }}</span><!-- --><span class="pr-3" v-html="line.highlighted" /></span></template></pre>
  </div>
</template>
