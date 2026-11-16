<script lang="ts" setup>
import type { ToasterProps } from "vue-sonner"
import { InfoIcon, Loader2Icon, OctagonXIcon, TriangleAlertIcon, XIcon } from "@lucide/vue"
import { Toaster as Sonner } from "vue-sonner"
import { cn } from "@/lib/utils"

const props = defineProps<ToasterProps>()
</script>

<template>
  <Sonner
    :class="cn('toaster group', props.class)"
    :style="{
      '--normal-bg': 'var(--color-popover)',
      '--normal-text': 'var(--color-popover-foreground)',
      '--normal-border': 'var(--color-border)',
      '--border-radius': 'var(--radius)',
    }"
    v-bind="props"
  >
    <template #success-icon>
      <svg class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="12" cy="12" r="10" pathLength="1" class="wow-draw" />
        <path d="m9 12 2 2 4-4" pathLength="1" class="wow-draw delay-200" />
      </svg>
    </template>
    <template #info-icon>
      <InfoIcon class="size-4" />
    </template>
    <template #warning-icon>
      <TriangleAlertIcon class="size-4" />
    </template>
    <template #error-icon>
      <OctagonXIcon class="size-4" />
    </template>
    <template #loading-icon>
      <div>
        <Loader2Icon class="size-4 animate-spin" />
      </div>
    </template>
    <template #close-icon>
      <XIcon class="size-4" />
    </template>
  </Sonner>
</template>

<!-- Unscoped, and more specific than the toast timing rules in style.css, which this replaces while a toast is up.
     It would outrank the reduced-motion cut the same way, hence the media query. -->
<style>
@media (prefers-reduced-motion: no-preference) {
  [data-sonner-toaster] [data-sonner-toast]:not([data-removed="true"]) {
    transition-duration: var(--transition-duration-slower) !important;
    transition-timing-function: var(--ease-spring) !important;
  }
}

/* The sheen moves the gradient, not the pseudo-element: clipping the toast would cut off sonner's hover bridge between stacked toasts. */
[data-sonner-toaster] [data-sonner-toast][data-type="success"]:not([data-removed="true"], [data-swiping="true"])::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: inherit;
  pointer-events: none;
  background: linear-gradient(105deg, transparent 35%, color-mix(in oklab, var(--color-wow-2) 40%, transparent) 50%, transparent 65%) 0 0 / 300% 100% no-repeat;
  animation: toast-sheen 900ms var(--ease-out) 250ms backwards;
}
@keyframes toast-sheen { from { background-position: 100% 0; } }
</style>
