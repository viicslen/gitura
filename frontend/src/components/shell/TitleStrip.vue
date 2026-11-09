<script setup lang="ts">
// ReviewPage teleports Refresh into #title-strip-actions.
import { ArrowLeft, ExternalLink } from 'lucide-vue-next'
import PRStateBadge from './PRStateBadge.vue'
import WindowControls from './WindowControls.vue'
import type { ReviewNavMeta } from './types'

defineProps<{ meta: ReviewNavMeta | null; fallback: string }>()
const emit = defineEmits<{ (e: 'open-pr'): void; (e: 'back'): void }>()
</script>

<template>
  <div class="h-10 flex items-stretch shrink-0 border-b border-border select-none" style="--wails-draggable: drag">
    <div class="flex-1 min-w-[8.5rem] flex items-center gap-2 pr-2" :class="meta ? 'pl-2' : 'pl-4'">
      <template v-if="meta">
        <button
          class="h-7 w-7 shrink-0 grid place-items-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
          style="--wails-draggable: no-drag"
          aria-label="Back to pull requests"
          title="Back to pull requests"
          @click="emit('back')"
        ><ArrowLeft class="h-4 w-4" /></button>
        <span class="text-sm truncate min-w-[4rem]" :title="`${meta.owner}/${meta.repo}#${meta.number} ${meta.title}`">
          <span class="text-muted-foreground">{{ meta.repo }} #{{ meta.number }}</span>
          <span class="font-medium ml-1.5">{{ meta.title }}</span>
        </span>
        <PRStateBadge :state="meta.state" :is-draft="meta.is_draft" />
      </template>
      <span v-else class="text-sm text-muted-foreground">{{ fallback }}</span>
    </div>
    <div class="flex items-center gap-0.5 pr-2" style="--wails-draggable: no-drag">
      <div id="title-strip-actions" class="contents" />
      <button
        v-if="meta"
        class="h-7 w-7 grid place-items-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
        aria-label="Open on GitHub"
        title="Open on GitHub"
        @click="emit('open-pr')"
      ><ExternalLink class="h-3.5 w-3.5" /></button>
    </div>
    <WindowControls />
  </div>
</template>

