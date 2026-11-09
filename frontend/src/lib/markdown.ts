import { marked } from 'marked'
import DOMPurify from 'dompurify'

export interface MarkdownRenderOptions {
  /** Page the markdown came from; relative links and images resolve against it. */
  baseUrl?: string
  repoOwner?: string
  repoName?: string
  repoRef?: string
  /** Enables task-list checkboxes and numbers them with data-task-index. */
  tasks?: boolean
}

const MARKDOWN_IMAGE_PROXY_PATH = '/__markdown-image'

function buildMarkdownImageProxyURL(params: {
  absoluteUrl?: string
  repoOwner?: string
  repoName?: string
  repoRef?: string
  repoPath?: string
}): string {
  const proxied = new URL(MARKDOWN_IMAGE_PROXY_PATH, 'http://localhost')
  if (params.absoluteUrl) proxied.searchParams.set('url', params.absoluteUrl)
  if (params.repoOwner) proxied.searchParams.set('owner', params.repoOwner)
  if (params.repoName) proxied.searchParams.set('repo', params.repoName)
  if (params.repoRef) proxied.searchParams.set('ref', params.repoRef)
  if (params.repoPath) proxied.searchParams.set('path', params.repoPath)
  return `${proxied.pathname}${proxied.search}`
}

function isProxyableMarkdownAssetURL(url: URL): boolean {
  const host = url.hostname.toLowerCase()
  return host === 'github.com'
    || host.endsWith('.githubusercontent.com')
    || host === 'storage.googleapis.com'
}

function resolveRepoImagePath(url: string, baseUrl?: string): string {
  const trimmed = url.trim()
  if (!trimmed) return ''
  if (trimmed.startsWith('/')) return trimmed.slice(1)

  const basePath = baseUrl ? new URL(baseUrl).pathname : '/'
  const baseDir = basePath.endsWith('/') ? basePath : `${basePath.substring(0, basePath.lastIndexOf('/') + 1)}`
  return new URL(trimmed, `https://github.local${baseDir}`).pathname.replace(/^\//, '')
}

function resolveURL(url: string, baseUrl?: string): string {
  if (!url || !baseUrl || url.startsWith('#')) return url
  try {
    return new URL(url, baseUrl).toString()
  } catch {
    return url
  }
}

function imageURL(rawSrc: string, opts: MarkdownRenderOptions): string {
  if (!rawSrc) return rawSrc
  const resolved = resolveURL(rawSrc, opts.baseUrl)
  try {
    const resolvedURL = new URL(resolved)
    return isProxyableMarkdownAssetURL(resolvedURL)
      ? buildMarkdownImageProxyURL({ absoluteUrl: resolvedURL.toString() })
      : resolvedURL.toString()
  } catch {
    // Not absolute: try repo-relative below.
  }
  if (opts.repoOwner && opts.repoName && opts.repoRef) {
    const repoPath = resolveRepoImagePath(rawSrc, opts.baseUrl)
    if (repoPath) {
      return buildMarkdownImageProxyURL({
        repoOwner: opts.repoOwner,
        repoName: opts.repoName,
        repoRef: opts.repoRef,
        repoPath,
      })
    }
  }
  return resolved
}

function srcsetURLs(srcset: string, opts: MarkdownRenderOptions): string {
  return srcset
    .split(',')
    .map((candidate) => {
      const [url, ...descriptor] = candidate.trim().split(/\s+/)
      return [imageURL(url, opts), ...descriptor].join(' ')
    })
    .join(', ')
}

const ICON = (paths: string) =>
  `<svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${paths}</svg>`

const ALERTS: Record<string, { title: string; icon: string }> = {
  note: { title: 'Note', icon: ICON('<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>') },
  tip: { title: 'Tip', icon: ICON('<path d="M15 14c.2-1 .7-1.7 1.5-2.5 1-.9 1.5-2.2 1.5-3.5A6 6 0 0 0 6 8c0 1 .2 2.2 1.5 3.5.7.7 1.3 1.5 1.5 2.5"/><path d="M9 18h6"/><path d="M10 22h4"/>') },
  important: { title: 'Important', icon: ICON('<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/><path d="M12 7v2"/><path d="M12 13h.01"/>') },
  warning: { title: 'Warning', icon: ICON('<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/>') },
  caution: { title: 'Caution', icon: ICON('<path d="M12 16h.01"/><path d="M12 8v4"/><path d="M15.312 2a2 2 0 0 1 1.414.586l4.688 4.688A2 2 0 0 1 22 8.688v6.624a2 2 0 0 1-.586 1.414l-4.688 4.688a2 2 0 0 1-1.414.586H8.688a2 2 0 0 1-1.414-.586l-4.688-4.688A2 2 0 0 1 2 15.312V8.688a2 2 0 0 1 .586-1.414l4.688-4.688A2 2 0 0 1 8.688 2z"/>') },
}

// GitHub only treats the marker as an alert when it sits alone on the quote's first line.
const ALERT_MARKER = /^\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\][ \t]*(?:\r?\n|$)/i

