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
        enter-active-class="animate-in fade-in-0 zoom-in-75 animation-duration-fast"
        leave-active-class="animate-out fade-out-0 zoom-out-75 animation-duration-faster"
      >
        <component :is="icons[mode]" :key="mode" class="col-start-1 row-start-1 size-4" />
      </Transition>
    </span>
    <span class="sr-only">{{ label }}</span>
  </Button>
</template>
