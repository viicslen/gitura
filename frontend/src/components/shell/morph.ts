import { ref } from 'vue'
import { reducedMotion, viewTransition } from '@/components/motion/wow'

/** The named view transition running now, mirrored on `html[data-vt]` so its `::view-transition-*` rules can scope on it. */
export const morphing = ref<string | null>(null)

export const canMorph = (): boolean => !reducedMotion() && 'startViewTransition' in document

let runs = 0

/** Makes `change` as the view transition `name`; where there are no view transitions the change just happens, synchronously. */
export async function morph(name: string, change: () => void | Promise<void>): Promise<void> {
  if (!canMorph()) {
    await change()
    return
  }
  const root = document.documentElement
  const run = ++runs
  morphing.value = root.dataset.vt = name
  try {
    await viewTransition(change)
  } finally {
    // A newer transition skips this one and owns the flags from then on.
    if (run === runs) {
      morphing.value = null
      if (root.dataset.vt === name) delete root.dataset.vt
    }
  }
}
