<script setup lang="ts">
// While reviewing, the rail also switches the review view.
import { FileDiff, GitPullRequest, LayoutList, MessagesSquare } from 'lucide-vue-next'
import UserMenu from './UserMenu.vue'
import { prView } from './reviewView'
import type { ReviewNavMeta } from './types'

defineProps<{ meta: ReviewNavMeta | null; login: string; avatarSrc?: string }>()
const emit = defineEmits<{ (e: 'home'): void; (e: 'settings'): void; (e: 'logout'): void }>()

const VIEWS = [
  { value: 'overview', label: 'Overview', icon: LayoutList },
  { value: 'conversation', label: 'Conversation', icon: MessagesSquare },
  { value: 'files', label: 'Files changed', icon: FileDiff },
] as const

const item = 'grid place-items-center h-9 w-9 rounded-md'
const idle = 'text-muted-foreground hover:bg-accent hover:text-foreground'
</script>

<template>
  <nav class="w-12 border-r border-border flex flex-col items-center shrink-0 bg-muted/30 select-none" aria-label="App navigation">
    <div class="h-10 w-full grid place-items-center text-xs font-bold tracking-tight" style="--wails-draggable: drag">g</div>
    <div class="flex flex-col items-center gap-1 mt-1">
      <button
        :class="[item, !meta ? 'bg-accent text-foreground' : idle]"
        aria-label="Pull requests"
        title="Pull requests"
        @click="emit('home')"
      ><GitPullRequest class="h-4 w-4" /></button>
      <template v-if="meta">
        <div class="h-px w-6 bg-border my-1" />
        <button
          v-for="v in VIEWS"
          :key="v.value"
          :class="[item, prView === v.value ? 'bg-accent text-foreground' : idle]"
          :aria-label="v.label"
          :aria-current="prView === v.value ? 'page' : undefined"
          :title="v.label"
          @click="prView = v.value"
        ><component :is="v.icon" class="h-4 w-4" /></button>
      </template>
    </div>
    <div class="flex-1" />
    <div class="pb-3">
      <UserMenu :login="login" :avatar-src="avatarSrc" @settings="emit('settings')" @logout="emit('logout')" />
    </div>
  </nav>
</template>
