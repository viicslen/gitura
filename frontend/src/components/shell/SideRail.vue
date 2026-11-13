<script setup lang="ts">
// While reviewing, the rail also switches the review view.
import { FileDiff, GitPullRequest, LayoutList, MessagesSquare } from '@lucide/vue'
import logoUrl from '@/assets/images/logo.svg'
import UserMenu from './UserMenu.vue'
import { useReviewTabs } from '@/composables/useReviewTabs'
import type { PRRef } from '@/types/pr'

const { reviewTab, tabBadges } = useReviewTabs()

defineProps<{ meta: PRRef | null; login: string; avatarSrc?: string; focused?: boolean }>()
const emit = defineEmits<{ (e: 'home'): void; (e: 'settings'): void; (e: 'logout'): void }>()

const VIEWS = [
  { value: 'overview', label: 'Overview', icon: LayoutList, counted: 'unresolved reviews' },
  { value: 'conversation', label: 'Conversation', icon: MessagesSquare, counted: 'unresolved threads' },
  { value: 'files', label: 'Files changed', icon: FileDiff, counted: 'files not viewed' },
] as const

function viewLabel(v: (typeof VIEWS)[number]): string {
  const count = tabBadges[v.value]
  return count ? `${v.label} (${count} ${v.counted})` : v.label
}

const item = 'relative grid place-items-center h-9 w-9 rounded-md'
const idle = 'text-muted-foreground hover:bg-accent hover:text-foreground'
</script>

<template>
  <nav class="w-12 border-r border-border flex flex-col items-center shrink-0 bg-muted/30 select-none" aria-label="App navigation">
    <div class="h-10 w-full grid place-items-center" style="--wails-draggable: drag">
      <img :src="logoUrl" alt="gitura" class="h-6 w-6" draggable="false" />
    </div>
    <div class="flex flex-col items-center gap-1 mt-1">
      <button
        v-if="!focused"
        :class="[item, !meta ? 'bg-accent text-foreground' : idle]"
        aria-label="Pull requests"
        title="Pull requests"
        @click="emit('home')"
      ><GitPullRequest class="h-4 w-4" /></button>
      <template v-if="meta">
        <div v-if="!focused" class="h-px w-6 bg-border my-1 animate-in fade-in-0" />
        <button
          v-for="v in VIEWS"
          :key="v.value"
          :class="[item, 'animate-in fade-in-0', reviewTab === v.value ? 'bg-accent text-foreground' : idle]"
          :aria-label="viewLabel(v)"
          :aria-current="reviewTab === v.value ? 'page' : undefined"
          :title="viewLabel(v)"
          @click="reviewTab = v.value"
        >
          <component :is="v.icon" class="h-4 w-4" />
          <Transition
            enter-active-class="animate-in fade-in-0 zoom-in-75 ease-out animation-duration-fast"
            leave-active-class="animate-out fade-out-0 zoom-out-75 ease-out animation-duration-faster"
          >
            <span
              v-if="tabBadges[v.value]"
              class="absolute -top-0.5 -right-0.5 min-w-4 h-4 px-1 rounded-full bg-primary text-primary-foreground text-[0.625rem] font-semibold leading-4 text-center tabular-nums"
              aria-hidden="true"
            >{{ tabBadges[v.value] > 99 ? '99+' : tabBadges[v.value] }}</span>
          </Transition>
        </button>
      </template>
    </div>
    <div class="flex-1" />
    <div class="pb-3">
      <UserMenu :login="login" :avatar-src="avatarSrc" @settings="emit('settings')" @logout="emit('logout')" />
    </div>
  </nav>
</template>
