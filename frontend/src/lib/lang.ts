/** Maps common file extensions to highlight.js language identifiers. */
const EXT_LANG: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
  vue: 'xml', py: 'python', rb: 'ruby', go: 'go', rs: 'rust',
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
