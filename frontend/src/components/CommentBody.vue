<script setup lang="ts">
/**
 * CommentBody — renders a PR comment body with interactive code blocks.
 *
 * Text segments are passed to MarkdownBody (full markdown rendering with syntax
 * highlighting). Fenced code blocks are extracted and rendered via
 * CodeBlockWithRunner, which adds a run button overlay:
 *  - Any fenced block gets a hover-visible "Run with command" button.
 *  - Blocks tagged ```run get a persistently-visible "Run" button.
 */
import { computed } from 'vue'
import MarkdownBody from './MarkdownBody.vue'
import CodeBlockWithRunner from './CodeBlockWithRunner.vue'
import { parseCodeBlocks } from '@/lib/parseCodeBlocks'
import type { model } from '../wailsjs/go/models'
import { RunCommands } from '../wailsjs/go/main/App'

const props = defineProps<{
  content: string
  commands: model.CommandDTO[]
  defaultCommandName: string
  threadRootId?: number
  commentId?: number
}>()

const emit = defineEmits<{
  (e: 'ran'): void
}>()

const segments = computed(() => parseCodeBlocks(props.content))

/** Returns the primary command (default or first). */
const primaryCommand = computed((): model.CommandDTO | null => {
  if (props.commands.length === 0) return null
  return props.commands.find((c) => c.name === props.defaultCommandName) ?? props.commands[0]
})

/**
 * Callback passed to MarkdownBody for run buttons on rendered code blocks.
 * Runs the primary command with the given text as input.
 */
function handleRunCode(commandName: string, text: string, threadRootId: number, commentId: number): void {
  void RunCommands([commandName], text, {
    thread_root_id: threadRootId,
    comment_id: commentId,
  }).then(() => emit('ran'))
}

const runCodeCallback = computed(() =>
  props.commands.length > 0 ? handleRunCode : undefined
)
</script>

<template>
  <div class="space-y-1">
    <template v-for="(seg, i) in segments" :key="i">
      <!-- Plain text / inline markdown -->
      <MarkdownBody
        v-if="seg.type === 'text'"
        :content="seg.content"
        :run-commands="commands.length > 0 ? commands : undefined"
        :default-command-name="defaultCommandName"
        :on-run-code="runCodeCallback"
        :thread-root-id="threadRootId"
        :comment-id="commentId"
      />
      <!-- Code block with optional agent runner -->
      <CodeBlockWithRunner
        v-else
        :lang="seg.lang"
        :content="seg.content"
        :is-run="seg.isRun"
        :commands="commands"
        :default-command-name="defaultCommandName"
        :thread-root-id="threadRootId"
        :comment-id="commentId"
        @ran="emit('ran')"
      />
    </template>
  </div>
</template>
