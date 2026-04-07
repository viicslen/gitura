<script setup lang="ts">
/**
 * SplitRunButton — a split button for running commands against comment content.
 *
 * Left part: runs the default command (or the only command) immediately.
 * Right part (chevron): opens a dropdown listing all commands to pick from.
 *
 * If no defaultCommandId is set, falls back to the first command in the list.
 */
import { computed, ref } from 'vue'
import { Play, ChevronDown, Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
  DropdownMenuLabel,
} from '@/components/ui/dropdown-menu'
import { RunCommands } from '../wailsjs/go/main/App'
import type { model } from '../wailsjs/go/models'

const props = defineProps<{
  /** All configured commands */
  commands: model.CommandDTO[]
  /** ID of the user's designated default command */
  defaultCommandId: string
  /** The text to pass as input to the command */
  input: string
  /** Label for the primary button */
  label?: string
  /** Button size variant */
  size?: 'sm' | 'default'
  /** Thread root ID to associate this run with (0 = unlinked) */
  threadRootId?: number
  /** Comment ID to associate this run with (0 = unlinked) */
  commentId?: number
}>()

const emit = defineEmits<{
  (e: 'ran'): void
}>()

const running = ref(false)

/** The command that fires when clicking the primary button. */
const primaryCommand = computed((): model.CommandDTO | null => {
  if (props.commands.length === 0) return null
  const def = props.commands.find((c) => c.id === props.defaultCommandId)
  return def ?? props.commands[0]
})

async function runCommand(cmd: model.CommandDTO): Promise<void> {
  running.value = true
  try {
    await RunCommands([cmd.id], props.input, {
      thread_root_id: props.threadRootId ?? 0,
      comment_id: props.commentId ?? 0,
    })
    emit('ran')
  } finally {
    running.value = false
  }
}

function handlePrimary(): void {
  if (!primaryCommand.value || running.value) return
  void runCommand(primaryCommand.value)
}

const btnSize = computed(() => props.size ?? 'sm')
</script>

<template>
  <div v-if="commands.length > 0" class="flex items-center">
    <!-- Primary action button -->
    <Button
      :size="btnSize"
      :disabled="running"
      variant="ghost"
      class="rounded-r-none border-r-0 gap-1.5 bg-muted/60 hover:bg-muted"
      @click="handlePrimary"
    >
      <Loader2 v-if="running" class="h-3 w-3 animate-spin text-green-500" />
      <Play v-else class="h-3 w-3 text-green-500" />
      <span v-if="label" class="text-xs">{{ label }}</span>
    </Button>

    <!-- Chevron dropdown trigger -->
    <DropdownMenu>
      <DropdownMenuTrigger as-child>
        <Button
          :size="btnSize"
          :disabled="running"
          variant="ghost"
          class="rounded-l-none px-1 bg-muted/60 hover:bg-muted"
          :aria-label="`Run with a different command`"
        >
          <ChevronDown class="h-3 w-3" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" class="min-w-[180px]">
        <DropdownMenuLabel class="text-xs font-medium text-muted-foreground">
          Run with…
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          v-for="cmd in commands"
          :key="cmd.id"
          :disabled="running"
          class="flex flex-col items-start gap-0.5 cursor-pointer"
          @click="runCommand(cmd)"
        >
          <span class="text-sm font-medium">{{ cmd.name }}</span>
          <span class="text-xs text-muted-foreground font-mono truncate max-w-[200px]">{{ cmd.command }}</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  </div>
</template>
