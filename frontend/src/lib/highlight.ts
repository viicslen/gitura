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
