<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { ChevronRight, FolderGit2, ListFilter, ShieldOff } from '@lucide/vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { isTestFile } from '@/lib/testFiles'
import type * as model from '@/bindings/gitura/internal/model'
import { fileKey, flattenFileTree, keyOf, repoOf, treeKey } from '@/lib/fileTree'
import { DIVERGED_NOTICE, groupedTree, groupKey, groupSubmodules, noticeText, submoduleChange, submoduleRouting } from '@/lib/submodules'
import DiffFileTree from './DiffFileTree.vue'
import Reveal from '@/components/motion/Reveal.vue'
import CountUp from '@/components/motion/CountUp.vue'
import { loaderShown, SWAP, TEXT_SWAP } from '@/components/motion/motion'

const props = defineProps<{
  files: model.PRFileDTO[]
  /** The files left after the hide filters; defaults to all files. */
  visibleFiles?: model.PRFileDTO[]
  /** Why GitHub's viewed state couldn't be loaded, if it couldn't. */
  viewedError?: string
  /** `fileKey` of the open file. */
  selectedKey: string | null
  loading?: boolean
  /** Unresolved thread count per `fileKey`. */
  commentCounts?: Record<string, number>
  /** The Files view is on screen; the open file is scrolled into view when it appears. */
  active?: boolean
  /** `fileKey`s of the files the user marked viewed a moment ago. */
  justViewed?: Set<string>
  /** Reviewing a local working tree, where comments on submodule files need no routing. */
  local?: boolean
}>()

const hideViewed = defineModel<boolean>('hideViewed', { default: false })
const hideTests = defineModel<boolean>('hideTests', { default: false })
const autoViewTests = defineModel<boolean>('autoViewTests', { default: false })

const emit = defineEmits<{
  (e: 'select', key: string): void
  (e: 'retry-viewed'): void
  (e: 'set-viewed', keys: string[], viewed: boolean): void
}>()

const shownFiles = computed(() => props.visibleFiles ?? props.files)
const grouped = computed(() => groupSubmodules(props.files, shownFiles.value))
const mainFiles = computed(() => props.files.filter((f) => !f.submodule))
const groups = computed(() => grouped.value.groups.map((g) => {
  const key = groupKey(g.pointer.filename)
  const routing = g.files.length ? submoduleRouting(g.pointer, !!props.local) : null
  const linesUnknown = g.files.some((f) => f.lines_unknown)
  const change = submoduleChange(g.pointer)
  return { ...g, key, rowKey: treeKey({ type: 'dir', key }), change, routing, linesUnknown, additions: sum(g.files, 'additions'), deletions: sum(g.files, 'deletions') }
}))
// Other reasons (added, removed, failed to load) are not access problems.
const ACCESS_PROBLEMS = new Set(['no access', 'not on GitHub'])
const tree = computed(() => groupedTree(grouped.value))
const sum = (files: model.PRFileDTO[], key: 'additions' | 'deletions') => files.reduce((n, f) => (f.lines_unknown ? n : n + f[key]), 0)
const capitalize = (text: string) => text.charAt(0).toUpperCase() + text.slice(1)
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

// Many rows changing at once (a folder, a load) cut instead of glide: a transition per row re-lays out the tree every frame.
const bulkViewed = ref(false)
let bulkViewedTimer: ReturnType<typeof setTimeout> | undefined
watch(viewedCount, (count, before) => {
  if (Math.abs(count - before) <= 15) return
  bulkViewed.value = true
  clearTimeout(bulkViewedTimer)
  bulkViewedTimer = setTimeout(() => (bulkViewed.value = false), 500)
})
const collapsed = ref(new Set<string>())
const rows = computed(() => flattenFileTree(tree.value, collapsed.value))

// Rows springing in, staggered: the tree's when it comes on screen, a folder's when it is expanded.
// About a screenful: wow-rise stops adding delay after 15, so the rest of them land together.
const CASCADE_ROWS = 40
const cascade = ref(new Map<string, number>())
let cascadeTimer: ReturnType<typeof setTimeout> | undefined
function rise(keys: string[]): void {
  cascade.value = new Map(keys.slice(0, CASCADE_ROWS).map((key, i) => [key, i]))
  clearTimeout(cascadeTimer)
  cascadeTimer = setTimeout(() => (cascade.value = new Map()), 1500)
}
watch(() => !!props.active && !props.loading && rows.value.length > 0, (shown) => {
  if (shown) rise(rows.value.map(treeKey))
}, { immediate: true })

