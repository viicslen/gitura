<template>
  <div class="flex items-center justify-center min-h-[60vh]">
    <Card class="w-full max-w-md">
      <CardHeader>
        <CardTitle>Sign in with GitHub</CardTitle>
        <CardDescription>
          Authenticate via GitHub OAuth Device Flow — no password required.
        </CardDescription>
      </CardHeader>

      <CardContent class="space-y-4">
        <!-- Error state -->
        <div
          v-if="error"
          class="rounded-md bg-destructive/10 border border-destructive/30 px-4 py-3 text-sm text-destructive"
          role="alert"
        >
          {{ error }}
        </div>

        <!-- Initial / idle state -->
        <template v-if="!deviceFlowInfo && !polling">
          <p class="text-sm text-muted-foreground">
            Click the button below to generate a one-time code. You will be
            redirected to GitHub to complete sign-in.
          </p>
          <Button class="w-full" :disabled="loading" @click="handleStart">
            <span v-if="loading">Starting…</span>
            <span v-else>Sign in with GitHub</span>
          </Button>
        </template>

        <!-- Device flow in progress -->
        <template v-else-if="deviceFlowInfo">
          <p class="text-sm text-muted-foreground">
            Enter the code below at
            <a
              :href="deviceFlowInfo.verification_uri"
              class="underline font-medium"
              target="_blank"
              rel="noopener noreferrer"
            >{{ deviceFlowInfo.verification_uri }}</a>
          </p>

          <div class="flex items-center justify-center">
            <span
              class="font-mono text-3xl font-bold tracking-widest select-all"
              aria-label="GitHub device authorization code"
            >
              {{ deviceFlowInfo.user_code }}
            </span>
          </div>

          <div class="flex gap-2">
            <Button
              variant="outline"
              class="flex-1"
              @click="handleOpenGitHub"
            >
              Open GitHub
            </Button>
            <Button
              variant="ghost"
              class="flex-1"
              @click="handleCancel"
            >
              Cancel
            </Button>
          </div>

          <!-- Polling indicator -->
          <div v-if="polling" class="flex items-center gap-2 text-xs text-muted-foreground" aria-live="polite">
            <span class="inline-block h-2 w-2 rounded-full bg-primary animate-pulse" />
            Waiting for authorization…
          </div>
        </template>
      </CardContent>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { BrowserOpenURL } from '../wailsjs/runtime/runtime'
import { useAuth } from '@/composables/useAuth'

const { deviceFlowInfo, polling, error, startDeviceFlow } = useAuth()
const loading = ref(false)

async function handleStart() {
  loading.value = true
  await startDeviceFlow()
  loading.value = false
}

function handleOpenGitHub() {
  if (deviceFlowInfo.value) {
    BrowserOpenURL(deviceFlowInfo.value.verification_uri)
  }
}

function handleCancel() {
  // Reset by triggering a new device flow attempt clears state in useAuth
  // For now reload the composable state by navigating the parent
  window.location.reload()
}
</script>
