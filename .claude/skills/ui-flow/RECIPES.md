# UI flow recipes

One pattern per kind of edge, each with the traps it has. Plain CSS first; use
the framework's or component kit's equivalent when it has one. Durations and
easings are the tokens below — the snippets name them, never a literal.

## Tokens

```css
:root {
  --motion-faster: 80ms;     /* exit of fast; pointer-out */
  --motion-fast: 130ms;      /* frequent, low end */
  --motion-base: 180ms;      /* frequent */
  --motion-slow: 260ms;      /* occasional */
  --motion-slower: 400ms;    /* rare */
  --ease-out: cubic-bezier(0.2, 0, 0, 1);
  --ease-in-out: cubic-bezier(0.4, 0, 0.2, 1);
}
```

Register them in the styling system's own theme so its utilities pick them up,
and make the system's default transition duration and easing point at
`base` and `ease-out`.

## Reduced motion

Every edge becomes a pop; progress indicators keep moving so nothing looks
frozen:

```css
@media (prefers-reduced-motion: reduce) {
  *:not(.spinner), ::before, ::after {
    animation-duration: 1ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 1ms !important;
    scroll-behavior: auto !important;
  }
}
```

Animation delays survive this rule, so delayed loaders still wait their 200ms.

## Overlay enter and exit

Dialog, menu, popover, tooltip, toast. Fade plus a small scale from the trigger.

```css
.popover {
  transform-origin: var(--trigger-origin, top center);
  transition: opacity var(--motion-fast) var(--ease-out), transform var(--motion-fast) var(--ease-out);
}
.popover[data-state="closed"] { opacity: 0; transform: scale(0.96); }
```

- Headless kits (Radix, Reka, Base UI, Headless UI) expose `data-state` and a
  transform-origin variable, and hold the element mounted until its exit
  animation ends. Use that rather than a wrapper, and check every content
  component sets the origin variable; generated ones often miss some.
- No kit: `@starting-style` for the enter and `transition-behavior:
  allow-discrete` on `display`/`overlay` for the exit.
- Tooltips: delay before the first one, none when moving between neighbours.
- Menus and context menus sit in the frequent tier: enter `fast`, exit `faster`.
- The backdrop fades; it never scales or blurs in.
- **Content outlives the open flag.** An overlay opened by `open = !!item`
  loses its content the instant `item` clears, so the exit animates an empty
  or "null" shell. Keep a separate open flag and leave the content in place
  until the next open.

## Swap in place

Loading → content, one label for another, icon A → icon B, tab panels.

Stack both in one grid cell so the box never collapses, and crossfade:

```css
.swap { display: grid; }
.swap > * { grid-area: 1 / 1; transition: opacity var(--motion-fast) var(--ease-out); }
.swap > [data-hidden] { opacity: 0; pointer-events: none; }
```

- The same stack keeps a button's width when its label gives way to a spinner:
  the label goes `visibility: hidden`, the spinner sits on top.
- Sequential swaps (Vue `<Transition mode="out-in">`, AnimatePresence
  `mode="wait"`) add the two durations together. Use them only with each half
  at `fast` or below, or when the two states differ in size and overlap would
  look broken.
- When the two states differ in size, fix the size of the container first
  (min-height, aspect-ratio, skeleton with the final shape), then crossfade.
- Settling (the old state gone in one frame, the new one fading in) is the
  rung below this one. It needs its reason; "cheapest" is not one.
