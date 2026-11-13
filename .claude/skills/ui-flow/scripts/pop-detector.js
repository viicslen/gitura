/* pop-detector.js: logs every visible change that happened without motion.
   Paste into the console of the running app, or load it with a <script> tag, then use the app.

     appear / vanish   an element was added or removed with nothing animating it or around it
     show / hide       an element's inline `display` flipped with nothing animating it
     shift             an element jumped to a new position in a single frame
     swap              a container's contents were replaced with nothing animating it
     jank              a frame took longer than 34ms while an animation was running (only when the
                       tab is really painting; a background tab reports none)

   __pops.report()          grouped log as text
   __pops.log               raw entries
   __pops.clear()           reset, e.g. after the first load
   __pops.ignore(selector)  silence a subtree

   Colour-only animations do not count as motion: a button whose hover colour is easing
   when it is removed still vanished. */
(() => {
  if (window.__pops) return;

  const MIN_SIDE = 12, MIN_AREA = 400, JANK_MS = 34, KEEP_FRAMES = 8, JUMP_PX = 6, MEASURES_PER_FRAME = 40;
  const COLOUR_ONLY = /color|shadow|outline|fill|stroke|decoration|caret/i;
  const META = new Set(['offset', 'computedOffset', 'easing', 'composite']);
  const log = [];
  const ignored = ['script', 'style', 'link', 'head', 'title', 'noscript'];
  const frames = [];
  const movesCache = new WeakMap();
  const rects = new WeakMap();
  let lastInput = null, measured = 0, lastFrame = performance.now(), lastWasPainted = false;

  const nextFrame = fn => {
    let done = false;
    const run = painted => { if (!done) { done = true; fn(painted); } };
    requestAnimationFrame(() => run(true));
    setTimeout(() => run(false), 50); // rAF never fires in a hidden tab
  };

  const moves = a => {
    if (movesCache.has(a)) return movesCache.get(a);
    let props = [];
    if (a.transitionProperty) props = [a.transitionProperty];
    else if (a.effect && a.effect.getKeyframes) props = [...new Set(a.effect.getKeyframes().flatMap(k => Object.keys(k)))].filter(p => !META.has(p));
    const result = props.some(p => !COLOUR_ONLY.test(p));
    movesCache.set(a, result);
    return result;
  };
  const movingNow = () => new Set(document.getAnimations().filter(moves).map(a => a.effect && a.effect.target).filter(Boolean));

  const flows = (el, sets) => {
    const inSets = n => sets.some(s => s.has(n));
    for (let n = el; n && n.nodeType === 1; n = n.parentElement) if (inSets(n)) return true;
    const sized = [...el.children].filter(c => { const r = c.getBoundingClientRect(); return r.width * r.height >= MIN_AREA; });
    return sized.length > 0 && sized.every(inSets);
  };

  const isIgnored = el => !el.closest || !!el.ownerSVGElement || !!el.closest(ignored.join(','));
  const tooSmall = r => r.width < MIN_SIDE || r.height < MIN_SIDE || r.width * r.height < MIN_AREA;
  const offscreen = r => r.bottom < 0 || r.right < 0 || r.top > innerHeight || r.left > innerWidth;
  const visible = el => (el.checkVisibility ? el.checkVisibility({ opacityProperty: true, visibilityProperty: true }) : true);

  const describe = el => {
    if (!el || el.nodeType !== 1) return String(el);
    const cls = [...el.classList].filter(c => /^[a-z][a-z0-9-]*$/i.test(c)).slice(0, 3).join('.');
    const role = el.getAttribute('role'), label = el.getAttribute('aria-label');
    const text = (el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 40);
    return el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') + (cls ? '.' + cls : '') +
      (role ? `[role=${role}]` : '') + (label ? `[aria-label="${label}"]` : '') + (text ? ` "${text}"` : '');
  };

  const record = (kind, what, r, extra) => {
    const now = performance.now();
    log.push({
      kind, what,
      size: r ? `${Math.round(r.width)}x${Math.round(r.height)}` : '?',
      input: lastInput ? `${lastInput.what} +${Math.round(now - lastInput.t)}ms` : 'no input yet',
      ...extra,
    });
  };

  for (const type of ['pointerdown', 'click', 'keydown']) {
    document.addEventListener(type, e => {
      const on = e.target && e.target.nodeType === 1 ? describe(e.target.closest('button,a,[role],input,textarea,select,summary') || e.target) : '';
      lastInput = { what: (type === 'keydown' ? `key ${e.key} on ` : `${type} on `) + on, t: performance.now() };
    }, true);
  }

  const frameLoop = painted => {
    const now = performance.now();
    const previous = frames[frames.length - 1];
    if (painted && lastWasPainted && now - lastFrame > JANK_MS && previous && previous.size) {
      record('jank', `${Math.round(now - lastFrame)}ms frame with ${previous.size} element(s) animating`, null);
    }
    lastWasPainted = !!painted;
    lastFrame = now;
    measured = 0;
    frames.push(movingNow());
    if (frames.length > KEEP_FRAMES) frames.shift();
    nextFrame(frameLoop);
  };
  frameLoop();

  const checkAppear = (el, kind, atInsert, entered) => nextFrame(() => nextFrame(() => {
    if (!el.isConnected || isIgnored(el) || !visible(el)) return;
    const r = el.getBoundingClientRect();
    rects.set(el, r);
    if (tooSmall(r) || offscreen(r)) return;
    const sets = [atInsert, ...frames, movingNow()];
    if (flows(el, sets)) return;
    if (entered && entered.every(c => !c.isConnected || flows(c, sets))) return;
    record(kind, describe(el), r);
  }));

  new MutationObserver(mutations => {
    const added = new Set();
    for (const m of mutations) {
      if (m.type === 'childList') {
        for (const n of m.addedNodes) if (n.nodeType === 1 && !isIgnored(n)) added.add(n);
      } else if (m.target.nodeType === 1 && !isIgnored(m.target)) {
        const el = m.target;
        const wasNone = /display:\s*none/.test(m.oldValue || ''), isNone = el.style.display === 'none';
        if (wasNone && !isNone) checkAppear(el, 'show', movingNow());
        else if (!wasNone && isNone && !flows(el.parentElement || el, [...frames]) && !frames.some(s => s.has(el))) {
          const r = rects.get(el);
          if (!r || !tooSmall(r)) record('hide', describe(el), r);
        }
      }
    }
    if (!added.size) return;
    const atInsert = movingNow();
    const byParent = new Map();
    for (const el of added) {
      let inner = false;
      for (let p = el.parentElement; p; p = p.parentElement) if (added.has(p)) { inner = true; break; }
      if (inner || !el.parentElement) continue;
      byParent.set(el.parentElement, [...(byParent.get(el.parentElement) || []), el]);
    }
    // Several children landing at once is one edge: the container's contents changed.
    for (const [parent, children] of byParent) {
      if (children.length > 2 && parent !== document.body) checkAppear(parent, 'swap', atInsert, children);
      else for (const el of children) checkAppear(el, 'appear', atInsert);
    }
  }).observe(document.documentElement, { childList: true, subtree: true, attributes: true, attributeFilter: ['style'], attributeOldValue: true });

  // Removal has to be caught before it happens: a detached node has no box to measure.
  const beforeRemoval = el => {
    if (!el || el.nodeType !== 1 || !el.isConnected || isIgnored(el) || ++measured > MEASURES_PER_FRAME || !visible(el)) return;
    const r = el.getBoundingClientRect();
    if (tooSmall(r) || offscreen(r)) return;
    if (flows(el, [...frames, movingNow()])) return;
    record('vanish', describe(el), r);
  };
  const removeChild = Node.prototype.removeChild;
  Node.prototype.removeChild = function (child) { beforeRemoval(child); return removeChild.call(this, child); };
  const remove = Element.prototype.remove;
  Element.prototype.remove = function () { beforeRemoval(this); return remove.call(this); };

  // A glide reports a shift on many consecutive frames; a jump reports exactly one.
  const shifting = new Map();
  try {
    new PerformanceObserver(list => {
      for (const entry of list.getEntries()) {
        for (const src of entry.sources || []) {
          if (!src.node || src.node.nodeType !== 1 || isIgnored(src.node)) continue;
          const dist = Math.hypot(src.currentRect.x - src.previousRect.x, src.currentRect.y - src.previousRect.y);
          const seen = shifting.get(src.node) || { count: 0, dist: 0 };
          shifting.set(src.node, { count: seen.count + 1, dist: Math.max(seen.dist, dist), rect: src.currentRect, last: performance.now() });
        }
      }
    }).observe({ type: 'layout-shift', buffered: false });
    setInterval(() => {
      const now = performance.now();
      const settled = [...shifting].filter(([, s]) => now - s.last >= 300);
      for (const [node] of settled) shifting.delete(node);
      const jumped = settled.filter(([node, s]) => s.count === 1 && s.dist >= JUMP_PX && node.isConnected && !tooSmall(s.rect));
      for (const [node, s] of jumped) {
        if (jumped.some(([other]) => other !== node && other.contains(node))) continue;
        record('shift', describe(node), s.rect, { moved: `${Math.round(s.dist)}px` });
      }
    }, 150);
  } catch { /* Layout Instability API is Chromium-only; the other kinds still work */ }

  window.__pops = {
    log,
    clear: () => { log.length = 0; },
    ignore: selector => { ignored.push(selector); },
    report: () => {
      const groups = new Map();
      for (const e of log) {
        const key = `${e.kind} | ${e.what}`;
        const g = groups.get(key) || { ...e, count: 0 };
        g.count += 1;
        groups.set(key, g);
      }
      return [...groups.values()]
        .map(g => `${g.kind.padEnd(6)} x${g.count}  ${g.what}  [${g.size}${g.moved ? ', moved ' + g.moved : ''}]  after ${g.input}`)
        .join('\n') || 'no pops';
    },
  };
})();
