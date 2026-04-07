<template>
  <!-- Toaster must live outside the overflow-hidden root div; fixed positioning
       in WebKit (Wails) is clipped by overflow:hidden ancestors. Vue 3 fragments
       allow multiple root elements. -->
  <Toaster position="bottom-right" richColors />
  <div class="h-screen flex flex-col overflow-hidden bg-background text-foreground">
    <!-- Nav bar shown when authenticated -->
    <nav v-if="authState.is_authenticated" class="border-b border-border px-6 py-3 flex items-center gap-6">
      <span class="font-semibold text-sm tracking-tight">gitura</span>
      <button
        class="text-sm text-muted-foreground hover:text-foreground transition-colors"
        :class="{ 'text-foreground font-medium': currentPage === 'pr' }"
        @click="currentPage = 'pr'"
      >
        Pull Requests
      </button>
      <button
        class="text-sm text-muted-foreground hover:text-foreground transition-colors"
        :class="{ 'text-foreground font-medium': currentPage === 'settings' }"
        @click="currentPage = 'settings'"
      >
        Settings
      </button>
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
        <SettingsPage v-if="currentPage === 'settings'" />
        <ReviewPage
          v-if="currentPage === 'review' && selectedPRItem"
          :pr-item="selectedPRItem"
          @close-review="handleCloseReview"
        />
      </template>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import AuthPage from '@/pages/AuthPage.vue'
import PRPage from '@/pages/PRPage.vue'
import SettingsPage from '@/pages/SettingsPage.vue'
import ReviewPage from '@/pages/ReviewPage.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { Toaster } from '@/components/ui/sonner'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useAuth } from '@/composables/useAuth'
import type { ReviewLoadInput } from '@/types/review'

const { authState, refreshAuthState, logout } = useAuth()
const currentPage = ref<'pr' | 'settings' | 'review'>('pr')
const selectedPRItem = ref<ReviewLoadInput | null>(null)

onMounted(async () => {
  await refreshAuthState()
})

async function handleLogout() {
  await logout()
}

function handleOpenReview(item: ReviewLoadInput) {
  selectedPRItem.value = item
  currentPage.value = 'review'
}

function handleCloseReview() {
  currentPage.value = 'pr'
  selectedPRItem.value = null
}
</script>
