<script setup lang="ts">
/** A check that draws itself each time it appears; `ring` draws a circle around it first. Sized and coloured by the parent. */
defineProps<{ ring?: boolean }>()
</script>

<template>
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <circle v-if="ring" class="wow-draw" cx="12" cy="12" r="10" pathLength="1" />
    <path class="wow-draw" :class="ring && 'after-ring'" d="M7 12.5l3.5 3.5L17 9" pathLength="1" />
  </svg>
</template>

<style scoped>
.after-ring {
  animation-delay: 250ms;
}
</style>
