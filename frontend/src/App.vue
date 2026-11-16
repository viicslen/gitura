<template>
  <!-- Toaster must live outside the overflow-hidden root div; fixed positioning
       in WebKit (Wails) is clipped by overflow:hidden ancestors. Vue 3 fragments
       allow multiple root elements. -->
  <Toaster position="bottom-right" richColors />
  <div class="h-screen flex overflow-hidden bg-background text-foreground">
    <!-- :css is off until the first auth check and while the app is revealed, so the rail is simply there then and only glides on sign-out. -->
    <Transition
      :css="authChecked && !morphing"
      enter-active-class="transition-[width] duration-slower"
      enter-from-class="w-0!"
      leave-active-class="transition-[width] duration-slow"
      leave-to-class="w-0!"
    >
      <div v-if="railLayout" class="flex w-12 shrink-0 justify-end overflow-hidden">
        <SideRail
          :meta="currentPage === 'review' ? reviewNavMeta : null"
          :login="authState.login"
          :avatar-src="authAvatarSrc"
          :focused="focused !== false"
          @home="goHome"
          @settings="settingsDialogOpen = true"
          @logout="handleLogout"
        />
      </div>
    </Transition>
    <div class="flex-1 min-w-0 flex flex-col overflow-hidden">
    <TitleStrip
      :meta="authState.is_authenticated && currentPage === 'review' ? reviewNavMeta : null"
      :fallback="authState.is_authenticated && !focused ? 'Pull Requests' : 'gitura'"
      :local-label="authState.is_authenticated && currentPage === 'local' ? localLabel : undefined"
      :focused="focused !== false"
      @open-pr="openPRInBrowser"
      @back="goHome"
    />

    <!-- Page routing: every page sits in the one grid cell, so the leaving page keeps its box while it fades out over the entering one. -->
    <main class="relative grid flex-1 grid-cols-1 grid-rows-1 overflow-hidden" data-vt-page>
      <!-- :css is off while a view transition makes the change, or the page would animate twice. -->
      <Transition
        :css="!morphing"
        :enter-active-class="PAGE_MOTION[pageMotion].enter"
        :leave-active-class="PAGE_MOTION[pageMotion].leave"
        @before-leave="(el) => pageScroll.set(el, el.scrollTop)"
        @enter="(el) => (el.scrollTop = pageScroll.get(el) ?? 0)"
      >
        <!-- Keeps PRListPage's scroll position and filter state while a review is open; emptied on sign-out. -->
        <KeepAlive :include="railLayout ? 'PRListPage' : []">
          <div v-if="page === 'boot'" :class="[cell, 'grid place-items-center text-muted-foreground']" role="status" aria-label="Loading">
            <div class="animate-in fade-in-0 delay-200 fill-mode-backwards">
              <RefreshCw class="h-6 w-6 animate-spin" />
            </div>
          </div>
          <AuthPage v-else-if="page === 'auth'" :class="cell" />
          <PRListPage
            v-else-if="page === 'prList' && focused === false"
            :class="cell"
            :pr-updates="prUpdates"
            @open-review="handleOpenReview"
            @open-local="handleOpenLocal"
          />
          <PRReviewPage
            v-else-if="page === 'review' && selectedPRItem"
            :class="cell"
            :pr-item="selectedPRItem"
            @update-pr-meta="handleUpdatePRMeta"
            @close-review="focused ? Application.Quit() : handleCloseReview()"
          />
          <LocalReviewPage v-else-if="page === 'local'" :class="cell" :path="localPath" />
        </KeepAlive>
      </Transition>
    </main>

    <Dialog :open="settingsDialogOpen" @update:open="settingsDialogOpen = $event">
      <DialogContent class="flex h-[min(34rem,85vh)] gap-0 overflow-hidden p-0 border border-border sm:max-w-3xl" :aria-describedby="undefined">
        <SettingsPage />
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="zoomOpen">
      <DialogContent
        class="w-auto max-w-[95vw] sm:max-w-[95vw] p-0 border-0 bg-transparent shadow-none"
        :style="{ transformOrigin: zoomedImage?.origin }"
        :aria-describedby="undefined"
        @click="zoomOpen = false"
      >
        <DialogTitle class="sr-only">{{ zoomedImage?.alt || 'Image' }}</DialogTitle>
        <img v-if="zoomedImage" :src="zoomedImage.src" :alt="zoomedImage.alt" class="max-h-[90vh] max-w-[95vw] object-contain cursor-zoom-out rounded-md" />
      </DialogContent>
    </Dialog>
    </div>
    <!-- The rim of the circle the app is revealed through: a quarter of its final size, scaled up with it. -->
    <div
      v-if="morphing === 'signin' && railLayout"
      class="vt-rim pointer-events-none fixed top-[calc(var(--vt-y)-var(--vt-r)/4)] left-[calc(var(--vt-x)-var(--vt-r)/4)] size-[calc(var(--vt-r)/2)] rounded-full border border-wow-2 shadow-[0_0_1.5rem_0.25rem_var(--color-wow-1),inset_0_0_1.5rem_0.25rem_var(--color-wow-3)]"
      aria-hidden="true"
    />
  </div>
