<template>
  <!-- Toaster must live outside the overflow-hidden root div; fixed positioning
       in WebKit (Wails) is clipped by overflow:hidden ancestors. Vue 3 fragments
       allow multiple root elements. -->
  <Toaster position="bottom-right" richColors />
  <div class="h-screen flex overflow-hidden bg-background text-foreground" :class="railLayout ? 'flex-row' : 'flex-col'">
    <SideRail
      v-if="railLayout"
      :meta="currentPage === 'review' ? reviewNavMeta : null"
      :login="authState.login"
      :avatar-src="authAvatarSrc"
      @home="goHome"
      @settings="settingsDialogOpen = true"
      @logout="handleLogout"
    />
    <div class="flex-1 min-w-0 flex flex-col overflow-hidden">
    <TitleStrip
      :meta="authState.is_authenticated && currentPage === 'review' ? reviewNavMeta : null"
      :fallback="authState.is_authenticated ? 'Pull Requests' : 'gitura'"
      @open-pr="openPRInBrowser"
    />

    <!-- Theme toggle shown in top-right corner when not authenticated -->
    <div v-if="!authState.is_authenticated" class="fixed top-12 right-4 z-50">
      <ThemeToggle />
    </div>

    <!-- Page routing -->
    <main class="flex-1 overflow-hidden">
      <AuthPage v-if="!authState.is_authenticated" />
      <template v-else>
        <!-- KeepAlive preserves PRPage scroll position and filter state when navigating to review -->
        <KeepAlive>
          <PRPage
            v-if="currentPage === 'pr'"
            @open-review="handleOpenReview"
          />
        </KeepAlive>
        <ReviewPage
          v-if="currentPage === 'review' && selectedPRItem"
          :pr-item="selectedPRItem"
          @update-pr-meta="handleUpdatePRMeta"
        />
      </template>
    </main>

    <Dialog :open="settingsDialogOpen" @update:open="settingsDialogOpen = $event">
      <DialogContent class="max-w-2xl max-h-[85vh] overflow-y-auto overflow-x-hidden p-0 border border-border">
        <SettingsPage />
      </DialogContent>
    </Dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useEventListener } from '@vueuse/core'
import AuthPage from '@/pages/AuthPage.vue'
import PRPage from '@/pages/PRPage.vue'
import ReviewPage from '@/pages/ReviewPage.vue'
import SettingsPage from '@/pages/SettingsPage.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { Toaster } from '@/components/ui/sonner'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { useAuth } from '@/composables/useAuth'
import { buildAvatarProxyURL } from '@/composables/useAvatarFallback'
import type { ReviewLoadInput } from '@/types/review'
import { BrowserOpenURL } from '../wailsjs/runtime/runtime'
import SideRail from '@/components/shell/SideRail.vue'
import TitleStrip from '@/components/shell/TitleStrip.vue'

interface ReviewNavMeta {
  owner: string
  repo: string
  number: number
  title: string
  state?: string
  is_draft?: boolean
  html_url?: string
}

const { authState, refreshAuthState, logout } = useAuth()
const currentPage = ref<'pr' | 'review'>('pr')
const selectedPRItem = ref<ReviewLoadInput | null>(null)
const reviewNavMeta = ref<ReviewNavMeta | null>(null)
const settingsDialogOpen = ref(false)
const authAvatarSrc = computed(() => buildAvatarProxyURL(authState.value.avatar_url))
const railLayout = computed(() => authState.value.is_authenticated)

function goHome() {
  if (currentPage.value === 'review') handleCloseReview()
  else currentPage.value = 'pr'
}

onMounted(async () => {
  await refreshAuthState()
})

async function handleLogout() {
  await logout()
}

function handleOpenReview(item: ReviewLoadInput) {
  selectedPRItem.value = item
  reviewNavMeta.value = {
    owner: item.owner,
    repo: item.repo,
    number: item.number,
    title: item.title,
    html_url: item.html_url,
  }
  currentPage.value = 'review'
}

function handleCloseReview() {
  currentPage.value = 'pr'
  selectedPRItem.value = null
  reviewNavMeta.value = null
}

function handleUpdatePRMeta(meta: ReviewNavMeta) {
  reviewNavMeta.value = meta
}

function openPRInBrowser() {
  const url = reviewNavMeta.value?.html_url
  if (!url) return
  BrowserOpenURL(url)
}

// Links in rendered comments would otherwise navigate the app window itself away.
function openExternalLink(event: MouseEvent): void {
  const link = event.target instanceof Element ? event.target.closest<HTMLAnchorElement>('a[href]') : null
  if (!link || event.defaultPrevented) return
  const url = new URL(link.href, location.href)
  if (url.origin === location.origin || !/^(https?|mailto):$/.test(url.protocol)) return
  event.preventDefault()
  BrowserOpenURL(url.href)
}
useEventListener(document, 'click', openExternalLink)
useEventListener(document, 'auxclick', openExternalLink)
</script>
