<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ChevronDown, ChevronRight, MessageSquare } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import type { model } from '@/wailsjs/go/models'
import { buildFileTree, flattenFileTree } from '@/lib/fileTree'

const props = defineProps<{
  files: model.PRFileDTO[]
  selectedPath: string | null
  loading?: boolean
  /** Unresolved thread count per file path. */
  commentCounts?: Record<string, number>
}>()

const emit = defineEmits<{
  (e: 'select', path: string): void
}>()

type StatusVariant = 'default' | 'secondary' | 'destructive' | 'outline'

interface StatusInfo {
  label: string
  variant: StatusVariant
}

function statusInfo(status: string): StatusInfo {
  switch (status) {
    case 'added':
      return { label: 'A', variant: 'default' }
    case 'removed':
      return { label: 'D', variant: 'destructive' }
    case 'renamed':
      return { label: 'R', variant: 'secondary' }
    default:
      return { label: 'M', variant: 'outline' }
  }
}

const tree = computed(() => buildFileTree(props.files))
const collapsed = ref(new Set<string>())
const rows = computed(() => flattenFileTree(tree.value, collapsed.value))

function toggle(path: string): void {
  const next = new Set(collapsed.value)
  if (!next.delete(path)) next.add(path)
  collapsed.value = next
}

// Keep the selected file visible when it is reached with next/previous file.
watch(() => props.selectedPath, (path) => {
  if (!path) return
  const next = new Set([...collapsed.value].filter((dir) => !path.startsWith(`${dir}/`)))
  if (next.size !== collapsed.value.size) collapsed.value = next
})

function handleKeydown(event: KeyboardEvent, action: () => void): void {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    action()
  }
}
</script>

<template>
  <div class="flex flex-col h-full border-r border-border" aria-label="Changed files">
    <!-- Loading skeleton -->
    <div v-if="loading" class="flex flex-col gap-1 p-2">
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

    <!-- File list -->
    <div v-else class="flex-1 overflow-y-auto min-h-0">
      <ul role="tree" aria-label="Changed files list" class="p-1">
        <template v-for="row in rows" :key="row.type === 'dir' ? `dir:${row.path}` : row.file.filename">
          <li
            v-if="row.type === 'dir'"
            role="treeitem"
            :aria-level="row.depth + 1"
            :aria-expanded="row.expanded"
            :tabindex="0"
            :title="row.path"
            class="flex items-center gap-1 rounded-md py-1 pr-2 cursor-pointer select-none text-xs text-muted-foreground outline-none hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring"
            :style="{ paddingLeft: `${row.depth * 12 + 4}px` }"
            @click="toggle(row.path)"
            @keydown="handleKeydown($event, () => toggle(row.path))"
          >
            <component :is="row.expanded ? ChevronDown : ChevronRight" class="h-3.5 w-3.5 shrink-0" />
            <span class="min-w-0 truncate">{{ row.name }}</span>
          </li>
          <li
            v-else
            role="treeitem"
            :aria-level="row.depth + 1"
            :aria-selected="row.file.filename === selectedPath"
            :tabindex="0"
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
              class="shrink-0 w-4 h-4 p-0 flex items-center justify-center text-[10px] font-bold"
              :aria-label="row.file.status === 'added' ? 'Added' : row.file.status === 'removed' ? 'Deleted' : row.file.status === 'renamed' ? 'Renamed' : 'Modified'"
            >
              {{ statusInfo(row.file.status).label }}
            </Badge>

            <span class="flex-1 min-w-0 truncate font-medium">{{ row.name }}</span>

            <span
              v-if="commentCounts?.[row.file.filename]"
              class="shrink-0 flex items-center gap-0.5 text-[10px] text-muted-foreground"
              :aria-label="`${commentCounts[row.file.filename]} unresolved comments`"
            >
              <MessageSquare class="h-3 w-3" />{{ commentCounts[row.file.filename] }}
            </span>

            <span v-if="!row.file.is_binary" class="shrink-0 flex gap-1 text-[10px]">
              <span v-if="row.file.additions > 0" class="text-green-600 dark:text-green-400">+{{ row.file.additions }}</span>
              <span v-if="row.file.deletions > 0" class="text-red-500 dark:text-red-400">-{{ row.file.deletions }}</span>
            </span>
            <span v-else class="text-[10px] text-muted-foreground shrink-0">bin</span>
          </li>
        </template>
      </ul>
    </div>
  </div>
</template>