function toggle(key: string): void {
  const next = new Set(collapsed.value)
  const opening = next.delete(key)
  if (!opening) next.add(key)
  collapsed.value = next
  const at = rows.value.findIndex((r) => r.type === 'dir' && r.key === key)
  if (!opening || at < 0) return
  const end = rows.value.findIndex((r, i) => i > at && r.depth <= rows.value[at].depth)
  rise(rows.value.slice(at + 1, end < 0 ? undefined : end).map(treeKey))
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
watch(() => props.selectedKey, (key) => {
  const file = props.files.find((f) => fileKey(f) === key)
  if (!file) return
  const sub = file.submodule
  const repo = repoOf(file)
  // The file's path keyed as its tree keys folders.
  const path = repo ? keyOf(groupKey(repo), file.filename) : file.filename
  const holds = (dir: string) => (sub !== undefined && dir === groupKey(sub)) || path === dir || path.startsWith(`${dir}/`)
  const next = new Set([...collapsed.value].filter((dir) => !holds(dir)))
  if (next.size !== collapsed.value.size) collapsed.value = next
  revealSelected()
}, { immediate: true })
watch(() => props.active, (active) => {
  if (active) revealSelected()
})

// One tab stop for the whole tree (the open file, else the first row); arrows move within it.
const tabStop = computed(() => {
  const row = rows.value.find((r) => r.type === 'file' && fileKey(r.file) === props.selectedKey) ?? rows.value[0]
  return row ? treeKey(row) : ''
})

// A link in a submodule header opens on its own instead of folding the group.
function onHeaderClick(event: MouseEvent, key: string): void {
  if (!(event.target instanceof Element && event.target.closest('a'))) toggle(key)
}

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
                <CountUp :value="viewedCount" /> of {{ files.length }} viewed
              </span>
            </Transition>
          </div>
          <div
            class="absolute inset-x-0 -bottom-px h-0.5 bg-primary/70 origin-left transition-[transform,opacity] duration-show ease-spring"
            :class="{ 'opacity-0': viewedError }"
            :style="{ transform: `scaleX(${viewedCount / files.length})` }"
            role="progressbar"
            aria-label="Files viewed"
            :aria-hidden="!!viewedError"
            :aria-valuenow="viewedCount"
            aria-valuemin="0"
            :aria-valuemax="files.length"
          >
            <!-- Re-keyed so every change of the count lights the bar up once. -->
            <span
              :key="viewedCount"
              aria-hidden="true"
              class="absolute inset-x-0 -top-0.5 bottom-0 [[data-effects=off]_&]:hidden bg-linear-to-r from-wow-1 via-wow-2 to-wow-3 shadow-[0_0_12px_2px_var(--color-wow-2)] animate-out fade-out-0 fill-mode-forwards animation-duration-show"
            />
          </div>
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
          <ul role="tree" aria-label="Changed files list" class="p-1" :class="bulkViewed && '**:transition-none'" @keydown="onTreeKeydown">
            <DiffFileTree
              :nodes="grouped.main"
              :depth="0"
              :files="mainFiles"
              :collapsed="collapsed"
              :selected-key="selectedKey"
              :tab-stop="tabStop"
              :comment-counts="commentCounts"
              :cascade="cascade"
              :just-viewed="justViewed"
              @select="emit('select', $event)"
              @toggle="toggle"
              @set-viewed="(paths, viewed) => emit('set-viewed', paths, viewed)"
            />
            <li v-if="grouped.groups.length" role="none" class="mx-2 mt-3 border-t border-border pb-1 pt-2 text-[0.625rem] font-semibold uppercase tracking-wide text-muted-foreground" :class="{ 'mt-0 border-t-0': !grouped.main.length }">
              Submodules
            </li>
            <li v-for="g in groups" :key="g.pointer.filename" role="none" class="mb-1 grid grid-cols-1">
              <div
                role="treeitem"
                aria-level="1"
                :aria-expanded="!collapsed.has(g.key)"
                :tabindex="tabStop === g.rowKey ? 0 : -1"
                :title="g.pointer.filename"
                class="flex cursor-pointer select-none flex-col gap-0.5 rounded-md px-1 py-1.5 text-xs outline-none transition-colors duration-faster hover:bg-accent hover:duration-0 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                :class="{ 'wow-rise': cascade.has(g.rowKey) }"
                :style="{ '--i': cascade.get(g.rowKey) }"
                @click="onHeaderClick($event, g.key)"
                @keydown.enter.space.self.prevent="toggle(g.key)"
              >
                <span class="flex items-center gap-1">
                  <ChevronRight class="h-3.5 w-3.5 shrink-0 transition-transform duration-slower ease-spring" :class="{ 'rotate-90': !collapsed.has(g.key) }" />
                  <FolderGit2 class="h-3.5 w-3.5 shrink-0 text-primary" aria-label="Submodule" />
                  <span class="min-w-0 flex-1 truncate font-medium">{{ g.pointer.filename }}</span>
                  <template v-if="g.files.length || !g.pointer.submodule_unavailable">
                    <span class="shrink-0 text-[0.625rem] text-muted-foreground tabular-nums">{{ g.files.length }} {{ g.files.length === 1 ? 'file' : 'files' }}</span>
                    <span v-if="g.additions || g.deletions" class="flex shrink-0 gap-1 text-[0.625rem] tabular-nums" :title="g.linesUnknown ? 'Counted from the files opened so far' : undefined">
                      <span v-if="g.additions" class="text-green-600 dark:text-green-400">+{{ g.additions }}</span>
                      <span v-if="g.deletions" class="text-red-500 dark:text-red-400">-{{ g.deletions }}</span>
                      <span v-if="g.linesUnknown" class="text-muted-foreground">so far</span>
                    </span>
                  </template>
                </span>
                <span v-if="g.pointer.submodule_range || g.pointer.submodule_backwards || g.pointer.submodule_diverged" class="flex min-w-0 flex-wrap items-center gap-1.5 pl-4.5 text-[0.625rem] text-muted-foreground">
                  <span v-if="g.pointer.submodule_range" class="min-w-0 truncate rounded bg-muted px-1 font-mono">{{ g.pointer.submodule_range }}</span>
                  <Badge v-if="g.change" variant="outline" class="shrink-0 px-1 py-0 text-[0.625rem]">{{ g.change }}</Badge>
                  <Badge v-if="g.pointer.submodule_backwards" variant="outline" class="shrink-0 border-amber-500/60 px-1 py-0 text-[0.625rem] text-amber-700 dark:text-amber-300">Moved backwards</Badge>
                  <Badge v-if="g.pointer.submodule_diverged" variant="outline" class="shrink-0 border-amber-500/60 px-1 py-0 text-[0.625rem] text-amber-700 dark:text-amber-300" :title="DIVERGED_NOTICE">Diverged</Badge>
                </span>
                <span v-if="g.pointer.submodule_unavailable" class="flex min-w-0 items-center gap-1 pl-4.5 text-[0.625rem] text-muted-foreground" :title="capitalize(g.pointer.submodule_unavailable)">
                  <ShieldOff v-if="ACCESS_PROBLEMS.has(g.pointer.submodule_unavailable)" class="h-3 w-3 shrink-0" /><span class="truncate">{{ capitalize(g.pointer.submodule_unavailable) }}</span>
                </span>
                <span v-if="g.routing" class="line-clamp-2 min-w-0 pl-4.5 text-[0.625rem] text-muted-foreground" :title="noticeText(g.routing)">
                  {{ g.routing.text }}
                  <a v-if="g.routing.pr" :href="g.routing.url" class="text-foreground underline underline-offset-2">{{ g.routing.pr }}</a>{{ g.routing.after }}
                </span>
                <span v-if="g.pointer.submodule_viewed_unknown" class="min-w-0 truncate pl-4.5 text-[0.625rem] text-amber-700 dark:text-amber-300" :title="`Couldn't read the Viewed marks on ${g.pointer.submodule_pr}; its files may show as not viewed. Refreshing tries again.`">
                  Couldn't read Viewed marks from {{ g.pointer.submodule_pr }}
                </span>
              </div>
              <Reveal axis="y" :open="!collapsed.has(g.key)">
                <ul role="group">
                  <DiffFileTree
                    :nodes="g.nodes"
                    :depth="1"
                    :files="g.files"
                    :collapsed="collapsed"
                    :selected-key="selectedKey"
                    :tab-stop="tabStop"
                    :comment-counts="commentCounts"
                    :cascade="cascade"
                    :just-viewed="justViewed"
                    @select="emit('select', $event)"
                    @toggle="toggle"
                    @set-viewed="(paths, viewed) => emit('set-viewed', paths, viewed)"
                  />
                </ul>
              </Reveal>
            </li>
          </ul>
        </div>
      </Transition>
    </div>
  </div>
</template>
