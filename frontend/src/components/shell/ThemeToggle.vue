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
</script>

<template>
  <Button
    variant="ghost"
    size="icon"
    :aria-label="`Theme: ${label}. Click to cycle.`"
    @click="cycleTheme"
  >
    <Sun v-if="mode === 'light'" class="size-4 animate-in fade-in-0 zoom-in-75 ease-out animation-duration-fast" />
    <Moon v-else-if="mode === 'dark'" class="size-4 animate-in fade-in-0 zoom-in-75 ease-out animation-duration-fast" />
    <Monitor v-else class="size-4 animate-in fade-in-0 zoom-in-75 ease-out animation-duration-fast" />
    <span class="sr-only">{{ label }}</span>
  </Button>
</template>
