<script setup lang="ts">
import { computed } from 'vue'
import hljs from 'highlight.js/lib/common'

const props = defineProps<{
  diffHunk: string
  /** 1-based index of the non-header line to highlight (the changed line). */
  highlightLine?: number
  /** hljs language id derived from the file extension (e.g. "typescript"). */
  language?: string
}>()

interface DiffLine {
  prefix: string
  highlighted: string  // hljs-escaped HTML for the code portion
  kind: 'add' | 'remove' | 'context' | 'header'
  contentIndex: number
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

const lines = computed<DiffLine[]>(() => {
  if (!props.diffHunk) return []
  const lang = props.language && hljs.getLanguage(props.language) ? props.language : 'plaintext'
  let contentIndex = 0

  return props.diffHunk.split('\n').map((raw) => {
    if (raw.startsWith('@@')) {
      return { prefix: '', highlighted: escapeHtml(raw), kind: 'header' as const, contentIndex: 0 }
    }
    contentIndex++
    const prefix = raw[0] === '+' || raw[0] === '-' ? raw[0] : ' '
    const code = raw.slice(1)
    const kind: DiffLine['kind'] = prefix === '+' ? 'add' : prefix === '-' ? 'remove' : 'context'
    let highlighted: string
    try {
      highlighted = hljs.highlight(code, { language: lang }).value
    } catch {
      highlighted = escapeHtml(code)
    }
    return { prefix, highlighted, kind, contentIndex }
  })
})
</script>

<template>
  <div aria-label="Diff context" class="rounded-md overflow-hidden border border-border text-xs font-mono">
    <pre class="overflow-x-auto m-0 p-0 leading-5"><template
      v-for="(line, i) in lines"
      :key="i"
    ><span
        :class="[
          'block px-3 min-w-0',
          line.kind === 'header'   ? 'bg-muted text-muted-foreground'
          : line.kind === 'add'    ? 'bg-[#f0fff4] dark:bg-[#1b4721]'
          : line.kind === 'remove' ? 'bg-[#ffeef0] dark:bg-[#78191b]'
          : '',
          highlightLine !== undefined && line.contentIndex === highlightLine
            ? 'border-l-2 border-yellow-400'
            : 'border-l-2 border-transparent',
        ]"
      ><span
          :class="[
            'select-none',
            line.kind === 'add'    ? 'text-green-600 dark:text-green-400'
            : line.kind === 'remove' ? 'text-red-500 dark:text-red-400'
            : 'text-muted-foreground',
          ]"
        >{{ line.prefix }}</span><!-- --><span v-html="line.highlighted" /></span></template></pre>
  </div>
</template>
