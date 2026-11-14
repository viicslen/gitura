<script setup lang="ts">
import { computed } from 'vue'
import { Sun, Moon, Monitor } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { useTheme } from '@/composables/useTheme'

const { mode, cycleTheme } = useTheme()

const label = computed(() => {
  if (mode.value === 'light') return 'Light'
  if (mode.value === 'dark') return 'Dark'
  return 'System'
})
const icons = { light: Sun, dark: Moon, auto: Monitor }
</script>

<template>
  <Button
    variant="ghost"
    size="icon"
    :aria-label="`Theme: ${label}. Click to cycle.`"
    @click="cycleTheme"
  >
    <span class="grid">
      <Transition
        enter-active-class="animate-in fade-in-0 zoom-in-50 spin-in-90 ease-spring animation-duration-slower"
        leave-active-class="animate-out fade-out-0 zoom-out-50 spin-out-90 animation-duration-fast"
      >
        <component :is="icons[mode]" :key="mode" class="col-start-1 row-start-1 size-4" />
      </Transition>
    </span>
    <span class="sr-only">{{ label }}</span>
  </Button>
</template>

<!-- The `theme` view transition, started by setAppearance. Unscoped: view-transition pseudo-elements hang off <html>. -->
<style>
@property --theme-hole {
  syntax: "<length>";
  inherits: false;
  initial-value: 0px;
}

/* Clicks reach the page under the snapshots while the circle grows. */
html[data-vt="theme"]::view-transition {
  pointer-events: none;
}
/* The new theme is clipped to a growing circle and the old one has a slightly larger hole punched in it;
   the accent colours behind both show through the gap as the circle's rim. */
html[data-vt="theme"]::view-transition-image-pair(root) {
  background: conic-gradient(at var(--theme-x) var(--theme-y), var(--color-wow-1), var(--color-wow-2), var(--color-wow-3), var(--color-wow-1));
}
html[data-vt="theme"]::view-transition-old(root),
html[data-vt="theme"]::view-transition-new(root) {
  mix-blend-mode: normal;
  animation: theme-reveal var(--transition-duration-show) ease-in-out both;
}
html[data-vt="theme"]::view-transition-old(root) {
  animation-name: theme-hole;
  mask-image: radial-gradient(circle at var(--theme-x) var(--theme-y), transparent calc(var(--theme-hole) + 0.5rem), black calc(var(--theme-hole) + 0.5rem + 1px));
}
@keyframes theme-reveal {
  from { clip-path: circle(0 at var(--theme-x) var(--theme-y)); }
  to { clip-path: circle(var(--theme-r) at var(--theme-x) var(--theme-y)); }
}
@keyframes theme-hole { to { --theme-hole: var(--theme-r); } }
</style>
