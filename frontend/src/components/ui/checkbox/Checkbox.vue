<script setup lang="ts">
import type { CheckboxRootEmits, CheckboxRootProps } from 'reka-ui'
import { ref, watch, type HTMLAttributes } from 'vue'
import { CheckboxIndicator, CheckboxRoot, useForwardPropsEmits } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<CheckboxRootProps & { class?: HTMLAttributes['class'] }>()
const emits = defineEmits<CheckboxRootEmits>()

const forwarded = useForwardPropsEmits(props, emits)

// Only a change draws the check; a box that mounts checked just shows it.
const changed = ref(false)
watch(() => props.modelValue, () => {
  changed.value = true
})
</script>

<template>
  <CheckboxRoot
    data-slot="checkbox"
    v-bind="forwarded"
    :class="cn(
      'peer size-4 shrink-0 rounded-sm border border-primary ring-offset-background transition-[background-color,opacity] duration-fast focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-primary data-[state=checked]:text-primary-foreground',
      props.class,
    )"
  >
    <CheckboxIndicator class="flex items-center justify-center text-current" :class="changed && 'wow-pop'">
      <svg class="size-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M4 12l5 5L20 6" pathLength="1" :class="changed && 'wow-draw'" />
      </svg>
    </CheckboxIndicator>
  </CheckboxRoot>
</template>
