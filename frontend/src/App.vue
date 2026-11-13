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
      :focused="focused !== false"
      @home="goHome"
      @settings="settingsDialogOpen = true"
      @logout="handleLogout"
    />
    <div class="flex-1 min-w-0 flex flex-col overflow-hidden">
    <TitleStrip
      :meta="authState.is_authenticated && currentPage === 'review' ? reviewNavMeta : null"
      :fallback="authState.is_authenticated && !focused ? 'Pull Requests' : 'gitura'"
      :local-label="authState.is_authenticated && currentPage === 'local' ? localLabel : undefined"
      :focused="focused !== false"
      @open-pr="openPRInBrowser"
      @back="goHome"
    />

    <!-- Theme toggle shown in top-right corner when not authenticated -->
    <div v-if="!authState.is_authenticated" class="fixed top-12 right-4 z-50">
      <ThemeToggle />
    </div>

    <!-- Page routing -->
    <main class="flex-1 overflow-hidden">
      <AuthPage v-if="!authState.is_authenticated" />
      <template v-else>
        <!-- KeepAlive preserves PRListPage scroll position and filter state when navigating to review -->
        <KeepAlive>
          <PRListPage
            v-if="currentPage === 'prList' && focused === false"
            :pr-updates="prUpdates"
            @open-review="handleOpenReview"
            @open-local="handleOpenLocal"
          />
        </KeepAlive>
        <PRReviewPage
          v-if="currentPage === 'review' && selectedPRItem"
          :pr-item="selectedPRItem"
          @update-pr-meta="handleUpdatePRMeta"
          @close-review="focused ? Application.Quit() : handleCloseReview()"
        />
        <LocalReviewPage v-if="currentPage === 'local'" :path="localPath" />
      </template>
    </main>

    <Dialog :open="settingsDialogOpen" @update:open="settingsDialogOpen = $event">
      <DialogContent class="flex h-[min(34rem,85vh)] gap-0 overflow-hidden p-0 border border-border sm:max-w-3xl" :aria-describedby="undefined">
        <SettingsPage />
      </DialogContent>
    </Dialog>

    <Dialog :open="!!zoomedImage" @update:open="!$event && (zoomedImage = null)">
      <DialogContent
        class="w-auto max-w-[95vw] sm:max-w-[95vw] p-0 border-0 bg-transparent shadow-none"
        :aria-describedby="undefined"
        @click="zoomedImage = null"
      >
        <DialogTitle class="sr-only">{{ zoomedImage?.alt || 'Image' }}</DialogTitle>
        <img v-if="zoomedImage" :src="zoomedImage.src" :alt="zoomedImage.alt" class="max-h-[90vh] max-w-[95vw] object-contain cursor-zoom-out rounded-md" />
      </DialogContent>
    </Dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { errorMessage } from '@/lib/utils'
import { computed, ref, reactive, onMounted, watch } from 'vue'
import { useEventListener } from '@vueuse/core'
import AuthPage from '@/pages/AuthPage.vue'
import PRListPage from '@/pages/PRListPage.vue'
import PRReviewPage from '@/pages/PRReviewPage.vue'
import LocalReviewPage from '@/pages/LocalReviewPage.vue'
import SettingsPage from '@/pages/SettingsPage.vue'
import ThemeToggle from '@/components/shell/ThemeToggle.vue'
import { Toaster } from '@/components/ui/sonner'
import { toast } from 'vue-sonner'
import { IsFocused, TakeLaunch } from '@/bindings/gitura/internal/app/app'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { useAuth } from '@/composables/useAuth'
import { buildAvatarProxyURL } from '@/lib/avatar'
import type { PRRef } from '@/types/pr'
import { Application, Browser, Events } from '@wailsio/runtime'
import SideRail from '@/components/shell/SideRail.vue'
import TitleStrip from '@/components/shell/TitleStrip.vue'

const { authState, refreshAuthState, logout } = useAuth()
const currentPage = ref<'prList' | 'review' | 'local'>('prList')
const localPath = ref('')
const localLabel = computed(() => localPath.value.split(/[\\/]/).filter(Boolean).pop() ?? localPath.value)
const selectedPRItem = ref<PRRef | null>(null)
const reviewNavMeta = ref<PRRef | null>(null)
const settingsDialogOpen = ref(false)
const prUpdates = reactive(new Map<string, PRRef>())
const authAvatarSrc = computed(() => buildAvatarProxyURL(authState.value.avatar_url))
const railLayout = computed(() => authState.value.is_authenticated)
// null until known, so a --focus window never mounts (and fetches) the PR list.
const focused = ref<boolean | null>(null)

function goHome() {
  if (focused.value !== false) return
  if (currentPage.value === 'review') handleCloseReview()
  else currentPage.value = 'prList'
}

onMounted(async () => {
  void IsFocused().then((f) => (focused.value = f), () => (focused.value = false))
  await refreshAuthState()
})

// Opens what the command line asked for (`gitura 123`, `gitura my-branch`, `gitura --local`) once signed in.
async function openLaunch() {
  if (!authState.value.is_authenticated) return
  try {
    const launch = await TakeLaunch()
    const pr = launch?.pr
    if (pr) handleOpenReview({ owner: pr.owner, repo: pr.repo, number: pr.number, title: pr.title, state: pr.state, is_draft: pr.is_draft, html_url: pr.html_url })
    else if (launch?.local_path) handleOpenLocal(launch.local_path)
  } catch (e) {
    toast.error('Could not open: ' + errorMessage(e))
  }
}
watch(() => authState.value.is_authenticated, openLaunch)
Events.On('launch', openLaunch)

async function handleLogout() {
  await logout()
}

function handleOpenReview(item: PRRef) {
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

function handleOpenLocal(path: string) {
  localPath.value = path
  currentPage.value = 'local'
}

function handleCloseReview() {
  currentPage.value = 'prList'
  selectedPRItem.value = null
  reviewNavMeta.value = null
}

function handleUpdatePRMeta(meta: PRRef) {
  reviewNavMeta.value = meta
  prUpdates.set(`${meta.owner}/${meta.repo}#${meta.number}`, meta)
}

function openPRInBrowser() {
  const url = reviewNavMeta.value?.html_url
  if (!url) return
  void Browser.OpenURL(url)
}

// Links in rendered comments would otherwise navigate the app window itself away.
function openExternalLink(event: MouseEvent): void {
  const link = event.target instanceof Element ? event.target.closest<HTMLAnchorElement>('a[href]') : null
  if (!link || event.defaultPrevented) return
  const url = new URL(link.href, location.href)
  if (url.origin === location.origin || !/^(https?|mailto):$/.test(url.protocol)) return
  event.preventDefault()
  void Browser.OpenURL(url.href)
}
useEventListener(document, 'click', openExternalLink)
useEventListener(document, 'auxclick', openExternalLink)

const zoomedImage = ref<{ src: string; alt: string } | null>(null)
// Linked images (badges, preview buttons) keep their link behaviour.
useEventListener(document, 'click', (event: MouseEvent) => {
  const img = event.target instanceof Element ? event.target.closest<HTMLImageElement>('.markdown-body img') : null
  if (!img || img.closest('a[href]')) return
  zoomedImage.value = { src: img.currentSrc || img.src, alt: img.alt }
})
</script>
