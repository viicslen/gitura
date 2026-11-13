---
name: ui-flow
description: Motion that makes a UI flow — every visible state change either flows into the next state (transition, animation, smoothing) or is a deliberate cut, budgeted by how often the user triggers it and what it costs to render. Use for a full-app motion sweep, when improving the feel of existing frontend/UI/UX, and when building any new UI, section or page.
---

# UI flow

An **edge** is one visible change from one state to another. An edge **flows**
when the change is no surprise: things arrive from somewhere, leave to
somewhere, and whatever stays put stays put. The opposite is a **cut** — one
frame the old state, the next frame the new one.

Flow is the default. A cut is a decision, and it needs a reason from
[Cut on purpose](#cut-on-purpose). Motion that is slow, repeated or janky is
worse than a cut, so every flow is sized by a **budget**: how often the user
triggers it, and what it costs to render.

## Process

### 1. Foundation (once per codebase)

Before touching a component, confirm the plumbing:

- **Tokens.** Durations and easings live as named tokens (CSS variables / theme
  values), one per tier of the time budget plus one below the lowest for exits
  and pointer-out. From then on every duration and easing in the codebase is a
  token.
- **Dead motion.** Existing animation classes and transition wrappers must
  resolve to real CSS. Component kits ship classes (`animate-in`, `fade-in-0`,
  `data-[state=open]:…`) that silently do nothing when their plugin is not
  installed. Compile the stylesheet and look for the keyframes.
- **Kit keyframes.** Read what the kit's animation utilities compile to. They
  may animate more than their names say (`transform` and `filter` on a plain
  fade) and fall back to their own duration and easing instead of your tokens.
  Point the fallbacks at the tokens once, centrally.
- **Reduced motion.** One global `prefers-reduced-motion: reduce` rule that
  turns every edge into a cut, exempting progress indicators.
- **Renderer.** Know the weakest target: an embedded webview (Tauri, Wails,
  Electron on Linux), a low-end phone. If it composites in software, shift the
  cost table down one tier: `opacity` and `transform` are cheap rather than
  free, and a fade over a viewport-sized area is a cut.

Done when a token exists for each tier, one known overlay visibly animates in
and out, and the reduced-motion rule is in place.

### 2. Inventory

List every edge in the surface you are working on. Which surface depends on the
job:

- **Sweep** — the whole app, one feature area at a time.
- **Improving existing UI** — the surface you were asked about, plus every
  motion already on it (audit those against the budget; shortening or removing
  one counts as an improvement).
- **New UI** — write the states first (empty, loading, loaded, error, partial,
  disabled, selected, open), then every edge between them, before any markup.

Find edges by reading the code for what can change on screen:

- conditional rendering (`v-if`, `&&`, ternaries, `<Show>`, route/page switches)
- async state: loading → loaded / empty / error, refresh, optimistic update
- **transient wrong states**: a default or empty state painted before the data
  arrives; an earlier state flashing back between two async steps; a control
  disabled or spinning for a few milliseconds, or by a background event
- collection changes: add, remove, reorder, filter, sort, paginate
- overlays: dialog, menu, popover, tooltip, toast, drawer — and whether their
  content reads the same state that closes them (it then vanishes before the
  exit finishes)
- disclosure: expand/collapse, tabs, accordions, resizable panels
- element states: hover, press, focus, selected, disabled, invalid, toggled
- value changes: counters, badges, progress, text or icon swapped in place
- viewport: programmatic scroll, scroll restoration, focus moves
- global: theme switch, layout mode, window resize
- third-party motion (toast, chart and carousel libraries): its durations count
  against the budget too

Edges that share a verdict and a reason are one row ("row hovers", "kit
overlays").

Done when every item in that list has been checked against the surface and each
edge found is on a row.

### 3. Verdict

Give every row exactly one verdict:

- **flow** — tier, properties, pattern.
- **cut** — with its reason from [Cut on purpose](#cut-on-purpose).
- **keep** — already flows within budget.
- **fix** — already animated, but over budget, on a costly property, dead, or
  replaying when it should not.
- **defer** — flowing it needs a structural change or a file outside the
  surface. Report it with the exact change.

Two rules settle most hard rows:

- **One edge, one animation.** The outermost element that changes carries the
  motion; everything inside it rides along with no enter of its own.
- **Several triggers, one budget.** An edge reachable by pointer and keyboard,
  or by first load and by a filter toggle, takes the budget of its most
  frequent trigger.

State logic is in scope. The worst flow defects are state bugs: a loader that
replaces content on refresh, a flag cleared one step early, an error wiped the
moment its retry starts. Fix them where the fix is small, and report every
behaviour change separately from the motion.

Done when no row is without a verdict.

### 4. Implement

Read [`RECIPES.md`](RECIPES.md) before writing motion code: it holds the
pattern for each kind of edge, the traps each pattern has, and notes per
stack. Reuse the framework's and component kit's own transition mechanism
before adding a library.

### 5. Verify

Still screenshots cannot show motion. Check each flow and fix edge:

- **Running it**: trigger the edge and read `document.getAnimations()` (or the
  devtools Animations panel) — properties are in the free/cheap tiers and the
  duration is inside its budget.
- **Rapid-fire**: trigger it five times fast and reverse it mid-flight.
  Nothing queues, lags, flashes or ends in the wrong state.
- **Reduced motion**: emulate it; the edge still reads as a cut.
- **Stability**: nothing outside the changing element shifts position.

An app that needs a backend you cannot start still runs in a browser: serve the
frontend alone and stub its transport (`fetch`, the IPC bridge) with canned
answers to reach the screens behind sign-in.

When the app cannot be run, do the static half and say the rest is unverified:
compile the stylesheet and confirm each motion class emits the duration and
properties you expect; search for `transition: all`, literal durations, and
enter-animation classes under an ancestor whose display toggles.

## Budget

### Time: how often is it triggered?

| Tier | Frequency | Examples | Duration |
| --- | --- | --- | --- |
| **constant** | many times a minute | hover, keypress, typing, row selection, scrubbing, drag | 0ms; pointer-out may ease over the smallest token |
| **frequent** | several times a minute | tabs, toggles, checkboxes, menus, tooltips, accordions | 100–150ms |
| **occasional** | a few times a session | dialogs, drawers, side panels, page changes, toasts | 150–250ms |
| **rare** | about once | first load, onboarding, success moment, empty → first content | up to 400ms |

- An exit uses the next token down from its enter.
- Within a tier, small elements and short travel take the low end.
- Run the **50th-time test**: picture the user triggering it for the fiftieth
  time today. If it would grate, drop a tier.
- Easing: `ease-out` for anything answering user input, exits included (an
  ease-in exit starts slow and reads as lag); `ease-in-out` for something
  moving between two on-screen positions; `linear` only for continuous motion
  (spinners, progress).

### Cost: what does a frame cost?

| Tier | Properties | Use |
| --- | --- | --- |
| **free** | `transform`, `opacity` | anywhere the renderer composites on the GPU |
| **cheap** | `color`, `background-color`, `border-color`, `outline`, small `box-shadow` | small elements |
| **costly** | anything that moves other elements: `width`, `height`, `margin`, `padding`, `top`/`left`, `grid-template-*`, `font-size` | only when the subtree that reflows is small |
| **heavy** | `filter`, `backdrop-filter`, large shadows, anything repainting a large area | static; fade a wrapper instead |

- Name the properties in every transition (`transition-colors`,
  `transition-[opacity,transform]`); `all` picks up costly ones by accident.
- A costly animation on a small panel is fine. When the change pushes a large
  document (a long list, a table, a diff, an editor), let the layout jump in one
  frame and flow the new content inside it with opacity/transform.
- More than about a dozen elements moving at once: animate their container.
- Entering lists animate only items added after first render. First render of
  a long or virtualised list appears as one block.
- Streaming content (logs, tokens, terminal output) appends without animation,
  and stays outside any container that measures its children to animate them.

## Cut on purpose

The reasons to leave an edge as a cut:

1. **Frequency** — it is in the constant tier, or failed the 50th-time test.
2. **Keyboard** — the user drove it from the keyboard and is already looking at
   the result (arrow-key selection, shortcut navigation, typing).
3. **Cost** — the only way to flow it is costly/heavy at this size, and no
   free-tier substitute reads well.
4. **Waiting** — an exit, a sequential swap or a delay would stand between the
   user and content or input they are waiting for. (The awaited content itself
   may still fade in from the frame it exists.)
5. **Acknowledgement** — it is the direct answer to a press (label → "Saving…",
   an optimistic toggle) and must land in the same frame.
6. **Rides along** — it changes in the same frame as a larger edge that already
   has its verdict (a child of an entering panel, a badge beside its list).
7. **Too small** — a fixed box, no shift, smaller than an icon; there is
   nothing to see at any duration.

"It was simpler" is not on the list. When the reason is effort, the verdict is
defer.

## What good flow obeys

- **Stable layout first.** A layout shift is the worst cut. Reserve space for
  what is loading, match skeletons to the final layout, use tabular numerals
  for changing numbers, and keep persistent chrome still while content changes.
- **Input never waits.** The UI accepts the next click or key while an
  animation runs. Content appears as soon as it exists; choreography never
  delays it. A stagger totals 100ms at most.
- **Interruptible.** State that toggles on an element that stays mounted uses
  transitions, which retarget from the current value. Mounting and unmounting
  restart from the beginning instead, so keep those at the frequent tier or
  below, or toggle visibility on an element that stays mounted.
- **Enters replay.** A keyframe animation class left on an element runs again
  every time the element re-enters rendering: an ancestor going from
  `display: none` to shown (tabs), a kept-alive view reactivating, a parent
  remounting, first render. Use that on purpose (one class fades a tab panel on
  every switch) or scope the class to the moment of entering.
- **Origin.** Things grow from what opened them (`transform-origin` at the
  trigger) and shrink back to it. Panels scale from 0.95–0.98, icon-sized
  elements from 0.75–0.85, nothing from 0. In-flow content travels 4–12px;
  only edge-anchored drawers and sheets travel their own size.
- **Direction means something.** Forward/deeper and back/shallower move in
  opposite directions; siblings crossfade.
- **Loaders are edges too.** Show a spinner or skeleton only after ~200ms, so
  fast responses go straight to content. Delay only the indicator: the frame
  and chrome around it stay mounted. An empty state painted before the data
  arrives is a loader and gets the same delay. A refresh keeps the old content
  on screen instead of replacing it with a loader.
- **Motion follows the user's cause.** Changes the user did not trigger
  (background refresh, live updates) settle in with a colour or opacity fade,
  never by moving what is under the pointer.
