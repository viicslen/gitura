<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useLocalStorage } from '@vueuse/core'
import { RefreshCw, Terminal } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import DiffReviewView from '@/components/diff/DiffReviewView.vue'
import RunPanel from '@/components/run/RunPanel.vue'
import type { UnsentComments } from '@/lib/diffLayout'
import { StartLocalReview, StopLocalReview } from '@/wailsjs/go/app/App'
import { EventsOn } from '@/wailsjs/runtime/runtime'

const props = defineProps<{ path: string }>()

const base = useLocalStorage<'merge-base' | 'HEAD'>('gitura.localBase', 'merge-base')
const started = ref(false)
const error = ref('')
const refreshing = ref(false)
const runPanelOpen = ref(false)
const diffReviewRef = ref<InstanceType<typeof DiffReviewView> | null>(null)
const unsentComments: UnsentComments = reactive(new Map())

async function start(): Promise<void> {
  error.value = ''
  try {
    await StartLocalReview(props.path, base.value)
    started.value = true
  } catch (err) {
    error.value = String(err)
  }
}

let reloadAgain = false

// A change landing mid-reload may be missing from it, so reload once more afterwards.
async function reload(): Promise<void> {
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
    toast.error('Could not refresh local changes: ' + String(err))
  } finally {
    refreshing.value = false
  }
}

watch([base, () => props.path], async () => {
  await start()
  if (!error.value) await reload()
})

const offChanged = EventsOn('local:changed', () => void reload())
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
        :disabled="!started || refreshing"
        aria-label="Refresh local changes"
        title="Refresh local changes"
        style="--wails-draggable: no-drag"
        @click="reload()"
      >
        <RefreshCw :class="['h-3.5 w-3.5', refreshing && 'animate-spin']" />
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

    <div v-if="error" class="flex-1 flex flex-col items-center justify-center gap-3 p-6">
      <p class="text-sm text-destructive text-center">{{ error }}</p>
      <Button variant="outline" size="sm" @click="start()">Retry</Button>
    </div>
    <div v-else-if="!started" class="flex-1 flex items-center justify-center text-muted-foreground">
      <RefreshCw class="h-6 w-6 animate-spin" />
    </div>
    <template v-else>
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
    </template>
  </div>
</template>
