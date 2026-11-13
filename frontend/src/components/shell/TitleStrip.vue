<script setup lang="ts">
// Pages teleport into #title-strip-pr-actions (next to the PR title) and #title-strip-actions (right side).
import { ArrowLeft, ExternalLink } from '@lucide/vue'
import PRStateBadge from './PRStateBadge.vue'
import WindowControls from './WindowControls.vue'
import type { PRRef } from '@/types/pr'

defineProps<{ meta: PRRef | null; fallback: string; localLabel?: string; focused?: boolean }>()
const emit = defineEmits<{ (e: 'open-pr'): void; (e: 'back'): void }>()
</script>

<template>
  <div class="h-10 flex items-stretch shrink-0 border-b border-border select-none" style="--wails-draggable: drag">
    <div class="flex-1 min-w-[8.5rem] flex items-center gap-2 pr-2" :class="meta || localLabel ? 'pl-2' : 'pl-4'">
      <button
        v-if="(meta || localLabel) && !focused"
        class="h-7 w-7 shrink-0 grid place-items-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
        style="--wails-draggable: no-drag"
        aria-label="Back to pull requests"
        title="Back to pull requests"
        @click="emit('back')"
      ><ArrowLeft class="h-4 w-4" /></button>
      <template v-if="meta">
        <span class="text-sm truncate min-w-[4rem]" :title="`${meta.owner}/${meta.repo}#${meta.number} ${meta.title}`">
          <span class="text-muted-foreground">{{ meta.repo }} #{{ meta.number }}</span>
          <span class="font-medium ml-1.5">{{ meta.title }}</span>
        </span>
        <PRStateBadge :state="meta.state" :is-draft="meta.is_draft" />
        <button
          class="h-7 w-7 shrink-0 grid place-items-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
          style="--wails-draggable: no-drag"
          aria-label="Open on GitHub"
          title="Open on GitHub"
          @click="emit('open-pr')"
        ><ExternalLink class="h-3.5 w-3.5" /></button>
      </template>
      <span v-else-if="localLabel" class="text-sm font-medium truncate min-w-[4rem]" :title="localLabel">{{ localLabel }}</span>
      <span v-else class="text-sm text-muted-foreground">{{ fallback }}</span>
      <div id="title-strip-pr-actions" class="contents" />
    </div>
    <div class="flex items-center gap-0.5 pr-2" style="--wails-draggable: no-drag">
      <div id="title-strip-actions" class="contents" />
    </div>
    <WindowControls />
  </div>
</template>

