<script setup lang="ts">
import { computed } from 'vue'
import { Sun, Moon, Monitor } from 'lucide-vue-next'
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
    <Sun v-if="mode === 'light'" class="size-4" />
    <Moon v-else-if="mode === 'dark'" class="size-4" />
    <Monitor v-else class="size-4" />
    <span class="sr-only">{{ label }}</span>
  </Button>
</template>
