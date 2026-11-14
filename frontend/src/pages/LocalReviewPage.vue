<script setup lang="ts">
import { errorMessage } from '@/lib/utils'
import { onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useLocalStorage } from '@vueuse/core'
import { FolderX, RefreshCw, Terminal } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import DiffReviewView from '@/components/diff/DiffReviewView.vue'
import RunPanel from '@/components/run/RunPanel.vue'
import { reducedMotion, vPointer } from '@/components/motion/wow'
import type { UnsentComments } from '@/lib/diffLayout'
import { StartLocalReview, StopLocalReview } from '@/bindings/gitura/internal/app/app'
import { Events } from '@wailsio/runtime'

const props = defineProps<{ path: string }>()

const base = useLocalStorage<'merge-base' | 'HEAD'>('gitura.localBase', 'merge-base')
const started = ref(false)
const starting = ref(false)
const error = ref('')
const refreshing = ref(false)
const runPanelOpen = ref(false)
const diffReviewRef = ref<InstanceType<typeof DiffReviewView> | null>(null)
const unsentComments: UnsentComments = reactive(new Map())

// A failed start keeps its message on screen until the retry answers.
async function start(): Promise<void> {
  if (starting.value) return
  starting.value = true
  try {
    await StartLocalReview(props.path, base.value)
    started.value = true
    error.value = ''
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    starting.value = false
  }
}

let reloadAgain = false

// A change landing mid-reload may be missing from it, so reload once more afterwards.
async function reload(): Promise<void> {
  if (error.value || !started.value) return start()
  if (refreshing.value) {
    reloadAgain = true
    return
  }
  refreshing.value = true
  try {
    do {
      reloadAgain = false
      await diffReviewRef.value?.reload()
    } while (reloadAgain)
  } catch (err) {
    toast.error('Could not refresh local changes: ' + errorMessage(err))
  } finally {
    refreshing.value = false
  }
}

/** The other scope's changes spring in from the side that scope sits on: uncommitted from the right, the branch from the left. */
function slideIn(fromRight: boolean): void {
  const el: unknown = diffReviewRef.value?.$el
  if (reducedMotion() || !(el instanceof HTMLElement)) return
  const theme = getComputedStyle(el)
  el.animate(
    [{ translate: `${fromRight ? 8 : -8}rem 0` }, { translate: '0 0' }],
    { duration: parseFloat(theme.getPropertyValue('--transition-duration-show')), easing: theme.getPropertyValue('--ease-spring') },
  )
}

watch([base, () => props.path], async ([scope], [previous]) => {
  await start()
  if (error.value) return
  await reload()
  if (scope !== previous) slideIn(scope === 'HEAD')
})

const offChanged = Events.On('local:changed', () => void reload())
onMounted(start)
onUnmounted(() => {
  offChanged()
  void StopLocalReview()
})
</script>

<template>
  <div class="flex flex-col h-full">
    <Teleport defer to="#title-strip-actions">
      <Select v-model="base">
        <SelectTrigger class="h-7 w-40 text-xs" aria-label="Changes to review" style="--wails-draggable: no-drag">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="merge-base">Branch changes</SelectItem>
          <SelectItem value="HEAD">Uncommitted</SelectItem>
        </SelectContent>
      </Select>
      <Button
        variant="ghost"
        size="icon"
        class="h-7 w-7 text-muted-foreground hover:text-foreground"
        aria-label="Refresh local changes"
        title="Refresh local changes"
        style="--wails-draggable: no-drag"
        @click="reload()"
      >
        <RefreshCw :class="['h-3.5 w-3.5', refreshing && 'animate-spin delay-200']" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        class="h-7 w-7"
        :class="runPanelOpen ? 'text-foreground' : 'text-muted-foreground hover:text-foreground'"
        :aria-expanded="runPanelOpen"
        aria-label="Run history"
        title="Run history"
        style="--wails-draggable: no-drag"
        @click="runPanelOpen = !runPanelOpen"
      >
        <Terminal class="h-3.5 w-3.5" />
      </Button>
    </Teleport>

    <div class="relative flex min-h-0 flex-1 flex-col">
      <Transition enter-active-class="animate-in fade-in-0" leave-active-class="absolute inset-0 animate-out fade-out-0 animation-duration-fast">
        <div v-if="error" v-pointer class="wow-spotlight relative flex flex-1 flex-col items-center justify-center overflow-hidden p-6">
          <div aria-hidden="true" class="wow-aurora opacity-60" />
          <div class="wow-tilt wow-focus-in relative flex max-w-md flex-col items-center gap-3 rounded-xl border border-border bg-card/80 px-10 py-8 shadow-lg backdrop-blur-sm">
            <FolderX class="size-8 text-wow-3 wow-pop [--i:4]" aria-hidden="true" />
            <p class="text-sm text-destructive text-center">{{ error }}</p>
            <Button variant="outline" size="sm" @click="start()">
              <RefreshCw :class="starting && 'animate-spin delay-200'" />
              Retry
            </Button>
          </div>
        </div>
        <div v-else-if="!started" class="flex-1 flex items-center justify-center text-muted-foreground">
          <div class="relative rounded-full p-3 wow-glow animate-in fade-in-0 delay-200 fill-mode-backwards">
            <RefreshCw class="h-6 w-6 animate-spin" />
          </div>
        </div>
        <div v-else class="flex min-h-0 flex-1 flex-col overflow-hidden">
          <DiffReviewView
            ref="diffReviewRef"
            local
            :threads="[]"
            :show-threads="false"
            :is-own-pr="false"
            head-sha=""
            :active="true"
            :unsent-comments="unsentComments"
            class="flex-1 min-h-0 overflow-hidden"
            @ran="runPanelOpen = true"
          />
          <RunPanel :open="runPanelOpen" @close="runPanelOpen = false" />
        </div>
      </Transition>
    </div>
  </div>
</template>
