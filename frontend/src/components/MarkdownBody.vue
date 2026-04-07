<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import hljs from 'highlight.js/lib/common'

interface RunCommandEntry {
  id: string
  name: string
  command: string
}

const props = defineProps<{
  content: string
  /** When provided, run split-buttons are injected on each code block */
  runCommands?: RunCommandEntry[]
  defaultCommandId?: string
  onRunCode?: (commandId: string, text: string, threadRootId: number, commentId: number) => void
  /** Thread root ID to associate runs with */
  threadRootId?: number
  /** Comment ID to associate runs with */
  commentId?: number
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

const PLAY_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="6 3 20 12 6 21 6 3"/></svg>`
const CHEVRON_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>`

async function attachCopyButtons(): Promise<void> {
  await nextTick()
  const container = containerRef.value
  if (!container) return

  const commands = props.runCommands ?? []
  const primaryCmd = commands.find((c) => c.id === props.defaultCommandId) ?? commands[0] ?? null

  container.querySelectorAll<HTMLElement>('pre').forEach((pre) => {
    // Skip if already wrapped.
    if (pre.parentElement?.classList.contains('code-block-wrapper')) return

    const wrapper = document.createElement('div')
    wrapper.className = 'code-block-wrapper'
    pre.parentNode!.insertBefore(wrapper, pre)
    wrapper.appendChild(pre)

    // ── Copy button (leftmost) ──────────────────────────────────────────────
    const copyBtn = document.createElement('button')
    copyBtn.className = 'copy-btn'
    copyBtn.textContent = 'Copy'
    copyBtn.setAttribute('aria-label', 'Copy code')
    copyBtn.addEventListener('click', async () => {
      const text = pre.querySelector('code')?.innerText ?? pre.innerText
      try {
        await navigator.clipboard.writeText(text)
        copyBtn.textContent = 'Copied!'
        copyBtn.classList.add('copied')
      } catch {
        copyBtn.textContent = 'Failed'
      } finally {
        setTimeout(() => {
          copyBtn.textContent = 'Copy'
          copyBtn.classList.remove('copied')
        }, 2000)
      }
    })
    // ── Run split button (rightmost, only when commands provided) ──────────
    if (commands.length > 0 && props.onRunCode) {
      const onRunCode = props.onRunCode

      const splitWrap = document.createElement('div')
      splitWrap.className = 'run-split'

      // Primary run button
      const runBtn = document.createElement('button')
      runBtn.className = 'run-btn run-btn-primary'
      runBtn.setAttribute('aria-label', `Run code with ${primaryCmd?.name ?? 'command'}`)
      runBtn.innerHTML = PLAY_SVG
      runBtn.addEventListener('click', () => {
        if (!primaryCmd) return
        const text = pre.querySelector('code')?.innerText ?? pre.innerText
        onRunCode(primaryCmd.id, text, props.threadRootId ?? 0, props.commentId ?? 0)
      })
      splitWrap.appendChild(runBtn)

      // Chevron button + dropdown (only when >1 commands)
      if (commands.length > 1) {
        const chevronBtn = document.createElement('button')
        chevronBtn.className = 'run-btn run-btn-chevron'
        chevronBtn.setAttribute('aria-label', 'Run with a different command')
        chevronBtn.innerHTML = CHEVRON_SVG

        const dropdown = document.createElement('div')
        dropdown.className = 'run-dropdown'

        const label = document.createElement('div')
        label.className = 'run-dropdown-label'
        label.textContent = 'Run with…'
        dropdown.appendChild(label)

        commands.forEach((cmd) => {
          const item = document.createElement('button')
          item.className = 'run-dropdown-item'
          item.innerHTML = `<span class="run-dropdown-name">${cmd.name}</span><span class="run-dropdown-cmd">${cmd.command}</span>`
          item.addEventListener('click', (e) => {
            e.stopPropagation()
            dropdown.classList.remove('open')
            const text = pre.querySelector('code')?.innerText ?? pre.innerText
            onRunCode(cmd.id, text, props.threadRootId ?? 0, props.commentId ?? 0)
          })
          dropdown.appendChild(item)
        })

        chevronBtn.addEventListener('click', (e) => {
          e.stopPropagation()
          dropdown.classList.toggle('open')
        })

        // Close on outside click
        document.addEventListener('click', () => dropdown.classList.remove('open'))

        splitWrap.appendChild(chevronBtn)
        splitWrap.appendChild(dropdown)
      }

      wrapper.appendChild(splitWrap)
    }

    wrapper.appendChild(copyBtn)
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
  right: 3.5rem;
  opacity: 0;
  transition: opacity 0.15s, background-color 0.15s;
  padding: 0 0.55rem;
  height: 1.5rem;
  box-sizing: border-box;
  border-radius: 4px;
  font-size: 0.7rem;
  font-family: inherit;
  cursor: pointer;
  background: var(--color-background);
  color: var(--color-foreground);
  border: 1px solid var(--color-border);
  line-height: 1;
  display: flex;
  align-items: center;
}
.markdown-body :deep(.copy-btn:hover) {
  background: color-mix(in srgb, var(--color-foreground) 10%, var(--color-background));
}
.markdown-body :deep(.copy-btn.copied) {
  color: var(--color-primary);
  border-color: var(--color-primary);
}
/* run split: positioned to the right of copy btn */
.markdown-body :deep(.run-split) {
  position: absolute;
  top: 0.4rem;
  right: 0.4rem;
  display: flex;
  align-items: stretch;
  gap: 0;
  opacity: 0;
  transition: opacity 0.15s;
  border-radius: 4px;
  height: 1.5rem;
  box-sizing: border-box;
}
.markdown-body :deep(.code-block-wrapper:hover .run-split),
.markdown-body :deep(.code-block-wrapper:focus-within .run-split) {
  opacity: 1;
}
.markdown-body :deep(.run-btn) {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 0.4rem;
  font-size: 0.7rem;
  cursor: pointer;
  background: var(--color-background);
  color: #22c55e;
  border: 1px solid var(--color-border);
  line-height: 1;
  transition: background-color 0.15s;
  box-sizing: border-box;
}
.markdown-body :deep(.run-btn:hover) {
  background: color-mix(in srgb, var(--color-foreground) 10%, var(--color-background));
}
.markdown-body :deep(.run-btn-primary) {
  border-radius: 4px 0 0 4px;
  border-right: none;
}
.markdown-body :deep(.run-btn-chevron) {
  border-radius: 0 4px 4px 0;
  padding-left: 0.3rem;
  padding-right: 0.3rem;
  margin-left: -1px;
}
/* dropdown */
.markdown-body :deep(.run-dropdown) {
  display: none;
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  z-index: 50;
  min-width: 180px;
  border-radius: 6px;
  border: 1px solid var(--color-border);
  background: var(--color-popover, var(--color-background));
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
  padding: 4px 0;
}
.markdown-body :deep(.run-dropdown.open) {
  display: block;
}
.markdown-body :deep(.run-dropdown-label) {
  padding: 4px 10px 2px;
  font-size: 0.65rem;
  color: var(--color-muted-foreground);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.markdown-body :deep(.run-dropdown-item) {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 1px;
  width: 100%;
  padding: 5px 10px;
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: none;
  color: var(--color-foreground);
  transition: background-color 0.1s;
}
.markdown-body :deep(.run-dropdown-item:hover) {
  background: color-mix(in srgb, var(--color-foreground) 8%, var(--color-popover, var(--color-background)));
}
.markdown-body :deep(.run-dropdown-name) {
  font-size: 0.75rem;
  font-weight: 500;
}
.markdown-body :deep(.run-dropdown-cmd) {
  font-size: 0.65rem;
  font-family: ui-monospace, monospace;
  color: var(--color-muted-foreground);
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
