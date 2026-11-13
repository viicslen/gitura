import { onBeforeUpdate, onUpdated, type Ref } from 'vue'

// A native <details> changes height in one frame. These handlers, bound on the element that holds rendered markdown, run the
// change as a height transition instead; the transition itself is in the component's CSS (`details { transition: height }`).

/** `click` handler: toggles the <details> whose summary was clicked by gliding its height. */
export function glideDetails(event: MouseEvent): void {
  const target = event.target instanceof Element ? event.target : null
  const summary = target?.closest('summary')
  const details = summary?.parentElement
  if (!(details instanceof HTMLDetailsElement) || event.defaultPrevented || target?.closest('a, input, button')) return
  event.preventDefault()

  const from = details.offsetHeight
  const opening = !details.open || 'closing' in details.dataset
  delete details.dataset.closing
  details.style.height = ''
  // Closing: shut for a moment to measure where the glide ends, then stay open until it gets there.
  details.open = opening
  const to = details.offsetHeight
  details.open = true
  if (!opening) details.dataset.closing = ''
  if (from === to) return settle(details)

  // clip, not hidden: hidden would keep the children's margins inside the box and change its height.
  details.style.overflow = 'clip'
  details.style.height = `${from}px`
  void details.offsetHeight // the transition has to start from `from`
  details.style.height = `${to}px`
}

/** `transitionend` and `transitioncancel` handler: hands the height back to the layout once the glide is over. */
export function settleDetails(event: TransitionEvent): void {
  const details = event.target
  if (!(details instanceof HTMLDetailsElement) || event.propertyName !== 'height') return
  // A reversed glide cancels the old transition and starts a new one; only the last one settles.
  if (details.getAnimations().some((a) => a instanceof CSSTransition && a.transitionProperty === 'height')) return
  settle(details)
}

function settle(details: HTMLDetailsElement): void {
  if ('closing' in details.dataset) details.open = false
  delete details.dataset.closing
  details.style.height = ''
  details.style.overflow = ''
}

/** Re-opens the <details> that were open before `v-html` replaced them, e.g. when a ticked checkbox saved a new body. */
export function keepDetailsOpen(container: Ref<HTMLElement | null>): void {
  let open: boolean[] = []
  onBeforeUpdate(() => {
    open = Array.from(container.value?.querySelectorAll('details') ?? [], (details) => details.open)
  })
  onUpdated(() => {
    container.value?.querySelectorAll('details').forEach((details, i) => {
      if (open[i]) details.open = true
    })
  })
}