- Content swapped inside a panel that stays (the detail of the selected item,
  a file's contents): key the content on what it shows and crossfade the
  keyed element, with the outgoing one out of flow so the panel keeps its
  size.
- Icon swaps: one icon rotated (`chevron`) beats two icons swapped; otherwise
  crossfade with a small scale.

## Toggle that must retarget

For a control that appears and disappears often and can be reversed mid-flight
(a clear button, a hover toolbar), keep it mounted and toggle visibility:

```css
.clear { transition: opacity var(--motion-fast), visibility var(--motion-fast); }
.clear[data-hidden] { opacity: 0; visibility: hidden; }
```

It retargets both ways, keeps its space (no shift), and leaves the tab order
while hidden.

## Expand and collapse

```css
.collapse { display: grid; grid-template-rows: 0fr; transition: grid-template-rows var(--motion-base) var(--ease-out); }
.collapse[data-open] { grid-template-rows: 1fr; }
.collapse > * { overflow: hidden; min-height: 0; }
```

- Padding and borders go on a wrapper inside the collapsing child, or the row
  never reaches zero.
- This is the costly tier: every frame lays out whatever sits below. Build it,
  run it against the largest case, and read the frame times before deciding it
  is too expensive. Static siblings that only change position are cheap to lay
  out; script that re-measures them every frame (a resize observer, a scroll
  sync) is what usually drops frames, and that script can be paused for the
  length of the animation.
- If it does drop frames: open the space in one frame, move everything below
  with one `transform` on a shared wrapper back to where it was, and animate
  that transform to zero (FLIP on the wrapper, not on each row).
- Only after both fail: open the space in one frame and fade the revealed
  content. Write the measurement on the row.
- Closing mirrors opening on the same rung.
- When script is paused for the animation, whatever depended on it (scroll
  sync, connectors) is driven from one height read per frame instead, or it
  lags behind the glide.
- `content-visibility: auto` makes an entering block open to its placeholder
  size and then snap. Native `<details>` needs script to animate, and
  `overflow: clip` while it does.
- Rotate the chevron with a transform either way.

## List add, remove, move

FLIP: record positions, apply the change, animate the delta with `transform`.

- A removed item fades while taken out of flow (`position: absolute`) so its
  siblings close up; an added one fades in at its final position. Neither
  slides in from an edge.
- Out-of-flow only keeps the item's place in a block container. In a flex or
  grid container it jumps to the start; make the list a block first.
- A FLIP container measures every child on every re-render. Keep it away from
  lists that re-render constantly (streaming output, per-keystroke filters)
  and cap it on lists that can reach hundreds of rows.
- Filtering and sorting move many items at once. Measure how many rows glide
  without dropping frames and cap the per-row glide there; above the cap, key
  the list on the filter and fade it in as one block.
- Items present at first render do not animate. Where the framework mounts the
  transition together with its content, gate it with a flag that turns true
  after mount.
- List ↔ empty state: give the empty state a key inside the same group, or
  fade each in enter-only.

## Delayed loader

A loader that only appears if the wait is real, with no timer code:

```css
.loader { animation: loader-in var(--motion-base) var(--ease-out) 200ms backwards; }
@keyframes loader-in { from { opacity: 0; } }
```

- Delay the indicator, not its frame: the panel, header and chrome around it
  render immediately.
- An in-button spinner gets the same delay; a call that returns in a few
  milliseconds then never flickers the icon.
- Refresh of content already on screen: keep the content, show activity on the
  control that triggered it, and dim the stale region only once the wait is
  real. Leave the control enabled if a second press can be queued; a disabled
  blink on every background refresh is its own flicker.
- An error stays on screen until its retry answers.
- Skeletons mirror the final layout box for box, so the swap is a crossfade
  with no shift.
- Optimistic updates beat any loader: apply the result now, roll back on error.

## Page and view change

Persistent chrome (nav, title bar, side rail) sits outside the transition and
does not move. The content area fades in with 4–8px of travel: forward/deeper
from one side, back from the other.

```css
.page { animation: page-in var(--motion-base) var(--ease-out); }
@keyframes page-in { from { opacity: 0; transform: translateY(6px); } }
```

- The old page fades out over the exit token while the new one enters on top
  (both absolutely positioned in the content area for that moment). When
  double-mounting two heavy pages drops frames, settle instead: the old page
  goes in one frame and the new one enters. Put the enter class on the page
  root; a transition wrapper without leave styles still holds the old page a
  few frames.
- The class replays when a kept-alive page reactivates, which is what gives
  "back" its animation.
- On a surface the size of a page or tab panel, fade with an opacity-only
  keyframe. Kit enter utilities also animate `transform`, which alone can drop
  frames at that size.
- Page-owned content living in the chrome (a title, toolbar actions) fades
  with its page; the chrome itself stays still.
- A page that opens on a loader needs no enter of its own: the content's
  arrival is the edge.
- Same-document View Transitions give shared-element moves (a list row growing
  into its detail header); feature-detect and fall back to the enter class.
- Restore scroll position on back before the enter starts.

## Hover and press

Instant in, eased out — a pointer crossing a dense list leaves no trail of
half-lit rows:

```css
.row { transition: background-color var(--motion-faster) var(--ease-out); }
.row:hover { transition-duration: 0s; }
.button:active { transform: scale(0.97); transition: transform var(--motion-faster) var(--ease-out); }
```

- `:hover { transition-duration: 0s }` also zeroes every other transition on
  the element while the pointer is on it, which is exactly when it gets
  clicked. Use it on elements whose only transition is the hover.
- A hover-revealed control must also show on `:focus-within` and while a menu
  it opened is open; otherwise keyboard users never see it and it vanishes
  under its own menu.
- Focus rings appear instantly: keep `box-shadow`/`outline` out of the
  transition, or zero the duration under `:focus-visible`.

## Value change

- Numbers: `font-variant-numeric: tabular-nums`; a ticking counter cuts or
  crossfades, never a count-up the user must wait for.
- Progress bars: transition `transform: scaleX()` with `transform-origin:
  left`, not `width`.
- A row changed by a background update: flash its background once and let it
  fade over ~600ms. No movement.

## Scroll

- Programmatic scroll is smooth only when the target is within about one
  viewport; farther, jump.
- Content inserted above the viewport must not move what the user is reading:
  rely on scroll anchoring (`overflow-anchor`) or adjust `scrollTop` by the
  inserted height in the same frame.
- Scrolling driven by a held or repeated key pops (`behavior: 'auto'`).

## Theme and global switches

Hundreds of elements each running their own colour transition look like a
smear. Suppress transitions for the frame of the switch, or wrap it in
`document.startViewTransition()` for a single root crossfade. The view
transition captures the whole page first; on a heavy screen that capture frame
can run past 100ms, so measure it before keeping it.

## Stack notes

**Vue**

- `<Transition>` on `v-if` never retargets: reversing mid-flight removes the
  leaving element and starts the enter from scratch.
- A `<Transition>` with no leave classes still keeps the old element for a
  couple of frames, and for the full duration of any unrelated
  `transition-duration` the element carries. Give it an explicit leave or use
  a plain enter class instead.
- `v-show` and `<KeepAlive>` replay keyframe classes on everything inside.
- `<TransitionGroup>` needs single-root children and re-measures all of them
  on every update when a move class is set.
- A `<Transition>` around each row of a `v-for` cannot run a leave: the row
  is gone with its list entry. Keep leaving rows in a list of their own until
  the exit ends.
- Teleported children are removed when their owner unmounts, before any leave
  runs. Put the `<Transition>` inside the teleport target, or fade a clone.
- When a transition ends Vue removes its class tokens from the element, and a
  base class with the same name (`grid`, `duration-*`) goes with them. Keep
  the element's own classes distinct from its transition classes.
- Test utils render `<Transition>` and `<TransitionGroup>` as stub elements, so
  a spec selecting the group by its tag (`ul[role=list]`) stops matching.

**Tailwind (v4) with tw-animate-css**

- `animate-in`/`animate-out` fall back to a literal `150ms` and `ease`, and
  their keyframes animate `transform` and `filter` even for a plain fade.
  Override `--animate-in`/`--animate-out` in the theme to use the tokens and
  keyframes of your own.
- A bare `duration-*` also sets `transition-duration`, and with no
  `transition-*` class the property is `all`: the element now transitions
  everything. Time animations with `animation-duration-*` (give it the same
  tokens) and keep `duration-*` for elements that name their transition.
- `ease-*` sets both the transition and the animation easing; `delay-*` sets
  both delays.
- Theme tokens are emitted only when a utility uses them; mark motion tokens
  `@theme static` if hand-written CSS reads them.
- Unlayered third-party CSS (a toast library) beats every utility; override it
  in unlayered CSS with `!important`.
- An important utility (`duration-fast!`) beats the reduced-motion rule; keep
  `!` off motion classes.
- `delay-*` also delays the leave of the element it sits on.
- `focus-visible:duration-0` on a kit button zeroes every transition while the
  button has keyboard focus, the same trap as the hover recipe.
- `scale-*`, `rotate-*` and `translate-*` set the individual `scale`, `rotate`
  and `translate` properties, so name those in `transition-[…]`.

## Support to check

Verify against the weakest target before relying on:

| Feature | Notes |
| --- | --- |
| `@starting-style`, `transition-behavior: allow-discrete` | recent evergreen browsers; older WebKit embeds lack them |
| same-document View Transitions | Chromium and Safari 18+; feature-detect always |
| `interpolate-size` / `calc-size()` | Chromium only; use the grid-rows collapse elsewhere |
| GPU compositing of `transform`/`opacity` | not guaranteed in embedded webviews; test in the real shell |
