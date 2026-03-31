<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import hljs from 'highlight.js/lib/common'

const props = defineProps<{
  content: string
}>()

// Configure marked once at module level with hljs renderer
marked.use({
  renderer: {
    code({ text, lang }: { text: string; lang?: string }) {
      const language = lang && hljs.getLanguage(lang) ? lang : 'plaintext'
      const highlighted = hljs.highlight(text, { language }).value
      return `<pre><code class="hljs language-${language}">${highlighted}</code></pre>`
    },
  },
})

const containerRef = ref<HTMLElement | null>(null)

const html = computed(() => {
  if (!props.content) return ''
  const raw = marked.parse(props.content, { async: false }) as string
  return DOMPurify.sanitize(raw)
})

async function attachCopyButtons(): Promise<void> {
  await nextTick()
  const container = containerRef.value
  if (!container) return

  container.querySelectorAll<HTMLElement>('pre').forEach((pre) => {
    // Skip if already wrapped.
    if (pre.parentElement?.classList.contains('code-block-wrapper')) return

    const wrapper = document.createElement('div')
    wrapper.className = 'code-block-wrapper'
    pre.parentNode!.insertBefore(wrapper, pre)
    wrapper.appendChild(pre)

    const btn = document.createElement('button')
    btn.className = 'copy-btn'
    btn.textContent = 'Copy'
    btn.setAttribute('aria-label', 'Copy code')
    btn.addEventListener('click', async () => {
      const text = pre.querySelector('code')?.innerText ?? pre.innerText
      try {
        await navigator.clipboard.writeText(text)
        btn.textContent = 'Copied!'
        btn.classList.add('copied')
      } catch {
        btn.textContent = 'Failed'
      } finally {
        setTimeout(() => {
          btn.textContent = 'Copy'
          btn.classList.remove('copied')
        }, 2000)
      }
    })
    wrapper.appendChild(btn)
  })
}

watch(html, attachCopyButtons, { immediate: true })
</script>

<template>
  <div
    ref="containerRef"
    class="markdown-body text-sm text-foreground"
    v-html="html"
  />
</template>

<style scoped>
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
.markdown-body :deep(.code-block-wrapper) {
  position: relative;
  margin: 0.5em 0;
}
.markdown-body :deep(.code-block-wrapper:hover .copy-btn),
.markdown-body :deep(.code-block-wrapper:focus-within .copy-btn) {
  opacity: 1;
}
.markdown-body :deep(pre) {
  background: var(--color-muted);
  border: 1px solid var(--color-border);
  border-radius: 6px;
  padding: 0.75rem 1rem;
  overflow-x: auto;
  font-size: 0.8em;
  margin: 0;
}
.markdown-body :deep(pre code) {
  background: transparent;
  border: none;
  padding: 0;
  /* override hljs theme's own padding — pre already has padding */
  display: block;
  overflow-x: auto;
}
.markdown-body :deep(.copy-btn) {
  position: absolute;
  top: 0.4rem;
  right: 0.4rem;
  opacity: 0;
  transition: opacity 0.15s, background-color 0.15s;
  padding: 0.2rem 0.55rem;
  border-radius: 4px;
  font-size: 0.7rem;
  font-family: inherit;
  cursor: pointer;
  background: var(--color-background);
  color: var(--color-foreground);
  border: 1px solid var(--color-border);
  line-height: 1.4;
}
.markdown-body :deep(.copy-btn:hover) {
  background: var(--color-accent);
}
.markdown-body :deep(.copy-btn.copied) {
  color: var(--color-primary);
  border-color: var(--color-primary);
}
.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  padding-left: 1.4em;
  margin: 0.4em 0;
}
.markdown-body :deep(li) {
  margin: 0.15em 0;
}
.markdown-body :deep(blockquote) {
  border-left: 3px solid var(--color-border);
  padding-left: 0.75em;
  color: var(--color-muted-foreground);
  margin: 0.4em 0;
}
.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4) {
  font-weight: 600;
  margin: 0.5em 0 0.25em;
  line-height: 1.3;
}
.markdown-body :deep(hr) {
  border: none;
  border-top: 1px solid var(--color-border);
  margin: 0.5em 0;
}
.markdown-body :deep(strong) {
  font-weight: 600;
}
.markdown-body :deep(em) {
  font-style: italic;
}
</style>