</template>

<script setup lang="ts">
import { errorMessage } from '@/lib/utils'
import { computed, nextTick, ref, reactive, onMounted, watch } from 'vue'
import { useEventListener } from '@vueuse/core'
import AuthPage from '@/pages/AuthPage.vue'
import PRListPage from '@/pages/PRListPage.vue'
import PRReviewPage from '@/pages/PRReviewPage.vue'
import LocalReviewPage from '@/pages/LocalReviewPage.vue'
import SettingsPage from '@/pages/SettingsPage.vue'
import { Toaster } from '@/components/ui/sonner'
import { toast } from 'vue-sonner'
import { RefreshCw } from '@lucide/vue'
import { IsFocused, TakeLaunch } from '@/bindings/gitura/internal/app/app'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { useAuth } from '@/composables/useAuth'
import { buildAvatarProxyURL } from '@/lib/avatar'
import type { PRRef } from '@/types/pr'
import { Application, Browser, Events } from '@wailsio/runtime'
import SideRail from '@/components/shell/SideRail.vue'
import TitleStrip from '@/components/shell/TitleStrip.vue'
import { canMorph, morph, morphing } from '@/components/shell/morph'

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
// False until the saved token is verified, so a signed-in launch never shows the sign-in page.
const authChecked = ref(false)

const page = computed(() => (!authChecked.value ? 'boot' : !authState.value.is_authenticated ? 'auth' : currentPage.value))
const PAGE_DEPTH = { boot: 0, auth: 0, prList: 1, review: 2, local: 2 }
// arrive and signOut are rare; the others are page changes, moved one tier faster because they repeat all day.
// hidden! takes the old page away in one frame where fading it out over the new one dropped frames.
const PAGE_MOTION = {
  arrive: { enter: 'animate-in fade-in-0 slide-in-from-bottom-2 animation-duration-slower', leave: 'animate-out fade-out-0 animation-duration-slow' },
  signOut: { enter: 'animate-in fade-in-0 slide-in-from-bottom-2 animation-duration-slower', leave: 'hidden!' },
  deeper: { enter: 'animate-in fade-in-0 slide-in-from-bottom-2', leave: 'animate-out fade-out-0 slide-out-to-top-2 animation-duration-fast' },
  back: { enter: 'animate-in fade-in-0 slide-in-from-top-2', leave: 'hidden!' },
  sibling: { enter: 'animate-in fade-in-0', leave: 'hidden!' },
}
// A kept-alive page is detached while it is away, which resets its scroll position; put it back before the enter starts.
const pageScroll = new WeakMap<Element, number>()
const cell = 'col-start-1 row-start-1 min-h-0 min-w-0'
const pageMotion = ref<keyof typeof PAGE_MOTION>('arrive')
// sync: the classes must be chosen before the render that swaps the page.
watch(
  page,
  (now, was) => {
    const step = PAGE_DEPTH[now] - PAGE_DEPTH[was]
    pageMotion.value = !PAGE_DEPTH[was] ? 'arrive' : !PAGE_DEPTH[now] ? 'signOut' : step > 0 ? 'deeper' : step < 0 ? 'back' : 'sibling'
  },
  { flush: 'sync' },
)

