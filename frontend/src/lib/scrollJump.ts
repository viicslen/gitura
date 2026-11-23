type Scroller = Pick<HTMLElement, 'scrollTop' | 'scrollHeight' | 'clientHeight'>

/**
 * Scrolls `el` so the next (or previous) of `tops`, ascending offsets in its content, sits `margin` below its top.
 * Skips any the scroll can't reach past where it is, so it never sticks at the end; false when none is left.
 */
export function scrollToNext(el: Scroller, tops: number[], dir: 1 | -1, margin = 0): boolean {
  const max = el.scrollHeight - el.clientHeight
  const targets = tops.map((top) => Math.round(Math.min(max, Math.max(0, top - margin))))
  const at = el.scrollTop
  const target = dir > 0 ? targets.find((t) => t > at + 1) : targets.reverse().find((t) => t < at - 1)
  if (target === undefined) return false
  el.scrollTop = target
  return true
}
