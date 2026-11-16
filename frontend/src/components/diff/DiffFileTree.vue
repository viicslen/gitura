<script setup lang="ts">
import { onMounted } from 'vue'
import { ChevronRight, MessageSquare } from '@lucide/vue'
import { Badge } from '@/components/ui/badge'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger } from '@/components/ui/context-menu'
import type * as model from '@/bindings/gitura/internal/model'
import { treeKey, type FileTreeNode } from '@/lib/fileTree'
import GlideList from '@/components/motion/GlideList.vue'
import Reveal from '@/components/motion/Reveal.vue'

/** One level of the changed-files tree; folders nest another level inside themselves. */
const props = defineProps<{
  nodes: FileTreeNode<model.PRFileDTO>[]
  depth: number
  /** Every changed file, including ones the filters hide; a folder's menu acts on all of them. */
  files: model.PRFileDTO[]
  collapsed: Set<string>
  selectedPath: string | null
  /** `treeKey` of the row that holds the tree's single tab stop. */
  tabStop: string
  commentCounts?: Record<string, number>
  /** Rows springing in now (`treeKey` to stagger index): the tree opening, or a folder being expanded. */
  cascade?: Map<string, number>
  /** Files the user marked viewed a moment ago. */
  justViewed?: Set<string>
}>()

const emit = defineEmits<{
  (e: 'select', path: string): void
  (e: 'toggle', path: string): void
  (e: 'set-viewed', paths: string[], viewed: boolean): void
}>()

const STATUS: Record<string, { label: string; name: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
  added: { label: 'A', name: 'Added', variant: 'default' },
  removed: { label: 'D', name: 'Deleted', variant: 'destructive' },
  renamed: { label: 'R', name: 'Renamed', variant: 'secondary' },
  copied: { label: 'C', name: 'Copied', variant: 'secondary' },
}
const statusOf = (status: string) => STATUS[status] ?? { label: 'M', name: 'Modified', variant: 'outline' as const }

const filesIn = (dir: string) => props.files.filter((f) => f.filename.startsWith(`${dir}/`))
const setFolderViewed = (dir: string, viewed: boolean) => emit('set-viewed', filesIn(dir).map((f) => f.filename), viewed)
const countOf = (path: string) => props.commentCounts?.[path] ?? 0

function onKey(event: KeyboardEvent, action: () => void): void {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    action()
  }
}

// Rows the tree opens with don't animate; ones a filter brings back later do.
let settled = false
onMounted(() => { settled = true })

// Highlights are instant in and eased out. The ring is inset because a closing folder clips its rows.
const ROW = 'flex items-center rounded-md pr-2 transition-colors duration-faster hover:bg-accent hover:duration-0'
</script>

