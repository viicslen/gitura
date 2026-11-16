<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { reducedMotion } from './wow'

/** A whole number that rolls to each new value, from zero the first time. */
const props = defineProps<{ value: number }>()

const shown = ref(reducedMotion() ? props.value : 0)
let frame = 0

watch(() => props.value, (to) => {
  cancelAnimationFrame(frame)
  if (reducedMotion()) {
    shown.value = to
    return
  }
  const from = shown.value
  const start = performance.now()
  const roll = (now: number): void => {
    const t = Math.min((now - start) / 700, 1)
    shown.value = Math.round(from + (to - from) * (1 - (1 - t) ** 3))
    if (t < 1) frame = requestAnimationFrame(roll)
  }
  frame = requestAnimationFrame(roll)
}, { immediate: true })

onBeforeUnmount(() => cancelAnimationFrame(frame))
</script>

<template>
  <span class="tabular-nums">{{ shown }}</span>
</template>
