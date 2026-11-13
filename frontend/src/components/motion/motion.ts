/** Class presets for `<Transition>` and `<TransitionGroup>`; spread them with `v-bind`. */

/** Two small states stacked in one grid cell (`[grid-area:1/1]` on both). Too costly for whole panels: fade those in only. */
export const crossfade = {
  enterActiveClass: 'transition-opacity duration-base',
  enterFromClass: 'opacity-0',
  leaveActiveClass: 'transition-opacity duration-fast pointer-events-none',
  leaveToClass: 'opacity-0',
}

/** A badge-sized element appearing in space that is already reserved. */
export const scaleIn = {
  enterActiveClass: 'transition-[opacity,scale] duration-fast',
  enterFromClass: 'opacity-0 scale-90',
  leaveActiveClass: 'transition-[opacity,scale] duration-faster',
  leaveToClass: 'opacity-0 scale-90',
}

/**
 * Base classes of an in-flow block that opens and closes so its siblings glide. Its single child needs `min-h-0` and no
 * padding or border of its own (they would keep the closed block from reaching zero); spacing goes on a margin of the
 * block or inside the child. The `minmax(0,1fr)` column keeps wide content (tables, code) scrolling inside the block
 * instead of widening it past its parent.
 */
export const collapsible = 'grid grid-cols-[minmax(0,1fr)] transition-[grid-template-rows,margin,opacity]'
export const collapse = {
  enterActiveClass: '*:overflow-hidden',
  enterFromClass: 'grid-rows-[0fr] my-0! opacity-0',
  enterToClass: 'grid-rows-[1fr]',
  leaveActiveClass: '*:overflow-hidden duration-fast pointer-events-none',
  leaveFromClass: 'grid-rows-[1fr]',
  leaveToClass: 'grid-rows-[0fr] my-0! opacity-0',
}

/**
 * The same along the inline axis, for an item of a flex row; the single child needs `min-w-0`. `closedMargin` is the
 * margin that cancels the row's gap while the item is closed (`-ml-2!` in a `gap-2` row), so it takes no room at all.
 */
export const collapsibleX = 'grid transition-[grid-template-columns,margin,opacity]'
export function collapseX(closedMargin: string) {
  return {
    enterActiveClass: '*:overflow-hidden',
    enterFromClass: `grid-cols-[0fr] ${closedMargin} opacity-0`,
    enterToClass: 'grid-cols-[1fr]',
    leaveActiveClass: '*:overflow-hidden duration-fast pointer-events-none',
    leaveFromClass: 'grid-cols-[1fr]',
    leaveToClass: `grid-cols-[0fr] ${closedMargin} opacity-0`,
  }
}

/**
 * Opens and closes an element by height, so what sits below glides with it. The element must be a
 * `grid` with one `min-h-0` child, and carry no transition, duration or overflow class of its own:
 * Vue strips these classes when the move ends, and would take a same-named class of the element with them.
 */
export const ROW_GLIDE = {
  enterActiveClass: 'overflow-clip transition-[grid-template-rows,opacity] duration-base',
  enterFromClass: 'grid-rows-[0fr] opacity-0',
  enterToClass: 'grid-rows-[1fr]',
  leaveActiveClass: 'overflow-clip transition-[grid-template-rows,opacity] duration-fast',
  leaveFromClass: 'grid-rows-[1fr]',
  leaveToClass: 'grid-rows-[0fr] opacity-0',
}

/**
 * Cross-fades what a box shows. The parent must be `relative`: the one leaving is taken
 * out of flow and laid over the box, so the box keeps its size and nothing moves.
 */
export const SWAP = {
  enterActiveClass: 'animate-in fade-in-0',
  leaveActiveClass: 'pointer-events-none absolute inset-0 animate-out fade-out-0 animation-duration-fast',
}

/** Cross-fades two pieces that share one grid cell (`[grid-area:1/1]`), which keeps the size of the larger. */
export const TEXT_SWAP = {
  enterActiveClass: 'animate-in fade-in-0',
  leaveActiveClass: 'animate-out fade-out-0 animation-duration-fast',
}

/** The old content goes in one frame and the new fades in: for swaps too heavy to hold both at once. */
export const SETTLE = {
  enterActiveClass: 'animate-in fade-in-0',
  leaveActiveClass: 'hidden',
}

/**
 * For a loader that stays mounted, on an element with `transition-[opacity,visibility]`: it shows only
 * once the wait is real, and a wait that ends sooner never paints it.
 */
export const loaderShown = (on: boolean) => (on ? 'delay-200' : 'invisible opacity-0')
