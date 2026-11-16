<script setup lang="ts">
// While reviewing, the rail also switches the review view.
import { computed } from 'vue'
import { FileDiff, GitPullRequest, LayoutList, MessagesSquare } from '@lucide/vue'
import logoUrl from '@/assets/images/logo.svg'
import UserMenu from './UserMenu.vue'
import { useReviewTabs } from '@/composables/useReviewTabs'
import type { PRRef } from '@/types/pr'

const { reviewTab, tabBadges } = useReviewTabs()

const props = defineProps<{ meta: PRRef | null; login: string; avatarSrc?: string; focused?: boolean }>()
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

// Where the selection highlight sits: on "Pull requests", or on the active view below the divider (h-9 buttons, gap-1).
const selectionY = computed(() => {
  if (!props.meta) return '0'
  const views = `${VIEWS.findIndex((v) => v.value === reviewTab.value)} * 2.5rem`
  return props.focused ? `calc(${views})` : `calc(${views} + 3.25rem + 1px)`
})

const item = 'group relative grid place-items-center h-9 w-9 rounded-md wow-pop'
const icon = 'h-4 w-4 transition-transform duration-slow ease-spring group-active:scale-60'
const idle = 'text-muted-foreground hover:bg-accent hover:text-foreground'
</script>

<template>
  <nav class="w-12 border-r border-border flex flex-col items-center shrink-0 bg-muted/30 select-none" aria-label="App navigation">
    <!-- The --i of the rail's items start late, so they still land one by one after the sign-in reveal has reached the rail. -->
    <div class="h-10 w-full grid place-items-center wow-pop" :style="{ '--i': 4 }" style="--wails-draggable: drag">
      <img
        :src="logoUrl"
        alt="gitura"
        class="h-6 w-6 transition-[rotate,scale,filter] duration-show ease-spring hover:rotate-[360deg] hover:scale-125 hover:drop-shadow-[0_0_0.5rem_var(--color-wow-1)]"
        draggable="false"
      />
    </div>
    <div class="relative flex flex-col items-center gap-1 mt-1">
      <div
        v-if="meta || !focused"
        class="absolute top-0 left-0 h-9 w-9 rounded-md bg-accent transition-transform duration-slower ease-spring"
        :style="{ transform: `translateY(${selectionY})` }"
        aria-hidden="true"
      >
        <!-- Rebuilt on every move, so the glow flares while the highlight travels and dies away once it has landed. -->
        <span
          :key="selectionY"
          class="absolute inset-0 rounded-md bg-wow-1/30 shadow-[0_0_0.75rem_0.125rem_var(--color-wow-1)] ring-1 ring-wow-2 animate-out fade-out-0 fill-mode-forwards animation-duration-show ease-in"
        />
      </div>
      <button
        v-if="!focused"
        :class="[item, !meta ? 'text-foreground' : idle]"
        :style="{ '--i': 6 }"
        aria-label="Pull requests"
        title="Pull requests"
        @click="emit('home')"
      ><GitPullRequest :class="icon" /></button>
      <Transition
        enter-active-class="animate-in fade-in-0 slide-in-from-top-1"
        leave-active-class="animate-out fade-out-0 slide-out-to-top-1 animation-duration-fast"
      >
        <div v-if="meta" class="flex flex-col items-center gap-1">
          <div v-if="!focused" class="h-px w-6 bg-border my-1" />
          <button
            v-for="(v, i) in VIEWS"
            :key="v.value"
            :class="[item, reviewTab === v.value ? 'text-foreground' : idle]"
            :style="{ '--i': i * 2 }"
            :aria-label="viewLabel(v)"
            :aria-current="reviewTab === v.value ? 'page' : undefined"
            :title="viewLabel(v)"
            @click="reviewTab = v.value"
          >
            <component :is="v.icon" :class="icon" />
            <!-- Keyed on what it shows, so a changed count pops in as a new badge, without the delay --i gives its button. -->
            <Transition
              enter-active-class="wow-pop"
              leave-active-class="animate-out fade-out-0 zoom-out-75 ease-out animation-duration-faster"
            >
              <span
                v-if="tabBadges[v.value]"
                :key="Math.min(tabBadges[v.value], 100)"
                :style="{ '--i': 0 }"
                class="absolute -top-0.5 -right-0.5 min-w-4 h-4 px-1 rounded-full bg-primary text-primary-foreground text-[0.625rem] font-semibold leading-4 text-center tabular-nums"
                aria-hidden="true"
              >{{ tabBadges[v.value] > 99 ? '99+' : tabBadges[v.value] }}</span>
            </Transition>
          </button>
        </div>
      </Transition>
    </div>
    <div class="flex-1" />
    <div class="pb-3 wow-pop" :style="{ '--i': 8 }">
      <UserMenu :login="login" :avatar-src="avatarSrc" @settings="emit('settings')" @logout="emit('logout')" />
    </div>
  </nav>
</template>
