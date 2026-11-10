import hljs from 'highlight.js/lib/common'

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

// Diff segments are rebuilt whenever an inline row opens or closes; sharing results avoids re-highlighting the file.
const cache = new Map<string, string>()

/** Highlights one line of code, returning HTML-escaped markup. */
export function highlightLine(content: string, language: string): string {
  const lang = hljs.getLanguage(language) ? language : 'plaintext'
  const key = `${lang}\0${content}`
  let html = cache.get(key)
  if (html === undefined) {
    try {
      html = hljs.highlight(content, { language: lang, ignoreIllegals: true }).value
    } catch {
      html = escapeHtml(content)
    }
    if (cache.size > 20000) cache.clear()
    cache.set(key, html)
  }
  return html
}

// Top-level single-file-component blocks start at column 0; nested <template> tags are indented.
const SFC_OPEN = /^<(script|style|template)\b([^>]*)>/i
const SFC_CLOSE = /^<\/(script|style|template)>/i

function sfcBlockLanguage(tag: string, attrs = ''): string {
  const lang = /\blang=["']?(\w+)/i.exec(attrs)?.[1]?.toLowerCase()
  if (tag.toLowerCase() === 'template') return 'xml'
  if (tag.toLowerCase() === 'script') return lang === 'js' || lang === 'jsx' ? 'javascript' : 'typescript'
  return lang === 'scss' || lang === 'sass' ? 'scss' : lang === 'less' ? 'less' : 'css'
}

// A diff shows slices of the file, so the block a slice starts in is inferred: the first
// top-level closing tag ahead names it, then markup in the slice, then the block the previous
// slice ended in, then the code.
function guessSfcStart(lines: string[], carried?: string): string {
  for (const line of lines) {
    const close = SFC_CLOSE.exec(line)
    if (close) return sfcBlockLanguage(close[1])
    if (SFC_OPEN.test(line)) return 'xml'
  }
  if (lines.some((l) => /^\s*<\/?[a-zA-Z][\w-]*(?:[\s>/]|$)/.test(l))) return 'xml'
  if (carried) return carried
  const scriptHint = /^\s*(import |export |const |let |var |function |async |await |return |if \(|for \(|\/\/|}\)?;?$|defineProps|defineEmits)|=>|\.value\b|:\s*(string|number|boolean)\b/
  if (lines.some((l) => scriptHint.test(l))) return 'typescript'
  if (lines.some((l) => /^\s*[\w-]+\s*:\s*[^;]+;\s*$/.test(l) && !/[<>]/.test(l))) return 'css'
  return 'xml'
}

/**
 * Language for each line of one contiguous run of code. Vue/Svelte files mix markup, script and
 * styles, so each line is highlighted in the language of the block it sits in.
 */
export function lineLanguages(lines: string[], language: string, carry: { mode?: string } = {}): string[] {
  if (language !== 'vue') return lines.map(() => language)
  let mode = guessSfcStart(lines, carry.mode)
  const langs = lines.map((line) => {
    const open = SFC_OPEN.exec(line)
    if (open) {
      mode = sfcBlockLanguage(open[1], open[2])
      return 'xml'
    }
    if (SFC_CLOSE.test(line)) {
      mode = 'xml'
      return 'xml'
    }
    return mode
  })
  carry.mode = mode
  return langs
}

/** Maps common file extensions to highlight.js language identifiers. */
const EXT_LANG: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
  vue: 'vue', svelte: 'vue', py: 'python', rb: 'ruby', go: 'go', rs: 'rust',
  java: 'java', cs: 'csharp', cpp: 'cpp', c: 'c', h: 'c',
  php: 'php', html: 'html', css: 'css', scss: 'scss',
  json: 'json', yaml: 'yaml', yml: 'yaml', sh: 'bash', bash: 'bash',
  md: 'markdown', sql: 'sql', xml: 'xml', swift: 'swift',
  kt: 'kotlin', kts: 'kotlin', toml: 'ini',
}

/** Derives an hljs language id from a file path by inspecting its extension. */
export function langFromPath(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase() ?? ''
  return EXT_LANG[ext] ?? 'plaintext'
}
