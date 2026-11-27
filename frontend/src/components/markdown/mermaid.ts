import { onMounted, onUpdated, watch, type Ref } from 'vue'
import DOMPurify from 'dompurify'
import { isDark } from '@/composables/useAppearance'
import { proxyMermaidImages } from '@/lib/markdown'

let queue = Promise.resolve()
let drawn = 0

async function draw(container: HTMLElement | null, redraw: boolean): Promise<void> {
  const nodes = Array.from(container?.querySelectorAll<HTMLElement>('div.mermaid') ?? []).filter(
    (node) => redraw || !node.dataset.source,
  )
  if (!nodes.length) return
  // Loaded on first use: mermaid is large and most bodies have no diagram.
  const { default: mermaid } = await import('mermaid')
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: 'strict',
    suppressErrorRendering: true,
    theme: isDark.value ? 'dark' : 'default',
  })
  for (const node of nodes) {
    const source = (node.dataset.source ??= node.textContent ?? '')
    try {
      const { svg } = await mermaid.render(`mermaid-${++drawn}`, proxyMermaidImages(source))
      // Mermaid's strict mode sanitizes too, with these options; ours keeps that guarantee from riding on its config.
      node.innerHTML = DOMPurify.sanitize(svg, {
        ADD_TAGS: ['foreignobject'],
        ADD_ATTR: ['dominant-baseline'],
        HTML_INTEGRATION_POINTS: { foreignobject: true },
      })
    } catch {
      // A diagram mermaid cannot parse or draw stays as its source text.
      node.textContent = source
    }
  }
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
