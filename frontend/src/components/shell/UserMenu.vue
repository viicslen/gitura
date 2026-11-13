<script setup lang="ts">
import { DropdownMenuRadioItem } from 'reka-ui'
import { Monitor, Moon, Sun } from '@lucide/vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useTheme } from '@/composables/useTheme'

defineProps<{ login: string; avatarSrc?: string }>()
const emit = defineEmits<{ (e: 'settings'): void; (e: 'logout'): void }>()
const { mode } = useTheme()

const themes = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'auto', label: 'System', icon: Monitor },
] as const

function setMode(value: unknown): void {
  mode.value = themes.find((t) => t.value === value)?.value ?? 'auto'
}
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger
      class="rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring"
      :aria-label="`Account menu for ${login}`"
    >
      <img v-if="avatarSrc" :src="avatarSrc" :alt="login" class="h-7 w-7 rounded-full" />
      <span v-else class="grid h-7 w-7 place-items-center rounded-full bg-muted text-xs">{{ login.slice(0, 1) }}</span>
    </DropdownMenuTrigger>
    <DropdownMenuContent side="right" align="end" class="w-52">
      <DropdownMenuRadioGroup :model-value="mode" class="relative flex gap-1 p-1" aria-label="Theme" @update:model-value="setMode">
        <!-- One third of the row inside p-1 and two gap-1 gaps, slid onto the checked theme. -->
        <span
          class="pointer-events-none absolute inset-y-1 left-1 w-[calc((100%-1rem)/3)] rounded-md border border-ring transition-transform duration-fast ease-in-out"
          :style="{ transform: `translateX(calc(${themes.findIndex((t) => t.value === mode)} * (100% + 0.25rem)))` }"
          aria-hidden="true"
        />
        <DropdownMenuRadioItem
          v-for="t in themes"
          :key="t.value"
          :value="t.value"
          :aria-label="t.label"
          :title="t.label"
          class="grid h-9 flex-1 place-items-center rounded-md text-muted-foreground outline-hidden select-none hover:bg-accent hover:text-foreground focus:bg-accent focus:text-foreground data-[state=checked]:text-foreground"
          @select.prevent
        >
          <component :is="t.icon" class="size-4" aria-hidden="true" />
        </DropdownMenuRadioItem>
      </DropdownMenuRadioGroup>
      <DropdownMenuSeparator />
      <DropdownMenuLabel class="font-normal text-muted-foreground">{{ login }}</DropdownMenuLabel>
      <DropdownMenuItem @click="emit('settings')">Settings</DropdownMenuItem>
      <DropdownMenuItem @click="emit('logout')">Sign out</DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
