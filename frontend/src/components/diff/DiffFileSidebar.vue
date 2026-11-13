<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { ListFilter } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { isTestFile } from '@/lib/testFiles'
import type * as model from '@/bindings/gitura/internal/model'
import { buildFileTree, flattenFileTree, treeKey } from '@/lib/fileTree'
import DiffFileTree from './DiffFileTree.vue'
import Reveal from '@/components/motion/Reveal.vue'
import { loaderShown, SWAP, TEXT_SWAP } from '@/components/motion/motion'

const props = defineProps<{
  files: model.PRFileDTO[]
  /** The files left after the hide filters; defaults to all files. */
  visibleFiles?: model.PRFileDTO[]
  /** Why GitHub's viewed state couldn't be loaded, if it couldn't. */
  viewedError?: string
  selectedPath: string | null
  loading?: boolean
  /** Unresolved thread count per file path. */
  commentCounts?: Record<string, number>
  /** The Files view is on screen; the open file is scrolled into view when it appears. */
  active?: boolean
}>()

const hideViewed = defineModel<boolean>('hideViewed', { default: false })
const hideTests = defineModel<boolean>('hideTests', { default: false })
const autoViewTests = defineModel<boolean>('autoViewTests', { default: false })

const emit = defineEmits<{
  (e: 'select', path: string): void
  (e: 'retry-viewed'): void
  (e: 'set-viewed', paths: string[], viewed: boolean): void
}>()

const shownFiles = computed(() => props.visibleFiles ?? props.files)
const tree = computed(() => buildFileTree(shownFiles.value))
const testCount = computed(() => props.files.filter((f) => isTestFile(f.filename)).length)
const hiddenCount = computed(() => props.files.length - shownFiles.value.length)
const filtering = computed(() => hideViewed.value || hideTests.value)

function showAll(): void {
  hideViewed.value = false
  hideTests.value = false
}
const changeSummary = computed(() => {
  const added = props.files.filter((f) => f.status === 'added').length
  const removed = props.files.filter((f) => f.status === 'removed').length
  const changed = props.files.length - added - removed
  return [changed && `${changed} modified`, added && `${added} added`, removed && `${removed} deleted`].filter(Boolean).join(' · ')
})
const viewedCount = computed(() => props.files.filter((f) => f.viewed).length)
const collapsed = ref(new Set<string>())
const rows = computed(() => flattenFileTree(tree.value, collapsed.value))

function toggle(path: string): void {
  const next = new Set(collapsed.value)
  if (!next.delete(path)) next.add(path)
  collapsed.value = next
}

const listEl = ref<HTMLElement | null>(null)

/** Centres the open file in the list when it is out of view. */
function revealSelected(): void {
  void nextTick(() => {
    const list = listEl.value
    const item = list?.querySelector<HTMLElement>('[aria-selected="true"]')
    if (!list || !item) return
    const box = list.getBoundingClientRect()
    const row = item.getBoundingClientRect()
    if (row.top < box.top || row.bottom > box.bottom) list.scrollTop += row.top - box.top - (box.height - row.height) / 2
  })
}

// Keep the selected file visible when it is reached with next/previous file.
watch(() => props.selectedPath, (path) => {
  if (!path) return
  const next = new Set([...collapsed.value].filter((dir) => !path.startsWith(`${dir}/`)))
  if (next.size !== collapsed.value.size) collapsed.value = next
  revealSelected()
}, { immediate: true })
watch(() => props.active, (active) => {
  if (active) revealSelected()
})

// One tab stop for the whole tree (the open file, else the first row); arrows move within it.
const tabStop = computed(() => {
  const row = rows.value.find((r) => r.type === 'file' && r.file.filename === props.selectedPath) ?? rows.value[0]
  return row ? treeKey(row) : ''
})

