<script setup lang="ts">
/**
 * SplitRunButton — a split button for running commands against comment content.
 *
 * Left part: runs the default command (or the only command) immediately.
 * Right part (chevron): opens a dropdown listing all commands to pick from.
 *
 * If no defaultCommandName is set, falls back to the first command in the list.
 */
import { computed, ref } from 'vue'
import { Play, ChevronDown, Loader2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
  DropdownMenuLabel,
} from '@/components/ui/dropdown-menu'
import { RunCommands } from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'

const props = defineProps<{
  /** All configured commands */
  commands: model.CommandDTO[]
  /** Name of the user's designated default command */
  defaultCommandName: string
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
  const def = props.commands.find((c) => c.name === props.defaultCommandName)
  return def ?? props.commands[0]
})

async function runCommand(cmd: model.CommandDTO): Promise<void> {
  running.value = true
  try {
    await RunCommands([cmd.name], props.input, {
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
      <!-- RunCommands answers as soon as the run starts, so the spinner only replaces the icon when that wait is real -->
      <span class="grid *:col-start-1 *:row-start-1">
        <Play class="h-3 w-3 text-green-500 transition-opacity" :class="running && 'opacity-0 delay-200'" />
        <span v-if="running" class="animate-in fade-in-0 animation-duration-base delay-200 fill-mode-backwards">
          <Loader2 class="h-3 w-3 animate-spin text-green-500" />
        </span>
      </span>
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
          :key="cmd.name"
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
