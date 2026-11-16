<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <div
    data-slot="skeleton"
    :class="cn('skeleton relative animate-pulse overflow-hidden rounded-md bg-muted', props.class)"
  />
</template>

<style scoped>
.skeleton::after {
  content: "";
  position: absolute;
  inset: 0;
  translate: -100% 0;
  background: linear-gradient(
    100deg,
    transparent 25%,
    color-mix(in oklab, var(--color-wow-1) 35%, transparent) 40%,
    color-mix(in oklab, var(--color-wow-2) 45%, transparent) 50%,
    color-mix(in oklab, var(--color-wow-3) 35%, transparent) 60%,
    transparent 75%
  );
  animation: wow-sheen 1.6s linear infinite;
  /* A caller that staggers its skeletons staggers the sweep with them. */
  animation-delay: inherit;
}
</style>