function onTreeKeydown(event: KeyboardEvent): void {
  // A collapsed folder keeps its rows mounted but inert.
  const items = Array.from((event.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('[role="treeitem"]')).filter((el) => !el.closest('[inert]'))
  const i = items.indexOf(document.activeElement as HTMLElement)
  if (i < 0) return
  switch (event.key) {
    case 'ArrowDown': items[i + 1]?.focus(); break
    case 'ArrowUp': items[i - 1]?.focus(); break
    case 'Home': items[0]?.focus(); break
    case 'End': items[items.length - 1]?.focus(); break
    default: return
  }
  event.preventDefault()
}

</script>

<template>
  <div class="flex flex-col h-full border-r border-border" aria-label="Changed files">
    <div class="relative h-9 shrink-0 border-b border-border px-3 flex items-center gap-2 text-xs">
      <Transition enter-active-class="animate-in fade-in-0">
        <div v-if="!loading && files.length > 0" class="flex min-w-0 flex-1 items-center gap-2">
          <!-- Both states share one cell, so they cross-fade without moving the filter button. -->
          <div class="grid min-w-0 flex-1 items-center">
            <Transition v-bind="TEXT_SWAP">
              <span v-if="viewedError" class="flex min-w-0 items-center gap-2 [grid-area:1/1]">
                <span class="min-w-0 flex-1 truncate text-muted-foreground" :title="viewedError">Viewed status unavailable</span>
                <button type="button" class="shrink-0 text-muted-foreground underline underline-offset-2 transition-colors hover:text-foreground" @click="emit('retry-viewed')">Retry</button>
              </span>
              <span v-else class="truncate tabular-nums text-muted-foreground [grid-area:1/1]" :title="changeSummary">
                {{ viewedCount }} of {{ files.length }} viewed
              </span>
            </Transition>
          </div>
          <div
            class="absolute inset-x-0 -bottom-px h-0.5 bg-primary/70 origin-left transition-[transform,opacity]"
            :class="{ 'opacity-0': viewedError }"
            :style="{ transform: `scaleX(${viewedCount / files.length})` }"
            role="progressbar"
            aria-label="Files viewed"
            :aria-hidden="!!viewedError"
            :aria-valuenow="viewedCount"
            aria-valuemin="0"
            :aria-valuemax="files.length"
          />
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button
                variant="ghost"
                size="sm"
                class="h-6 -mr-1.5 gap-1 px-1.5 text-xs"
                :class="filtering ? 'text-primary' : 'text-muted-foreground'"
                :aria-label="filtering ? `Filter files, ${hiddenCount} hidden` : 'Filter files'"
                title="Filter files"
              >
                <ListFilter class="h-3.5 w-3.5" />
                <Reveal axis="x" :open="hiddenCount > 0" class="tabular-nums" :class="{ '-ml-1': !hiddenCount }">{{ hiddenCount }} hidden</Reveal>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-60">
              <DropdownMenuCheckboxItem v-model="hideViewed" @select.prevent>
                <span class="flex-1">Hide viewed files</span>
                <span class="text-xs text-muted-foreground tabular-nums">{{ viewedCount }}</span>
              </DropdownMenuCheckboxItem>
              <DropdownMenuCheckboxItem v-model="hideTests" @select.prevent>
                <span class="flex-1">Hide test files</span>
                <span class="text-xs text-muted-foreground tabular-nums">{{ testCount }}</span>
              </DropdownMenuCheckboxItem>
              <DropdownMenuCheckboxItem v-model="autoViewTests" :disabled="!hideTests" @select.prevent>
                Mark hidden tests as viewed
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </Transition>
    </div>

    <div class="relative flex min-h-0 flex-1 flex-col">
      <!-- Stays mounted so it can fade out; the bars only pulse while it is the loader. -->
      <div class="pointer-events-none absolute inset-0 flex flex-col gap-1 p-2 transition-[opacity,visibility]" :class="loaderShown(!!loading)" aria-hidden="true">
        <div
          v-for="i in 5"
          :key="i"
          class="h-8 rounded bg-muted"
          :class="{ 'animate-pulse': loading }"
        />
      </div>

      <Transition v-bind="SWAP">
        <div v-if="loading" />
        <div
          v-else-if="files.length === 0"
          class="flex-1 flex items-center justify-center text-xs text-muted-foreground p-4 text-center"
        >
          No changed files
        </div>

        <div
          v-else-if="shownFiles.length === 0"
          class="flex-1 flex flex-col items-center justify-center gap-2 p-4 text-center text-xs text-muted-foreground"
        >
          All {{ files.length }} files are hidden by filters.
          <Button variant="outline" size="sm" class="h-7 text-xs" @click="showAll">Show all files</Button>
        </div>

        <div v-else ref="listEl" class="flex-1 overflow-y-auto min-h-0">
          <ul role="tree" aria-label="Changed files list" class="p-1" @keydown="onTreeKeydown">
            <DiffFileTree
              :nodes="tree"
              :depth="0"
              :files="files"
              :collapsed="collapsed"
              :selected-path="selectedPath"
              :tab-stop="tabStop"
              :comment-counts="commentCounts"
              @select="emit('select', $event)"
              @toggle="toggle"
              @set-viewed="(paths, viewed) => emit('set-viewed', paths, viewed)"
            />
          </ul>
        </div>
      </Transition>
    </div>
  </div>
</template>