function renderAlerts(doc: Document): void {
  doc.querySelectorAll('blockquote').forEach((quote) => {
    const first = quote.firstElementChild
    const text = first?.tagName === 'P' ? first.firstChild : null
    if (!first || !text || text.nodeType !== Node.TEXT_NODE) return
    const match = (text.textContent ?? '').match(ALERT_MARKER)
    if (!match) return

    text.textContent = (text.textContent ?? '').slice(match[0].length)
    if (text.nextSibling?.nodeName === 'BR') text.nextSibling.remove()
    if (!first.textContent?.trim() && !first.querySelector('img')) first.remove()

    const kind = match[1].toLowerCase()
    const alert = doc.createElement('div')
    alert.className = `markdown-alert markdown-alert-${kind}`
    const title = doc.createElement('p')
    title.className = 'markdown-alert-title'
    title.innerHTML = `${ALERTS[kind].icon}${ALERTS[kind].title}`
    alert.append(title, ...Array.from(quote.childNodes))
    quote.replaceWith(alert)
  })
}

const TASK_BOX = 'li input[type="checkbox"]'

function markTaskLists(doc: Document, enable: boolean): void {
  doc.querySelectorAll<HTMLInputElement>(TASK_BOX).forEach((box, i) => {
    const item = box.closest('li')
    if (!item) return
    item.classList.add('task-list-item')
    item.parentElement?.classList.add('contains-task-list')
    if (!enable) return
    box.removeAttribute('disabled')
    box.dataset.taskIndex = String(i)
    box.setAttribute('aria-label', item.textContent?.trim() || 'Task')
  })
}

function taskStates(content: string): boolean[] {
  const doc = new DOMParser().parseFromString(renderGitHubMarkdown(content), 'text/html')
  return Array.from(doc.querySelectorAll<HTMLInputElement>(TASK_BOX), (box) => box.checked)
}

export function countTaskListItems(content: string): number {
  return taskStates(content).length
}

/**
 * Returns `content` with its index-th rendered checkbox set to `checked`, or null when that box
 * doesn't exist or already has that state. The renderer itself decides which `[ ]` is the box, so
 * look-alikes in code blocks, HTML blocks or link text are never touched.
 */
export function setTaskListItem(content: string, index: number, checked: boolean): string | null {
  const before = taskStates(content)
  if (before[index] === undefined || before[index] === checked) return null
  for (const match of content.matchAll(/\[[ xX]\]/g)) {
    const at = (match.index ?? 0) + 1
    const next = content.slice(0, at) + (checked ? 'x' : ' ') + content.slice(at + 1)
    if (next === content) continue
    const after = taskStates(next)
    if (after.length === before.length && after.every((state, i) => state === (i === index ? checked : before[i]))) {
      return next
    }
  }
  return null
}

/** Renders a GitHub comment body to sanitized HTML that matches how github.com displays it. */
export function renderGitHubMarkdown(content: string, opts: MarkdownRenderOptions = {}): string {
  if (!content) return ''
  const raw = marked.parse(content, { async: false }) as string
  const doc = new DOMParser().parseFromString(DOMPurify.sanitize(raw), 'text/html')

  doc.querySelectorAll('img').forEach((img) => {
    img.setAttribute('src', imageURL(img.getAttribute('src') ?? '', opts))
  })
  doc.querySelectorAll('source[srcset]').forEach((source) => {
    source.setAttribute('srcset', srcsetURLs(source.getAttribute('srcset') ?? '', opts))
  })
  doc.querySelectorAll('a[href]').forEach((link) => {
    link.setAttribute('href', resolveURL(link.getAttribute('href') ?? '', opts.baseUrl))
  })
  renderAlerts(doc)
  markTaskLists(doc, !!opts.tasks)

  return doc.body.innerHTML
}
