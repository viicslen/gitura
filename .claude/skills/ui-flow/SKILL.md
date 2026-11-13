---
name: ui-flow
description: Motion that makes a UI flow — every visible state change flows into the next state (transition, animation, smoothing) as far as its budget allows, sized by how often the user triggers it and what it costs to render. Use for a full-app motion sweep, when improving the feel of existing frontend/UI/UX, and when building any new UI, section or page.
---

# UI flow

An **edge** is one visible change from one state to another. An edge **flows**
when the change is no surprise: things arrive from somewhere, leave to
somewhere, and what they displace glides out of the way. A **pop** is the
opposite: one frame the old state, the next frame the new one.

Every edge flows as far up the [ladder](#the-ladder) as its **budget** allows:
how often the user triggers it, and what it costs to render. A flow nobody can
see is still a pop, and a flow that is slow, repeated or janky is worse than
one.

## The ladder

Each edge sits on the highest rung it can afford:

1. **Glide** — the new state enters, the old one exits, and everything they
   displace moves to its new place over the same duration.
2. **Crossfade** — old and new overlap in a box that keeps its size; nothing
   around it moves.
3. **Settle** — the old state is gone in one frame; the new one fades or
   settles in.
4. **Pop** — nothing.

Glide exists only where the change displaces something. Where old and new
share a box and nothing around it moves, crossfade is the top rung.

Each rung down is paid for with one of four reasons, written beside the edge:

- **Frequency** — the edge is in the constant tier of the time budget (pointer
  entering a row, typing, dragging, a held or repeated key). Constant-tier
  edges pop. An edge that fails the 50th-time test moves one tier faster; it
  does not drop a rung.
- **Cost** — the higher rung was built and measured against the rung below, in
  the same session, interleaved, several runs each, and it drops more frames.
  Say what was measured. Step down one rung at a time. A long first frame from
  mounting heavy content belongs to the content, not to the motion: it is a
  note on the row, not a reason.
- **Waiting** — the higher rung would hold back content or input the user is
  waiting for. An exit that would delay what replaces it overlaps it instead
  (crossfade); only when overlap is impossible does the edge settle.
- **Cause** — the user did not trigger it (a background refresh, a live
  update). It settles with opacity or colour and never moves what is under the
  pointer.

Effort is never a reason. If flowing an edge needs a wrapper element, an
element kept mounted, a separate state flag or a keyed container, that
structure is the work.

One exception is not a rung down: a change **rides along** when it happens
inside a larger edge that is already flowing (the contents of an entering
panel). The outer element carries the motion; the inner ones add none.

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
  turns every edge into a pop, exempting progress indicators.
- **A way to run every screen.** The inventory and the verification both need
  the app running with realistic data, including a large case of whatever it
  lists (a long list, a big document). When the backend cannot be started,
  serve the frontend alone and stub its transport (`fetch`, the IPC bridge)
  with fixtures.
- **Renderer.** Know the weakest target: an embedded webview (Tauri, Wails,
  Electron on Linux), a low-end phone. Cost is measured there, or on the
  closest thing to it.

Done when a token exists for each tier, one known overlay visibly animates in
and out, the reduced-motion rule is in place, and every screen of the surface
can be reached in a running app.

### 2. Inventory

List every edge in the surface you are working on. Which surface depends on the
job:

- **Sweep** — the whole app, one feature area at a time.
- **Improving existing UI** — the surface you were asked about, plus every
  motion already on it (audit those against the budget).
- **New UI** — write the states first (empty, loading, loaded, error, partial,
  disabled, selected, open), then every edge between them, before any markup.

The inventory has two sources, and it needs both.

**Hunt pops in the running app.** Load [`scripts/pop-detector.js`](scripts/pop-detector.js)
into the page and use every control on the surface: every button, tab, row,
toggle, menu, form, shortcut, each loading, empty and error state, each list
getting an item added and removed. `__pops.report()` then lists every element
that appeared, vanished, was swapped or jumped with nothing animating it. Each
line is an edge. Drive hover and focus with real pointer and keyboard input;
scripted events do not trigger them.

**Read the code** for the edges a run will not show you:

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

Edges that share a rung and a reason are one row ("row hovers", "kit overlays").

Done when every control on the surface has been used with the detector
running, every item in the list has been checked against the code, and each
edge found is on a row.

### 3. Place each edge on the ladder

Give every row its rung, and for every rung below glide, the reason and what
the higher rung would have cost. An edge that already flows is audited the
same way: over budget, on a costly property, replaying when it should not, or
sitting lower than it can afford are all rows to fix.

Two rules settle most hard rows:

- **One edge, one animation.** The outermost element that changes carries the
  motion; everything inside it rides along.
- **Several triggers, one budget.** An edge reachable by pointer and keyboard,
  or by first load and by a filter toggle, takes the budget of its most
  frequent trigger, unless the code can tell the triggers apart: then each
  gets its own (typing pops the list, a click glides it).

State logic is in scope. The worst flow defects are state bugs: a loader that
replaces content on refresh, a flag cleared one step early, an error wiped the
moment its retry starts. Fix them, and report every behaviour change
separately from the motion.

Done when no row is without a rung, every rung below glide names its reason,
and the only pops are constant-tier edges or carry a measurement.

### 4. Implement

Read [`RECIPES.md`](RECIPES.md) before writing motion code: it holds the
pattern for each kind of edge, the traps each pattern has, and notes per
stack. Reuse the framework's and component kit's own transition mechanism
before adding a library.

### 5. Verify

Still screenshots cannot show motion. Verify in the running app:

- **Hunt again.** Reload, load the detector, use every control on the surface
  once more. Every line left in `__pops.report()` matches a row whose rung is
  pop, or a settle whose vanished half was paid for.
- **Budget.** Trigger each edge and read `document.getAnimations()`: the
  properties are in the free/cheap tiers unless the row says otherwise, and the
  duration is inside its tier.
- **Rapid-fire.** Trigger each toggling edge five times fast and reverse it
  mid-flight. Nothing queues, lags, flashes or ends in the wrong state.
- **Cost.** Run every glide and every costly-tier animation against the large
  case and sample frame times across the animation, with the motion on and off
  in turn (the detector's `jank` lines are a hint, not a measurement). An edge
  that drops more frames than the rung below it goes down one rung, with the
  numbers on its row.
- **Reduced motion.** Emulate it; every edge still reads as a pop.

Done when the second hunt reports nothing the rows do not explain. What could
not be run is reported as unverified, edge by edge, never folded into "done".

## Budget

### Time: how often is it triggered?

| Tier | Frequency | Examples | Duration |
| --- | --- | --- | --- |
| **constant** | many times a minute | pointer entering a row, typing, scrubbing, drag, held or repeated keys | 0ms; pointer-out may ease over the smallest token |
| **frequent** | several times a minute | tabs, toggles, checkboxes, menus, selecting an item, inline disclosure | 120–180ms |
| **occasional** | a few times a session | dialogs, drawers, side panels, page changes, toasts | 200–300ms |
| **rare** | about once | first load, onboarding, success moment, empty → first content | up to 500ms |

- An exit uses the next token down from its enter.
- Within a tier, small elements and short travel take the low end.
- **Visible or nothing.** On anything larger than an icon, opacity alone under
  about 120ms cannot be seen. Pair a short fade with 4–8px of travel or a
  slight scale, or give it the top of its tier.
- Run the **50th-time test**: picture the user triggering it for the fiftieth
  time today. If it would grate, move it one tier faster.
- Easing: `ease-out` for anything answering user input, exits included (an
  ease-in exit starts slow and reads as lag); `ease-in-out` for something
  moving between two on-screen positions; `linear` only for continuous motion
  (spinners, progress).

### Cost: what does a frame cost?

| Tier | Properties | Use |
| --- | --- | --- |
| **free** | `transform`, `opacity` | anywhere the renderer composites on the GPU |
| **cheap** | `color`, `background-color`, `border-color`, `outline`, small `box-shadow` | small elements |
| **costly** | anything that moves other elements: `width`, `height`, `margin`, `padding`, `top`/`left`, `grid-template-*`, `font-size` | when the measured frame time holds |
| **heavy** | `filter`, `backdrop-filter`, large shadows, anything repainting a large area | static; fade a wrapper instead |

- Name the properties in every transition (`transition-colors`,
  `transition-[opacity,transform]`); `all` picks up costly ones by accident.
- Costly is a measurement, not a ban. A size animation that reflows a few
  hundred elements holds 60fps on most targets; one that re-lays-out thousands
  of rows, or triggers script that measures them, usually does not. Measure
  before stepping down, and prefer moving the displaced elements with
  `transform` (FLIP) to giving up the glide.
- More than about a dozen elements moving independently: animate their
  container.
- Entering lists animate only items added after first render. First render of
  a long or virtualised list appears as one block.
- Streaming content (logs, tokens, terminal output) appends without animation,
  and stays outside any container that measures its children to animate them.

## What good flow obeys

- **Stable layout first.** An unannounced layout shift is the worst pop.
  Reserve space for what is loading, match skeletons to the final layout, use
  tabular numerals for changing numbers, and keep persistent chrome still.
  When layout has to change, what is displaced glides.
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
