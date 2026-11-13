<template>
  <div class="h-full">
    <div class="absolute top-2 right-4">
      <ThemeToggle />
    </div>
    <div class="flex items-center justify-center min-h-[60vh]">
      <Card class="w-full max-w-md">
        <CardHeader>
          <CardTitle>Sign in with GitHub</CardTitle>
          <CardDescription>
            Authenticate via GitHub OAuth Device Flow — no password required.
          </CardDescription>
        </CardHeader>

        <!-- Each step collapses while the next one opens, so the card glides to its new height. -->
        <CardContent>
          <Transition v-bind="step">
            <div v-if="error" class="grid" role="alert">
              <div class="min-h-0 overflow-hidden">
                <div class="mb-4 rounded-md bg-destructive/10 border border-destructive/30 px-4 py-3 text-sm text-destructive">
                  {{ error }}
                </div>
              </div>
            </div>
          </Transition>

          <Transition v-bind="step">
            <div v-if="!deviceFlowInfo" class="grid">
              <div class="min-h-0 space-y-4 overflow-hidden">
                <p class="text-sm text-muted-foreground">
                  Click the button below to generate a one-time code. You will be
                  redirected to GitHub to complete sign-in.
                </p>
                <!-- The label only gives way to "Starting…" once the wait is real. -->
                <Button class="grid w-full" @click="handleStart">
                  <span class="col-start-1 row-start-1" :class="loading && 'animate-out fade-out-0 delay-200 fill-mode-forwards'">Sign in with GitHub</span>
                  <span v-if="loading" class="col-start-1 row-start-1 animate-in fade-in-0 delay-200 fill-mode-backwards">Starting…</span>
                </Button>
              </div>
            </div>
          </Transition>

          <Transition v-bind="step">
            <div v-if="deviceFlowInfo" class="grid">
              <div class="min-h-0 space-y-4 overflow-hidden">
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
                    @click="cancelDeviceFlow"
                  >
                    Cancel
                  </Button>
                </div>

                <!-- Keeps its line when polling stops, so the card does not jump. -->
                <div
                  class="flex items-center gap-2 text-xs text-muted-foreground transition-[opacity,visibility] duration-fast"
                  :class="!polling && 'invisible opacity-0'"
                  aria-live="polite"
                >
                  <span class="inline-block h-2 w-2 rounded-full bg-primary animate-pulse" />
                  Waiting for authorization…
                </div>
              </div>
            </div>
          </Transition>
        </CardContent>
      </Card>
    </div>
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
import ThemeToggle from '@/components/shell/ThemeToggle.vue'
import { Browser } from '@wailsio/runtime'
import { useAuth } from '@/composables/useAuth'

const { deviceFlowInfo, polling, error, startDeviceFlow, cancelDeviceFlow } = useAuth()
const loading = ref(false)

const size = 'transition-[grid-template-rows,opacity] duration-slow'
const step = {
  enterActiveClass: size,
  enterFromClass: 'grid-rows-[0fr] opacity-0',
  enterToClass: 'grid-rows-[1fr]',
  leaveActiveClass: size,
  leaveFromClass: 'grid-rows-[1fr]',
  leaveToClass: 'grid-rows-[0fr] opacity-0',
}

async function handleStart() {
  if (loading.value) return
  loading.value = true
  await startDeviceFlow()
  loading.value = false
}

function handleOpenGitHub() {
  if (deviceFlowInfo.value) {
    void Browser.OpenURL(deviceFlowInfo.value.verification_uri)
  }
}
</script>
