/** Script half of the showpiece motion; the CSS half is the `wow-*` utilities in style.css. */
import { nextTick, type Directive } from 'vue'

/** True too where the question cannot be asked (tests), so effects stay off there. */
export const reducedMotion = (): boolean => globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? true

/**
 * Publishes the pointer's place over the element: `--mx`/`--my` in px for `wow-spotlight`, `--px`/`--py`
 * from -0.5 to 0.5 for `wow-tilt` (back to the centre when the pointer leaves).
 */
export const vPointer: Directive<HTMLElement> = {
  mounted(el) {
    el.addEventListener('pointermove', (e) => {
      const box = el.getBoundingClientRect()
      el.style.setProperty('--mx', `${e.clientX - box.x}px`)
      el.style.setProperty('--my', `${e.clientY - box.y}px`)
      el.style.setProperty('--px', String((e.clientX - box.x) / box.width - 0.5))
      el.style.setProperty('--py', String((e.clientY - box.y) / box.height - 0.5))
    }, { passive: true })
    el.addEventListener('pointerleave', () => {
      el.style.setProperty('--px', '0')
      el.style.setProperty('--py', '0')
    })
  },
}

/**
 * Morphs the screen before `change` into the one after it, pairing elements that share a
 * `view-transition-name`. Where the webview has no view transitions the change just happens.
 */
export async function viewTransition(change: () => void | Promise<void>): Promise<void> {
  if (reducedMotion() || !document.startViewTransition) {
    await change()
    return
  }
  await document.startViewTransition(async () => {
    await change()
    await nextTick()
  }).finished
}

/** A burst of confetti from the middle of `from`, or from the upper middle of the window. */
export function confetti(from?: Element | null, count = 110): void {
  if (reducedMotion()) return
  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  canvas.className = 'pointer-events-none fixed inset-0 z-100 size-full'
  canvas.setAttribute('aria-hidden', 'true')
  canvas.width = innerWidth
  canvas.height = innerHeight
  document.body.append(canvas)

  const box = from?.getBoundingClientRect()
  const x = box ? box.x + box.width / 2 : innerWidth / 2
  const y = box ? box.y + box.height / 2 : innerHeight / 3
  const theme = getComputedStyle(document.documentElement)
  const colours = ['--color-wow-1', '--color-wow-2', '--color-wow-3'].map((name) => theme.getPropertyValue(name)).concat(theme.getPropertyValue('--color-foreground'))
  const bits = Array.from({ length: count }, (_, i) => {
    const angle = Math.random() * Math.PI * 2
    const speed = 5 + Math.random() * 11
    return { x, y, vx: Math.cos(angle) * speed, vy: Math.sin(angle) * speed - 6, spin: Math.random() * 6, size: 5 + Math.random() * 6, colour: colours[i % colours.length] }
  })

  const LIFE = 1800
  const start = performance.now()
  let last = start
  const frame = (now: number): void => {
    const age = (now - start) / LIFE
    if (age >= 1) {
      canvas.remove()
      return
    }
    const step = Math.min((now - last) / 16.7, 3)
    last = now
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    ctx.globalAlpha = 1 - age * age
    for (const bit of bits) {
      bit.vy += 0.35 * step
      bit.vx *= 0.985 ** step
      bit.x += bit.vx * step
      bit.y += bit.vy * step
      bit.spin += 0.2 * step
      ctx.save()
      ctx.translate(bit.x, bit.y)
      ctx.rotate(bit.spin)
      ctx.fillStyle = bit.colour
      ctx.fillRect(-bit.size / 2, -bit.size / 4, bit.size, bit.size / 2)
      ctx.restore()
    }
    requestAnimationFrame(frame)
  }
  requestAnimationFrame(frame)
}
