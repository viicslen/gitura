<script setup lang="ts">
/**
 * CommandComposer — adds or edits a command: a name, a multi-line template, and
 * menus that insert a placeholder at the cursor or fill in a ready-made template.
 */
import { onMounted, ref, useTemplateRef, watch } from 'vue'
import { Braces, Check, ChevronDown, Plus } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { GetCommandVariables } from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'

const props = defineProps<{
  /** The command being edited; null composes a new one. */
  editing: model.CommandDTO | null
  save: (cmd: model.CommandDTO) => Promise<void>
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
}>()

const CONTEXT = `You are on {{repo}} PR #{{pr_number}} by {{author}}, {{head_branch}} into {{base_branch}}: {{pr_title}}

{{comment_author}} left this review comment on {{file}}:{{line}}:

{{instructions}}`

// "." is the T3 project: the run's working directory, which is the PR's local clone.
const TEMPLATES = [
  {
    name: 'Fix with T3 Code',
    hint: 'Starts a T3 Code thread that checks the comment, fixes it if valid, and pushes.',
    command: `t3 thread start --title "Fix comment on PR #{{pr_number}}" . "${CONTEXT}

Check whether the comment is still valid against the current code. If it is, make the smallest fix that addresses it, run the checks that cover the change, then commit and push to {{head_branch}}. If it is not valid, change nothing and explain why. Do not reply to or resolve the comment on GitHub."`,
  },
  {
    name: 'Evaluate with T3 Code',
    hint: 'Waits for a T3 Code verdict on the comment and shows it here. Changes nothing.',
    command: `t3 thread start --wait --title "Evaluate comment on PR #{{pr_number}}" . "${CONTEXT}

Decide whether the comment is valid against the current code. Reply with VALID or INVALID on the first line, then the reasoning and the fix you would make. Do not change any files."`,
  },
]

const BUILT_INS: model.CommandVariableDTO[] = [
  { name: 'instructions', description: 'Comment text; piped to stdin when the command leaves it out' },
  { name: 'repo_path', description: 'Local checkout of the PR' },
]
const variables = ref<model.CommandVariableDTO[]>(BUILT_INS)
onMounted(async () => {
  try {
    variables.value = [...BUILT_INS, ...(await GetCommandVariables())]
  } catch {
    // the built-ins still work
  }
})

const name = ref('')
const command = ref('')
const saving = ref(false)
watch(
  () => props.editing,
  (cmd) => {
    name.value = cmd?.name ?? ''
    command.value = cmd?.command ?? ''
  },
  { immediate: true },
)

const textarea = useTemplateRef<{ $el: HTMLTextAreaElement }>('textarea')
const nameInput = useTemplateRef<{ $el: HTMLInputElement }>('nameInput')
onMounted(() => nameInput.value?.$el.focus())

// Where the caret goes after an insert. The menu holds focus until it closes, so it is placed then.
let caret: number | null = null

function insert(variable: string): void {
  const el = textarea.value?.$el
  const token = `{{${variable}}}`
  const start = el?.selectionStart ?? command.value.length
  const end = el?.selectionEnd ?? command.value.length
  command.value = command.value.slice(0, start) + token + command.value.slice(end)
  caret = start + token.length
}

function restoreCaret(event: Event): void {
  if (caret === null) return
  event.preventDefault()
  const el = textarea.value?.$el
  el?.focus()
  el?.setSelectionRange(caret, caret)
  caret = null
}

function applyTemplate(t: (typeof TEMPLATES)[number]): void {
  if (!name.value.trim()) name.value = t.name
  command.value = t.command
}

async function submit(): Promise<void> {
  if (saving.value || !name.value.trim() || !command.value.trim()) return
  saving.value = true
  try {
    await props.save({ name: name.value, command: command.value })
  } finally {
    saving.value = false
  }
}

function onKeydown(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    void submit()
  }
}
</script>

<template>
  <div class="space-y-2 px-4 py-3" @keydown="onKeydown">
    <div class="flex items-center gap-2">
      <Input
        ref="nameInput"
        v-model="name"
        placeholder="Name (e.g. Fix with T3 Code)"
        class="h-8 flex-1 text-sm"
        aria-label="Command name"
      />
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button variant="outline" size="sm" class="h-8 text-xs">
            Templates
            <ChevronDown class="h-3 w-3" aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-72">
          <DropdownMenuItem
            v-for="t in TEMPLATES"
            :key="t.name"
            class="flex flex-col items-start gap-0.5"
            @select="applyTemplate(t)"
          >
            <span class="text-sm font-medium">{{ t.name }}</span>
            <span class="text-xs text-muted-foreground">{{ t.hint }}</span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
    <Textarea
      ref="textarea"
      v-model="command"
      placeholder="Command (e.g. opencode run --agent reviewer &quot;{{instructions}}&quot;)"
      class="max-h-48 min-h-20 font-mono text-xs md:text-xs"
      aria-label="Command to run"
      spellcheck="false"
    />
    <div class="flex items-center gap-2">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button variant="outline" size="sm" class="h-8 text-xs">
            <Braces class="h-3.5 w-3.5" aria-hidden="true" />
            Insert placeholder
            <ChevronDown class="h-3 w-3" aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" class="max-h-72 w-72" @close-auto-focus="restoreCaret">
          <DropdownMenuLabel class="text-xs font-normal text-muted-foreground">Each fills in where it stands and never adds arguments.</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            v-for="v in variables"
            :key="v.name"
            class="flex flex-col items-start gap-0.5"
            @select="insert(v.name)"
          >
            <span class="font-mono text-xs">{{ v.name }}</span>
            <span class="text-xs text-muted-foreground">{{ v.description }}</span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <Button variant="ghost" size="sm" class="ml-auto" @click="emit('cancel')">Cancel</Button>
      <Button size="sm" :disabled="!name.trim() || !command.trim()" @click="submit">
        <component :is="editing ? Check : Plus" class="h-3.5 w-3.5" aria-hidden="true" />
        {{ editing ? 'Save' : 'Add' }}
      </Button>
    </div>
  </div>
</template>
