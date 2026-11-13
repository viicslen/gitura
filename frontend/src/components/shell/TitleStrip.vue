<script setup lang="ts">
// Pages teleport into #title-strip-pr-actions (next to the PR title, only there while a PR is shown) and
// #title-strip-actions (right side). Both fade in their direct children, which overrides any animation class set on such a child.
import { computed, useTemplateRef, watch } from 'vue'
import { ArrowLeft, ExternalLink } from '@lucide/vue'
import PRStateBadge from './PRStateBadge.vue'
import WindowControls from './WindowControls.vue'
import type { PRRef } from '@/types/pr'

const props = defineProps<{ meta: PRRef | null; fallback: string; localLabel?: string; focused?: boolean }>()
const emit = defineEmits<{ (e: 'open-pr'): void; (e: 'back'): void }>()

const content = useTemplateRef('content')
const shown = computed(() => (props.meta ? `${props.meta.owner}/${props.meta.repo}#${props.meta.number}` : props.localLabel ?? props.fallback))
// The badge PRStateBadge will pick, so it only crossfades when what it shows changes.
const badge = computed(() => (props.meta?.state === 'merged' || props.meta?.state === 'closed' ? props.meta.state : props.meta?.is_draft ? 'draft' : 'open'))

// Teleported actions are removed the instant their page unmounts, so what fades out is a copy of the strip taken before the
// change, laid over the live strip while that restarts its own fade-in.
watch(
  shown,
  () => {
    const live = content.value
    if (!live) return
    const ghost = live.cloneNode(true) as HTMLElement
    ghost.querySelectorAll('[id]').forEach((el) => el.removeAttribute('id'))
    ghost.inert = true
    ghost.className = 'pointer-events-none absolute inset-0 flex items-stretch animate-out fade-out-0 animation-duration-fast fill-mode-forwards **:animate-none!'
    ghost.style.opacity = getComputedStyle(live).opacity
    ghost.addEventListener('animationend', () => ghost.remove())
    live.after(ghost)
    live.style.animation = 'none'
    void live.offsetWidth
    live.style.animation = ''
  },
  { flush: 'pre' },
)
</script>

<template>
  <div class="h-10 flex items-stretch shrink-0 border-b border-border select-none" style="--wails-draggable: drag">
    <div class="relative flex min-w-0 flex-1">
      <div ref="content" class="flex min-w-0 flex-1 items-stretch animate-in fade-in-0">
        <!-- Only moves what stays: when the state badge changes width, the actions after it glide. -->
        <TransitionGroup
          tag="div"
          :css="false"
          move-class="transition-transform ease-in-out"
          class="flex-1 min-w-[8.5rem] flex items-center gap-2 pr-2"
          :class="meta || localLabel ? 'pl-2' : 'pl-4'"
        >
          <button
            v-if="(meta || localLabel) && !focused"
            key="back"
            class="h-7 w-7 shrink-0 grid place-items-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
            style="--wails-draggable: no-drag"
            aria-label="Back to pull requests"
            title="Back to pull requests"
            @click="emit('back')"
          ><ArrowLeft class="h-4 w-4" /></button>
          <template v-if="meta">
            <span :key="shown" class="text-sm truncate min-w-[4rem]" :title="`${meta.owner}/${meta.repo}#${meta.number} ${meta.title}`">
              <span class="text-muted-foreground">{{ meta.repo }} #{{ meta.number }}</span>
              <span class="font-medium ml-1.5">{{ meta.title }}</span>
            </span>
            <span key="state" class="grid shrink-0">
              <Transition
                enter-active-class="animate-in fade-in-0 zoom-in-90"
                leave-active-class="animate-out fade-out-0 zoom-out-90 animation-duration-fast"
              >
                <PRStateBadge
                  :key="badge"
                  :state="meta.state"
                  :is-draft="meta.is_draft"
                  class="col-start-1 row-start-1 justify-self-start"
                />
              </Transition>
            </span>
            <div key="actions" class="flex shrink-0 items-center gap-2">
              <button
                class="h-7 w-7 shrink-0 grid place-items-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                style="--wails-draggable: no-drag"
                aria-label="Open on GitHub"
                title="Open on GitHub"
                @click="emit('open-pr')"
              ><ExternalLink class="h-3.5 w-3.5" /></button>
              <div id="title-strip-pr-actions" class="contents *:animate-in *:fade-in-0" />
            </div>
          </template>
          <span v-else-if="localLabel" :key="shown" class="text-sm font-medium truncate min-w-[4rem]" :title="localLabel">{{ localLabel }}</span>
          <span v-else :key="shown" class="text-sm text-muted-foreground">{{ fallback }}</span>
        </TransitionGroup>
        <div class="flex items-center gap-0.5 pr-2" style="--wails-draggable: no-drag">
          <div id="title-strip-actions" class="contents *:animate-in *:fade-in-0" />
        </div>
      </div>
    </div>
    <WindowControls />
  </div>
</template>