// The PR list row title that is the other end of the title strip title's flight (see the `page` rules below).
let flown: Element | null = null
function fly(pr: PRRef | null) {
  flown?.removeAttribute('data-vt-title')
  flown = pr && document.querySelector(`[data-pr-key="${CSS.escape(`${pr.owner}/${pr.repo}#${pr.number}`)}"] [data-pr-title]`)
  flown?.setAttribute('data-vt-title', '')
}

let turns = 0
/** Changes the page as the view transition `page`; `pr` is the pull request whose title flies between its row and the title strip. */
async function turnPage(back: boolean, pr: PRRef | null, change: () => void) {
  if (!canMorph()) return change()
  const turn = ++turns
  document.documentElement.toggleAttribute('data-vt-back', back)
  if (!back) fly(pr)
  await morph('page', async () => {
    change()
    if (!back) return
    await nextTick()
    fly(pr)
  })
  if (turn !== turns) return
  fly(null)
  document.documentElement.removeAttribute('data-vt-back')
}

function goHome() {
  if (focused.value !== false) return
  if (currentPage.value === 'review') handleCloseReview()
  else void turnPage(true, null, () => (currentPage.value = 'prList'))
}

onMounted(async () => {
  void IsFocused().then((f) => (focused.value = f), () => (focused.value = false))
  await refreshAuthState()
  authChecked.value = true
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
  void turnPage(false, item, () => {
    selectedPRItem.value = item
    reviewNavMeta.value = { ...item }
    currentPage.value = 'review'
  })
}

function handleOpenLocal(path: string) {
  void turnPage(false, null, () => {
    localPath.value = path
    currentPage.value = 'local'
  })
}

function handleCloseReview() {
  void turnPage(true, reviewNavMeta.value, () => {
    currentPage.value = 'prList'
    selectedPRItem.value = null
    reviewNavMeta.value = null
  })
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

const zoomedImage = ref<{ src: string; alt: string; origin: string } | null>(null)
// Separate from zoomedImage so the image stays rendered through the dialog's close animation.
const zoomOpen = ref(false)
// Linked images (badges, preview buttons) keep their link behaviour.
useEventListener(document, 'click', (event: MouseEvent) => {
  const img = event.target instanceof Element ? event.target.closest<HTMLImageElement>('.markdown-body img') : null
  if (!img || img.closest('a[href]')) return
  const box = img.getBoundingClientRect()
  // The dialog is centred in the window, so this origin makes it grow from the image that was clicked.
  const origin = `calc(50% + ${box.x + box.width / 2 - innerWidth / 2}px) calc(50% + ${box.y + box.height / 2 - innerHeight / 2}px)`
  zoomedImage.value = { src: img.currentSrc || img.src, alt: img.alt, origin }
  zoomOpen.value = true
})
</script>

<!-- Unscoped: view-transition pseudo-elements hang off <html>. Every rule is scoped to the transition's name in html[data-vt]. -->
<style>
/* page: the old page scales through and the new one springs forward (the other way round going back),
   while the pull request title flies between its list row and the title strip. Nothing can be clicked
   until the last of these animations ends, so the way back to the list is the shorter one. */
html[data-vt='page'] [data-vt-page] {
  view-transition-name: vt-page;
}
html[data-vt='page'] [data-vt-title] {
  view-transition-name: vt-title;
}
html[data-vt='page']::view-transition-group(vt-page) {
  overflow: clip;
}
html[data-vt='page']::view-transition-old(vt-page) {
  mix-blend-mode: normal;
  animation: vt-through var(--transition-duration-base) var(--ease-out) both;
}
html[data-vt='page']::view-transition-new(vt-page) {
  mix-blend-mode: normal;
  animation: vt-forward var(--transition-duration-show) var(--ease-spring) both;
}
html[data-vt='page'][data-vt-back]::view-transition-old(vt-page) {
  animation-name: vt-recede;
}
html[data-vt='page'][data-vt-back]::view-transition-new(vt-page) {
  animation-name: vt-return;
  animation-duration: var(--transition-duration-slower);
}
html[data-vt='page']::view-transition-group(vt-title) {
  z-index: 1;
  animation-duration: var(--transition-duration-slower);
  animation-timing-function: var(--ease-out);
}
html[data-vt='page']::view-transition-image-pair(vt-title) {
  animation: vt-lift var(--transition-duration-slower) var(--ease-out);
}
/* Each end keeps its own size instead of being stretched to the other's box. */
html[data-vt='page']::view-transition-old(vt-title),
html[data-vt='page']::view-transition-new(vt-title) {
  height: 100%;
  object-fit: none;
  object-position: left center;
  animation-duration: var(--transition-duration-slower);
}
@keyframes vt-through {
  to {
    opacity: 0;
    scale: 1.12;
  }
}
@keyframes vt-forward {
  from {
    opacity: 0;
    scale: 0.86;
    translate: 0 2rem;
  }
}
@keyframes vt-recede {
  to {
    opacity: 0;
    scale: 0.86;
  }
}
@keyframes vt-return {
  from {
    opacity: 0;
    scale: 1.12;
  }
}
@keyframes vt-lift {
  50% {
    scale: 1.12;
    filter: drop-shadow(0 0 0.5rem var(--color-wow-1));
  }
}

/* signin: the app is revealed through a circle growing out of the sign-in card; AuthPage sets --vt-x, --vt-y and --vt-r. */
html[data-vt='signin'] .vt-rim {
  view-transition-name: vt-rim;
}
html[data-vt='signin']::view-transition-old(root) {
  animation: vt-recede var(--transition-duration-show) var(--ease-out) both;
}
html[data-vt='signin']::view-transition-new(root) {
  mix-blend-mode: normal;
  animation: vt-reveal var(--transition-duration-show) var(--ease-out) both;
}
html[data-vt='signin']::view-transition-new(vt-rim) {
  animation:
    vt-rim-grow var(--transition-duration-show) var(--ease-out) both,
    vt-rim-fade var(--transition-duration-show) ease-in both;
}
@keyframes vt-reveal {
  from {
    clip-path: circle(0 at var(--vt-x, 50%) var(--vt-y, 50%));
  }
  to {
    clip-path: circle(var(--vt-r, 150vmax) at var(--vt-x, 50%) var(--vt-y, 50%));
  }
}
@keyframes vt-rim-grow {
  from {
    scale: 0;
  }
  to {
    scale: 4;
  }
}
@keyframes vt-rim-fade {
  to {
    opacity: 0;
  }
}
</style>