<template>
  <GlideList v-slot="{ item: node }" :items="nodes" :item-key="treeKey" :appear="settled">
    <li v-if="node.type === 'dir'" role="none" class="grid">
      <div class="min-h-0">
        <ContextMenu>
          <ContextMenuTrigger as-child>
            <div
              role="treeitem"
              :aria-level="depth + 1"
              :aria-expanded="!collapsed.has(node.path)"
              :tabindex="tabStop === treeKey(node) ? 0 : -1"
              :title="node.path"
              class="cursor-pointer select-none gap-1 py-1 text-xs text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring data-[state=open]:bg-accent"
              :class="[ROW, { 'wow-rise': cascade?.has(treeKey(node)) }]"
              :style="{ paddingLeft: `${depth * 12 + 4}px`, '--i': cascade?.get(treeKey(node)) }"
              @click="emit('toggle', node.path)"
              @keydown="onKey($event, () => emit('toggle', node.path))"
            >
              <ChevronRight class="h-3.5 w-3.5 shrink-0 transition-transform duration-slower ease-spring" :class="{ 'rotate-90': !collapsed.has(node.path) }" />
              <span class="min-w-0 truncate">{{ node.name }}</span>
            </div>
          </ContextMenuTrigger>
          <ContextMenuContent class="w-52">
            <ContextMenuItem :disabled="filesIn(node.path).every((f) => f.viewed)" @select="setFolderViewed(node.path, true)">
              Mark folder as viewed
            </ContextMenuItem>
            <ContextMenuItem :disabled="!filesIn(node.path).some((f) => f.viewed)" @select="setFolderViewed(node.path, false)">
              Mark folder as not viewed
            </ContextMenuItem>
          </ContextMenuContent>
        </ContextMenu>
        <Reveal axis="y" :open="!collapsed.has(node.path)">
          <ul role="group">
            <DiffFileTree
              :nodes="node.children"
              :depth="depth + 1"
              :files="files"
              :collapsed="collapsed"
              :selected-path="selectedPath"
              :tab-stop="tabStop"
              :comment-counts="commentCounts"
              :cascade="cascade"
              :just-viewed="justViewed"
              @select="emit('select', $event)"
              @toggle="emit('toggle', $event)"
              @set-viewed="(paths, viewed) => emit('set-viewed', paths, viewed)"
            />
          </ul>
        </Reveal>
      </div>
    </li>
    <li
      v-else
      role="treeitem"
      :aria-level="depth + 1"
      :aria-selected="node.file.filename === selectedPath"
      :tabindex="tabStop === treeKey(node) ? 0 : -1"
      :title="node.file.filename"
      class="group/row grid cursor-pointer select-none text-xs outline-none"
      :data-selected="node.file.filename === selectedPath"
      @click="emit('select', node.file.filename)"
      @keydown="onKey($event, () => emit('select', node.file.filename))"
    >
      <!-- The look sits here so the row has no transition of its own; line height, not padding, so it can close to nothing. -->
      <div
        class="relative min-h-0 gap-2 leading-7 hover:text-accent-foreground group-focus-visible/row:ring-2 group-focus-visible/row:ring-inset group-focus-visible/row:ring-ring group-data-[selected=true]/row:bg-accent group-data-[selected=true]/row:text-accent-foreground group-data-[selected=true]/row:duration-0"
        :class="[ROW, { 'wow-rise': cascade?.has(treeKey(node)), 'wow-sheen': justViewed?.has(node.file.filename) }]"
        :style="{ paddingLeft: `${depth * 12 + 8}px`, '--i': cascade?.get(treeKey(node)) }"
      >
        <Badge
          :variant="statusOf(node.file.status).variant"
          class="shrink-0 w-4 h-4 p-0 flex items-center justify-center text-[0.625rem] font-bold"
          :aria-label="statusOf(node.file.status).name"
        >
          {{ statusOf(node.file.status).label }}
        </Badge>

        <span class="flex-1 min-w-0 truncate font-medium transition-colors" :class="{ 'text-muted-foreground': node.file.viewed }">{{ node.name }}</span>

        <Reveal axis="x" :open="node.file.viewed" class="shrink-0" :class="{ '-ml-2': !node.file.viewed }">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-3.5 w-3.5 text-muted-foreground" role="img" aria-label="Viewed">
            <path d="M4 12l5 5L20 6" pathLength="1" :class="{ 'wow-draw': justViewed?.has(node.file.filename) }" />
          </svg>
        </Reveal>

        <Reveal
          axis="x"
          :open="countOf(node.file.filename) > 0"
          class="shrink-0 text-[0.625rem] text-muted-foreground"
          :class="{ '-ml-2': !countOf(node.file.filename) }"
          :aria-label="`${countOf(node.file.filename)} unresolved comments`"
        >
          <MessageSquare class="mr-0.5 h-3 w-3" />{{ countOf(node.file.filename) }}
        </Reveal>

        <span v-if="!node.file.is_binary" class="shrink-0 flex gap-1 text-[0.625rem]">
          <span v-if="node.file.additions > 0" class="text-green-600 dark:text-green-400">+{{ node.file.additions }}</span>
          <span v-if="node.file.deletions > 0" class="text-red-500 dark:text-red-400">-{{ node.file.deletions }}</span>
        </span>
        <span v-else class="text-[0.625rem] text-muted-foreground shrink-0">bin</span>
      </div>
    </li>
  </GlideList>
</template>
