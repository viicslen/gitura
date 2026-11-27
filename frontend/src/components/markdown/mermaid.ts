import { onMounted, onUpdated, watch, type Ref } from 'vue'
import { isDark } from '@/composables/useAppearance'

let queue = Promise.resolve()

async function draw(container: HTMLElement | null, redraw: boolean): Promise<void> {
  const nodes = Array.from(container?.querySelectorAll<HTMLElement>('div.mermaid') ?? []).filter(
    (node) => redraw || !node.dataset.source,
  )
  if (!nodes.length) return
  // Loaded on first use: mermaid is large and most bodies have no diagram.
  const { default: mermaid } = await import('mermaid')
  mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: isDark.value ? 'dark' : 'default' })
  for (const node of nodes) {
    node.dataset.source ??= node.textContent ?? ''
    node.textContent = node.dataset.source
    node.removeAttribute('data-processed')
  }
  // A bad diagram stays as its source text.
  await mermaid.run({ nodes, suppressErrors: true })
}

/** Draws the ```mermaid blocks that `v-html` puts in `container`, and redraws them when the theme changes. */
export function renderMermaid(container: Ref<HTMLElement | null>): void {
  // mermaid.initialize is global, so draws run one at a time to keep each one's theme.
  const enqueue = (redraw: boolean) => {
    queue = queue.then(() => draw(container.value, redraw)).catch((error: unknown) => console.error(error))
  }
  onMounted(() => enqueue(false))
  onUpdated(() => enqueue(false))
  watch(isDark, () => enqueue(true))
}
