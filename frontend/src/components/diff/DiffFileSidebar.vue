<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { Check, ChevronRight, ListFilter, MessageSquare } from '@lucide/vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger } from '@/components/ui/context-menu'
import { isTestFile } from '@/lib/testFiles'
import type * as model from '@/bindings/gitura/internal/model'
import { buildFileTree, flattenFileTree } from '@/lib/fileTree'

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

type StatusVariant = 'default' | 'secondary' | 'destructive' | 'outline'

interface StatusInfo {
  label: string
  variant: StatusVariant
}

const STATUS_NAMES: Record<string, string> = { added: 'Added', removed: 'Deleted', renamed: 'Renamed', copied: 'Copied' }

function statusInfo(status: string): StatusInfo {
  switch (status) {
    case 'added':
      return { label: 'A', variant: 'default' }
    case 'removed':
      return { label: 'D', variant: 'destructive' }
    case 'renamed':
      return { label: 'R', variant: 'secondary' }
    case 'copied':
      return { label: 'C', variant: 'secondary' }
    default:
      return { label: 'M', variant: 'outline' }
  }
}

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

/** Every changed file under a folder, including ones the filters hide. */
const filesIn = (dir: string) => props.files.filter((f) => f.filename.startsWith(`${dir}/`))

function setFolderViewed(dir: string, viewed: boolean): void {
  emit('set-viewed', filesIn(dir).map((f) => f.filename), viewed)
}

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
  const i = rows.value.findIndex((r) => r.type === 'file' && r.file.filename === props.selectedPath)
  return i >= 0 ? i : 0
})

function onTreeKeydown(event: KeyboardEvent): void {
  const items = Array.from((event.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('[role="treeitem"]'))
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

function handleKeydown(event: KeyboardEvent, action: () => void): void {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    action()
  }
}
</script>

<template>
  <div class="flex flex-col h-full border-r border-border" aria-label="Changed files">
    <div v-if="!loading && files.length > 0" class="relative h-9 shrink-0 border-b border-border px-3 flex items-center gap-2 text-xs">
      <template v-if="viewedError">
        <span class="min-w-0 flex-1 truncate text-muted-foreground" :title="viewedError">Viewed status unavailable</span>
        <button type="button" class="shrink-0 text-muted-foreground underline underline-offset-2 hover:text-foreground" @click="emit('retry-viewed')">Retry</button>
      </template>
      <span v-else class="flex-1 truncate tabular-nums text-muted-foreground" :title="changeSummary">
        {{ viewedCount }} of {{ files.length }} viewed
      </span>
      <div
        v-if="!viewedError"
        class="absolute inset-x-0 -bottom-px h-0.5 bg-primary/70 origin-left transition-transform"
        :style="{ transform: `scaleX(${viewedCount / files.length})` }"
        role="progressbar"
        aria-label="Files viewed"
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
            <Transition enter-active-class="animate-in fade-in-0">
              <span v-if="hiddenCount > 0" class="tabular-nums">{{ hiddenCount }} hidden</span>
            </Transition>
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

    <!-- Loading skeleton; mt-9 holds the header row's place so the list doesn't shift when it loads -->
    <div v-if="loading" class="mt-9 flex flex-col gap-1 p-2 animate-in fade-in-0 delay-200 fill-mode-backwards">
      <div
        v-for="i in 5"
        :key="i"
        class="h-8 rounded bg-muted animate-pulse"
      />
    </div>

    <!-- Empty state -->
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

    <!-- File list -->
    <div v-else ref="listEl" class="flex-1 overflow-y-auto min-h-0">
      <ul role="tree" aria-label="Changed files list" class="p-1" @keydown="onTreeKeydown">
        <template v-for="(row, i) in rows" :key="row.type === 'dir' ? `dir:${row.path}` : row.file.filename">
          <ContextMenu v-if="row.type === 'dir'">
            <ContextMenuTrigger as-child>
              <li
                role="treeitem"
                :aria-level="row.depth + 1"
                :aria-expanded="row.expanded"
                :tabindex="i === tabStop ? 0 : -1"
                :title="row.path"
                class="flex items-center gap-1 rounded-md py-1 pr-2 cursor-pointer select-none text-xs text-muted-foreground outline-none hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:bg-accent"
                :style="{ paddingLeft: `${row.depth * 12 + 4}px` }"
                @click="toggle(row.path)"
                @keydown="handleKeydown($event, () => toggle(row.path))"
              >
                <ChevronRight class="h-3.5 w-3.5 shrink-0 transition-transform duration-fast" :class="{ 'rotate-90': row.expanded }" />
                <span class="min-w-0 truncate">{{ row.name }}</span>
              </li>
            </ContextMenuTrigger>
            <ContextMenuContent class="w-52">
              <ContextMenuItem :disabled="filesIn(row.path).every((f) => f.viewed)" @select="setFolderViewed(row.path, true)">
                Mark folder as viewed
              </ContextMenuItem>
              <ContextMenuItem :disabled="!filesIn(row.path).some((f) => f.viewed)" @select="setFolderViewed(row.path, false)">
                Mark folder as not viewed
              </ContextMenuItem>
            </ContextMenuContent>
          </ContextMenu>
          <li
            v-else
            role="treeitem"
            :aria-level="row.depth + 1"
            :aria-selected="row.file.filename === selectedPath"
            :tabindex="i === tabStop ? 0 : -1"
            :title="row.file.filename"
            class="flex items-center gap-2 py-1.5 pr-2 rounded-md cursor-pointer select-none text-xs outline-none
                   hover:bg-accent hover:text-accent-foreground
                   focus-visible:ring-2 focus-visible:ring-ring
                   data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground"
            :style="{ paddingLeft: `${row.depth * 12 + 8}px` }"
            :data-selected="row.file.filename === selectedPath"
            @click="emit('select', row.file.filename)"
            @keydown="handleKeydown($event, () => emit('select', row.file.filename))"
          >
            <Badge
              :variant="statusInfo(row.file.status).variant"
              class="shrink-0 w-4 h-4 p-0 flex items-center justify-center text-[0.625rem] font-bold"
              :aria-label="STATUS_NAMES[row.file.status] ?? 'Modified'"
            >
              {{ statusInfo(row.file.status).label }}
            </Badge>

            <span class="flex-1 min-w-0 truncate font-medium" :class="{ 'text-muted-foreground': row.file.viewed }">{{ row.name }}</span>

            <Check v-if="row.file.viewed" class="h-3.5 w-3.5 shrink-0 text-muted-foreground" role="img" aria-label="Viewed" />

            <span
              v-if="commentCounts?.[row.file.filename]"
              class="shrink-0 flex items-center gap-0.5 text-[0.625rem] text-muted-foreground"
              :aria-label="`${commentCounts[row.file.filename]} unresolved comments`"
            >
              <MessageSquare class="h-3 w-3" />{{ commentCounts[row.file.filename] }}
            </span>

            <span v-if="!row.file.is_binary" class="shrink-0 flex gap-1 text-[0.625rem]">
              <span v-if="row.file.additions > 0" class="text-green-600 dark:text-green-400">+{{ row.file.additions }}</span>
              <span v-if="row.file.deletions > 0" class="text-red-500 dark:text-red-400">-{{ row.file.deletions }}</span>
            </span>
            <span v-else class="text-[0.625rem] text-muted-foreground shrink-0">bin</span>
          </li>
        </template>
      </ul>
    </div>
  </div>
</template>
