<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import type { model } from '@/wailsjs/go/models'

const props = defineProps<{
  files: model.PRFileDTO[]
  selectedPath: string | null
  loading?: boolean
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

const fileItems = computed(() =>
  props.files.map((f) => ({
    ...f,
    status: statusInfo(f.status),
    displayName: f.filename.split('/').pop() ?? f.filename,
    dirName: f.filename.includes('/') ? f.filename.slice(0, f.filename.lastIndexOf('/')) : '',
  })),
)

function handleKeydown(event: KeyboardEvent, path: string): void {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    emit('select', path)
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
      <ul role="listbox" aria-label="Changed files list" class="p-1">
        <li
          v-for="file in fileItems"
          :key="file.filename"
          role="option"
          :aria-selected="file.filename === selectedPath"
          :tabindex="0"
          class="flex items-center gap-2 px-2 py-1.5 rounded-md cursor-pointer select-none text-xs outline-none
                 hover:bg-accent hover:text-accent-foreground
                 focus-visible:ring-2 focus-visible:ring-ring
                 data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground"
          :data-selected="file.filename === selectedPath"
          @click="emit('select', file.filename)"
          @keydown="handleKeydown($event, file.filename)"
        >
          <!-- Status badge -->
          <Badge
            :variant="file.status.variant"
            class="shrink-0 w-4 h-4 p-0 flex items-center justify-center text-[10px] font-bold"
            :aria-label="file.status.label === 'A' ? 'Added' : file.status.label === 'D' ? 'Deleted' : file.status.label === 'R' ? 'Renamed' : 'Modified'"
          >
            {{ file.status.label }}
          </Badge>

          <!-- Filename -->
          <span class="flex-1 min-w-0 truncate" :title="file.filename">
            <span v-if="file.dirName" class="text-muted-foreground">{{ file.dirName }}/</span>
            <span class="font-medium">{{ file.displayName }}</span>
          </span>

          <!-- +/- counts -->
          <span v-if="!file.is_binary" class="shrink-0 flex gap-1 text-[10px]">
            <span v-if="file.additions > 0" class="text-green-600 dark:text-green-400">+{{ file.additions }}</span>
            <span v-if="file.deletions > 0" class="text-red-500 dark:text-red-400">-{{ file.deletions }}</span>
          </span>
          <span v-else class="text-[10px] text-muted-foreground shrink-0">bin</span>
        </li>
      </ul>
    </div>
  </div>
</template>
