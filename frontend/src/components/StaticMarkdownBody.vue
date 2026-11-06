<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

const props = defineProps<{
  content: string
  baseUrl?: string
  repoOwner?: string
  repoName?: string
  repoRef?: string
}>()

const MARKDOWN_IMAGE_PROXY_PATH = '/__markdown-image'

function buildMarkdownImageProxyURL(params: {
  absoluteUrl?: string
  repoOwner?: string
  repoName?: string
  repoRef?: string
  repoPath?: string
}): string {
  const proxied = new URL(MARKDOWN_IMAGE_PROXY_PATH, 'http://localhost')
  if (params.absoluteUrl) {
    proxied.searchParams.set('url', params.absoluteUrl)
  }
  if (params.repoOwner) {
    proxied.searchParams.set('owner', params.repoOwner)
  }
  if (params.repoName) {
    proxied.searchParams.set('repo', params.repoName)
  }
  if (params.repoRef) {
    proxied.searchParams.set('ref', params.repoRef)
  }
  if (params.repoPath) {
    proxied.searchParams.set('path', params.repoPath)
  }
  return `${proxied.pathname}${proxied.search}`
}

function isGitHubImageURL(url: URL): boolean {
  const host = url.hostname.toLowerCase()
  return host === 'github.com'
    || host === 'raw.githubusercontent.com'
    || host === 'media.githubusercontent.com'
    || host.endsWith('.githubusercontent.com')
}

function isProxyableMarkdownAssetURL(url: URL): boolean {
  const host = url.hostname.toLowerCase()
  return isGitHubImageURL(url) || host === 'storage.googleapis.com'
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
  if (!url) return url
  if (!baseUrl) return url
  if (url.startsWith('#')) return url

  try {
    return new URL(url, baseUrl).toString()
  } catch {
    return url
  }
}

const html = computed(() => {
  if (!props.content) return ''
  const raw = marked.parse(props.content, { async: false }) as string
  const sanitized = DOMPurify.sanitize(raw)
  const doc = new DOMParser().parseFromString(sanitized, 'text/html')
  doc.querySelectorAll<HTMLImageElement>('img').forEach((img) => {
    const rawSrc = img.getAttribute('src') ?? ''
    if (!rawSrc) return

    const resolved = resolveURL(rawSrc, props.baseUrl)
    try {
      const resolvedURL = new URL(resolved)
      if (isProxyableMarkdownAssetURL(resolvedURL)) {
        img.src = buildMarkdownImageProxyURL({ absoluteUrl: resolvedURL.toString() })
        return
      }

      img.src = resolvedURL.toString()
      return
    } catch {
      // Fall through to repo-relative handling.
    }

    if (props.repoOwner && props.repoName && props.repoRef) {
      const repoPath = resolveRepoImagePath(rawSrc, props.baseUrl)
      if (repoPath) {
        img.src = buildMarkdownImageProxyURL({
          repoOwner: props.repoOwner,
          repoName: props.repoName,
          repoRef: props.repoRef,
          repoPath,
        })
        return
      }
    }

    img.src = resolved
  })
  doc.querySelectorAll<HTMLAnchorElement>('a').forEach((link) => {
    link.href = resolveURL(link.getAttribute('href') ?? '', props.baseUrl)
  })
  return doc.body.innerHTML
})
</script>

<template>
  <div class="markdown-body text-sm text-foreground" v-html="html" />
</template>

<style scoped>
.markdown-body :deep(> :first-child) {
  margin-top: 0;
}

.markdown-body :deep(> :last-child) {
  margin-bottom: 0;
}

.markdown-body :deep(p) {
  margin: 0.4em 0;
}
.markdown-body :deep(p:first-child) {
  margin-top: 0;
}
.markdown-body :deep(p:last-child) {
  margin-bottom: 0;
}
.markdown-body :deep(a) {
  color: var(--color-primary);
  text-decoration: underline;
  cursor: pointer;
}
.markdown-body :deep(img) {
  display: block;
  max-width: 100%;
  height: auto;
}
.markdown-body :deep(summary) {
  cursor: pointer;
  user-select: none;
}
.markdown-body :deep(details) {
  cursor: default;
}
.markdown-body :deep(code) {
  font-family: ui-monospace, monospace;
  font-size: 0.85em;
  background: var(--color-muted);
  border: 1px solid var(--color-border);
  border-radius: 3px;
  padding: 0.1em 0.35em;
}
.markdown-body :deep(pre) {
  background: var(--color-muted);
  border: 1px solid var(--color-border);
  border-radius: 6px;
  padding: 0.75rem 1rem;
  overflow-x: auto;
  font-size: 0.8em;
  margin: 0.5em 0;
}
.markdown-body :deep(pre code) {
  background: transparent;
  border: none;
  padding: 0;
  display: block;
  overflow-x: auto;
}
.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 0.4em 0;
  padding-left: 1.4em;
}
.markdown-body :deep(li) {
  margin: 0.15em 0;
}
.markdown-body :deep(blockquote) {
  margin: 0.5em 0;
  padding-left: 0.9em;
  border-left: 3px solid var(--color-border);
  color: var(--color-muted-foreground);
}
.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4) {
  margin: 0.75em 0 0.35em;
  font-weight: 600;
  line-height: 1.25;
}
.markdown-body :deep(hr) {
  border: 0;
  border-top: 1px solid var(--color-border);
  margin: 0.8em 0;
}
</style>
