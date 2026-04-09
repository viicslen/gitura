<template>
  <!-- Toaster must live outside the overflow-hidden root div; fixed positioning
       in WebKit (Wails) is clipped by overflow:hidden ancestors. Vue 3 fragments
       allow multiple root elements. -->
  <Toaster position="bottom-right" richColors />
  <div class="h-screen flex flex-col overflow-hidden bg-background text-foreground">
    <!-- Nav bar shown when authenticated -->
    <nav v-if="authState.is_authenticated" class="h-14 border-b border-border px-6 flex items-center gap-4 shrink-0">
      <span class="font-semibold text-sm tracking-tight">gitura</span>
      <button
        class="text-sm text-muted-foreground hover:text-foreground transition-colors"
        :class="{ 'text-foreground font-medium': currentPage === 'pr' }"
        @click="currentPage === 'review' ? handleCloseReview() : (currentPage = 'pr')"
      >
        Pull Requests
      </button>
      <template v-if="currentPage === 'review' && reviewNavMeta">
        <ChevronRight class="h-4 w-4 text-muted-foreground/80 shrink-0" />
        <div class="min-w-0 leading-tight">
          <div class="flex items-center gap-2 min-w-0">
            <span class="text-sm font-semibold truncate max-w-[42vw]">
              {{ reviewNavMeta.title }}
            </span>
          </div>
          <div class="flex items-center gap-2 text-xs text-muted-foreground mt-0.5">
            <span class="truncate max-w-[42vw]">{{ reviewNavMeta.owner }}/{{ reviewNavMeta.repo }}</span>
            <span class="shrink-0">#{{ reviewNavMeta.number }}</span>
            <Badge v-if="reviewNavMeta.is_draft" variant="secondary" class="text-xs shrink-0">
              Draft
            </Badge>
            <Badge
              v-else-if="reviewNavMeta.state === 'merged'"
              class="text-xs shrink-0 bg-violet-500/15 text-violet-700 dark:text-violet-300 border-violet-500/30"
            >
              Merged
            </Badge>
            <Badge v-else-if="reviewNavMeta.state === 'closed'" variant="destructive" class="text-xs shrink-0">
              Closed
            </Badge>
            <Badge v-else variant="secondary" class="text-xs shrink-0">
              Open
            </Badge>
          </div>
        </div>
      </template>
      <div class="ml-auto flex items-center gap-3">
        <ThemeToggle />
        <DropdownMenu>
          <DropdownMenuTrigger class="flex items-center gap-2 rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <img
              v-if="authState.avatar_url"
              :src="authState.avatar_url"
              :alt="authState.login"
              class="h-6 w-6 rounded-full"
            />
            <span class="text-sm text-muted-foreground">{{ authState.login }}</span>
            <ChevronDown class="h-3 w-3 text-muted-foreground" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem @click="settingsDialogOpen = true">
              Settings
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem @click="handleLogout">
              Sign out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </nav>

    <!-- Theme toggle shown in top-right corner when not authenticated -->
    <div v-else-if="!authState.is_authenticated" class="fixed top-3 right-4 z-50">
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
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ChevronDown, ChevronRight } from 'lucide-vue-next'
import AuthPage from '@/pages/AuthPage.vue'
import PRPage from '@/pages/PRPage.vue'
import ReviewPage from '@/pages/ReviewPage.vue'
import SettingsPage from '@/pages/SettingsPage.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { Toaster } from '@/components/ui/sonner'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { useAuth } from '@/composables/useAuth'
import type { ReviewLoadInput } from '@/types/review'

interface ReviewNavMeta {
  owner: string
  repo: string
  number: number
  title: string
  state?: string
  is_draft?: boolean
}

const { authState, refreshAuthState, logout } = useAuth()
const currentPage = ref<'pr' | 'review'>('pr')
const selectedPRItem = ref<ReviewLoadInput | null>(null)
const reviewNavMeta = ref<ReviewNavMeta | null>(null)
const settingsDialogOpen = ref(false)

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
</script>
