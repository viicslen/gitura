<script setup lang="ts">
import { Loader2 } from '@lucide/vue'

defineProps<{ busy: boolean }>()
</script>

<template>
  <!-- The label keeps the button's width, and the spinner replaces it only once the wait is real. -->
  <span class="grid place-items-center">
    <span class="[grid-area:1/1]" :class="busy && 'animate-out fade-out-0 animation-duration-fast delay-200 fill-mode-forwards'"><slot /></span>
    <span v-if="busy" class="[grid-area:1/1] animate-in fade-in-0 animation-duration-fast delay-200 fill-mode-backwards">
      <Loader2 class="size-4 animate-spin" aria-hidden="true" />
    </span>
  </span>
</template>
