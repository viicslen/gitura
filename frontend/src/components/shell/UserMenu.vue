<script setup lang="ts">
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useTheme } from '@/composables/useTheme'

defineProps<{ login: string; avatarSrc?: string }>()
const emit = defineEmits<{ (e: 'settings'): void; (e: 'logout'): void }>()
const { mode } = useTheme()
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
      <DropdownMenuLabel class="font-normal text-muted-foreground">{{ login }}</DropdownMenuLabel>
      <DropdownMenuSeparator />
      <DropdownMenuLabel class="text-xs text-muted-foreground">Theme</DropdownMenuLabel>
      <DropdownMenuRadioGroup v-model="mode">
        <DropdownMenuRadioItem value="auto">System</DropdownMenuRadioItem>
        <DropdownMenuRadioItem value="light">Light</DropdownMenuRadioItem>
        <DropdownMenuRadioItem value="dark">Dark</DropdownMenuRadioItem>
      </DropdownMenuRadioGroup>
      <DropdownMenuSeparator />
      <DropdownMenuItem @click="emit('settings')">Settings</DropdownMenuItem>
      <DropdownMenuItem @click="emit('logout')">Sign out</DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
