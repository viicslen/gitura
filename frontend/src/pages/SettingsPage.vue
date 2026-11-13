<script setup lang="ts">
import { errorMessage } from '@/lib/utils'
import { ref, onMounted } from 'vue'
import {
  GetIgnoredCommenters,
  AddIgnoredCommenter,
  RemoveIgnoredCommenter,
  GetCommands,
  AddCommand,
  RemoveCommand,
  GetDefaultCommandName,
  SetDefaultCommandName,
} from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Trash2, UserX, Terminal, Plus, Star, Palette, Keyboard, RotateCcw, GitMerge, FileDiff, List } from '@lucide/vue'
import { TOGGLES, TOGGLE_SECTIONS, setToggle, toggleLock, useToggles } from '@/composables/useToggles'
import {
  useAppearance,
  setAppearance,
  appearanceOptions,
  loadSystemFonts,
  systemFontOptions,
  type AppearanceKey,
} from '@/composables/useAppearance'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { KEYBINDING_ACTIONS, bindKey, keyLabel, useKeybindings, type KeybindingAction } from '@/composables/useKeybindings'

const SECTIONS = [
  { value: 'appearance', label: 'Appearance', icon: Palette },
  { value: 'review', label: TOGGLE_SECTIONS.review, icon: GitMerge },
  { value: 'diff', label: TOGGLE_SECTIONS.diff, icon: FileDiff },
  { value: 'prList', label: TOGGLE_SECTIONS.prList, icon: List },
  { value: 'commenters', label: 'Ignored commenters', icon: UserX },
  { value: 'commands', label: 'Commands', icon: Terminal },
  { value: 'keybindings', label: 'Keybindings', icon: Keyboard },
] as const
const section = ref<string>('appearance')

const card = 'rounded-lg border border-border bg-card/40 divide-y divide-border'
const row = 'flex items-center gap-4 px-4 py-3'

// ── Appearance ─────────────────────────────────────────────────────────────
const { appearance } = useAppearance()
const appearanceRows: { key: AppearanceKey; label: string; hint: string }[] = [
  { key: 'text_font', label: 'Text font', hint: 'Used for prose, comments and the interface.' },
  { key: 'code_font', label: 'Code font', hint: 'Used for diffs and code blocks.' },
  { key: 'text_size', label: 'Text size', hint: 'Base size for text across the app.' },
  { key: 'theme', label: 'Theme', hint: 'Also in the account menu.' },
]

// ── Toggles ────────────────────────────────────────────────────────────────
const { toggles } = useToggles()
const toggleSections = Object.keys(TOGGLE_SECTIONS) as (keyof typeof TOGGLE_SECTIONS)[]

// ── Ignored Commenters ─────────────────────────────────────────────────────
const commenters = ref<model.IgnoredCommenterDTO[]>([])
const loadError = ref('')

const newLogin = ref('')
const addError = ref('')
const adding = ref(false)

const removingLogin = ref<string | null>(null)

onMounted(async () => {
  void loadSystemFonts()
  try {
    commenters.value = await GetIgnoredCommenters()
  } catch (e: unknown) {
    loadError.value = errorMessage(e)
  }
  try {
    commands.value = await GetCommands()
  } catch (e: unknown) {
    loadCmdError.value = errorMessage(e)
  }
  try {
    defaultCommandName.value = await GetDefaultCommandName()
  } catch {
    // non-fatal
  }
})

async function handleAdd() {
  const login = newLogin.value.trim()
  if (!login) {
    addError.value = 'Enter a GitHub username.'
    return
  }
  adding.value = true
  addError.value = ''
  try {
    await AddIgnoredCommenter(login)
    commenters.value = await GetIgnoredCommenters()
    newLogin.value = ''
  } catch (e: unknown) {
    const msg = errorMessage(e)
    addError.value = msg.replace('validation:', '').trim() || 'Failed to add commenter.'
  } finally {
    adding.value = false
  }
}

async function handleRemove(login: string) {
  removingLogin.value = login
  try {
    await RemoveIgnoredCommenter(login)
    commenters.value = await GetIgnoredCommenters()
  } catch {
    // ignore — list will be stale at worst
  } finally {
    removingLogin.value = null
  }
}

// ── Keyboard ───────────────────────────────────────────────────────────────
const { bindings, reset: resetBindings } = useKeybindings()
const recording = ref<KeybindingAction | null>(null)
const bindError = ref('')

function startRecording(action: KeybindingAction): void {
  recording.value = action
  bindError.value = ''
}

async function onRecordKeydown(event: KeyboardEvent, action: KeybindingAction): Promise<void> {
  if (recording.value !== action || ['Shift', 'Control', 'Alt', 'Meta', 'Tab'].includes(event.key)) return
  event.preventDefault()
  event.stopPropagation()
  bindError.value = event.key === 'Escape' ? '' : (await bindKey(action, event)) ?? ''
  if (!bindError.value) recording.value = null
}

async function handleResetBindings(): Promise<void> {
  bindError.value = (await resetBindings()) ?? ''
}

// ── Commands ───────────────────────────────────────────────────────────────
const commands = ref<model.CommandDTO[]>([])
const loadCmdError = ref('')
const defaultCommandName = ref('')

const newCmdName = ref('')
const newCmdCommand = ref('')
const addCmdError = ref('')
const addingCmd = ref(false)

const removingCmdName = ref<string | null>(null)
const settingDefaultName = ref<string | null>(null)

async function handleSetDefault(name: string): Promise<void> {
  settingDefaultName.value = name
  try {
    await SetDefaultCommandName(name)
    defaultCommandName.value = name
  } catch {
    // ignore
  } finally {
    settingDefaultName.value = null
  }
}

async function handleAddCommand() {
  const name = newCmdName.value.trim()
  const command = newCmdCommand.value.trim()
  if (!name) {
    addCmdError.value = 'Enter a command name.'
    return
  }
  if (!command) {
    addCmdError.value = 'Enter a command to run.'
    return
  }
  addingCmd.value = true
  addCmdError.value = ''
  try {
    commands.value = await AddCommand({ name, command })
    newCmdName.value = ''
    newCmdCommand.value = ''
  } catch (e: unknown) {
    const msg = errorMessage(e)
    addCmdError.value = msg.replace('validation:', '').trim() || 'Failed to add command.'
  } finally {
    addingCmd.value = false
  }
}

async function handleRemoveCommand(name: string) {
  removingCmdName.value = name
  try {
    commands.value = await RemoveCommand(name)
    // If the removed command was the default, clear local state
    if (defaultCommandName.value === name) {
      defaultCommandName.value = ''
    }
  } catch {
    // ignore
  } finally {
    removingCmdName.value = null
  }
}
</script>

<template>
  <Tabs v-model="section" orientation="vertical" class="h-full min-h-0 w-full flex-row gap-0">
    <TabsList class="h-full w-48 shrink-0 flex-col items-stretch justify-start gap-0.5 rounded-none border-r border-border bg-muted/30 p-2">
      <TabsTrigger
        v-for="s in SECTIONS"
        :key="s.value"
        :value="s.value"
        class="h-8 flex-none justify-start gap-2 px-2 font-normal text-muted-foreground transition-none hover:bg-accent/60 hover:text-foreground data-[state=active]:bg-accent data-[state=active]:text-foreground data-[state=active]:shadow-none dark:data-[state=active]:bg-accent dark:data-[state=active]:border-transparent"
      >
        <component :is="s.icon" aria-hidden="true" />
        {{ s.label }}
      </TabsTrigger>
    </TabsList>

    <div class="flex min-w-0 flex-1 flex-col">
      <header class="flex h-12 shrink-0 items-center gap-2 border-b border-border pl-5 pr-12 text-sm">
        <DialogTitle class="text-sm font-normal text-muted-foreground">Settings</DialogTitle>
        <span class="text-muted-foreground" aria-hidden="true">/</span>
        <span :key="section" class="font-medium animate-in fade-in-0 animation-duration-fast">{{ SECTIONS.find((s) => s.value === section)?.label }}</span>
        <Button
          v-if="section === 'keybindings'"
          variant="ghost"
          size="sm"
          class="ml-auto h-7 text-xs animate-in fade-in-0"
          @click="handleResetBindings"
        >
          <RotateCcw class="h-3.5 w-3.5" aria-hidden="true" />
          Restore defaults
        </Button>
      </header>

      <div :key="section" class="min-h-0 flex-1 overflow-y-auto overflow-x-hidden px-6 py-5 animate-in fade-in-0 animation-duration-fast">
        <TabsContent value="appearance">
          <div :class="card">
            <div v-for="r in appearanceRows" :key="r.key" :class="row">
              <div class="min-w-0 flex-1">
                <p :id="`appearance-${r.key}-label`" class="text-sm font-medium">{{ r.label }}</p>
                <p class="text-xs text-muted-foreground">{{ r.hint }}</p>
              </div>
              <Select :model-value="appearance[r.key]" @update:model-value="setAppearance(r.key, String($event))">
                <SelectTrigger :aria-labelledby="`appearance-${r.key}-label`" class="h-8 w-44">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="option in appearanceOptions[r.key]" :key="option.value" :value="option.value">
                    {{ option.label }}
                  </SelectItem>
                  <SelectGroup v-if="systemFontOptions(r.key).length">
                    <SelectLabel class="px-2 py-1.5 text-xs text-muted-foreground">Installed</SelectLabel>
                    <SelectItem v-for="option in systemFontOptions(r.key)" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
          </div>
        </TabsContent>

        <TabsContent v-for="key in toggleSections" :key="key" :value="key">
          <ul :class="card">
            <li v-for="t in TOGGLES.filter((t) => t.section === key)" :key="t.id" :class="row">
              <div class="min-w-0 flex-1">
                <p :id="`toggle-${t.id}-label`" class="text-sm font-medium">{{ t.label }}</p>
                <p class="text-xs text-muted-foreground">{{ toggleLock(t.id) || t.hint }}</p>
              </div>
              <Switch
                :model-value="toggles[t.id]"
                :disabled="!!toggleLock(t.id)"
                :aria-labelledby="`toggle-${t.id}-label`"
                @update:model-value="setToggle(t.id, $event)"
              />
            </li>
          </ul>
        </TabsContent>

        <TabsContent value="commenters" class="space-y-3">
          <p class="text-xs text-muted-foreground">
            Comments from these GitHub usernames are hidden in the review view.
          </p>
          <div :class="card">
            <div :class="row">
              <Input
                v-model="newLogin"
                placeholder="GitHub username"
                class="h-8 flex-1 text-sm"
                aria-label="GitHub username to ignore"
                :disabled="adding"
                @keydown.enter="handleAdd"
              />
              <Button
                size="sm"
                :disabled="adding || !newLogin.trim()"
                aria-label="Add ignored commenter"
                @click="handleAdd"
              >
                <Plus class="h-3.5 w-3.5" aria-hidden="true" />
                Add
              </Button>
            </div>
            <p v-if="loadError" class="px-4 py-3 text-xs text-destructive" role="alert">{{ loadError }}</p>
            <div v-else-if="commenters.length === 0" class="flex flex-col items-center gap-2 py-6 text-muted-foreground animate-in fade-in-0 animation-duration-fast">
              <UserX class="h-8 w-8 opacity-40" aria-hidden="true" />
              <p class="text-sm">No ignored commenters.</p>
            </div>
            <TransitionGroup v-else tag="ul" class="contents" aria-label="Ignored commenters list" appear enter-active-class="animate-in fade-in-0 animation-duration-fast" leave-active-class="pointer-events-none animate-out fade-out-0 animation-duration-faster">
              <li v-for="item in commenters" :key="item.login" :class="[row, 'group py-2']">
                <span class="flex-1 font-mono text-sm">{{ item.login }}</span>
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-6 w-6 text-destructive opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100 focus-visible:opacity-100"
                  :disabled="removingLogin === item.login"
                  :aria-label="`Remove ${item.login} from ignored commenters`"
                  @click="handleRemove(item.login)"
                >
                  <Trash2 class="h-3.5 w-3.5" aria-hidden="true" />
                </Button>
              </li>
            </TransitionGroup>
          </div>
          <p v-if="addError" class="text-xs text-destructive animate-in fade-in-0 animation-duration-fast" role="alert">{{ addError }}</p>
        </TabsContent>

        <TabsContent value="commands" class="space-y-3">
          <p class="text-xs text-muted-foreground">
            CLI commands to run against PR comment content. Use
            <code v-pre class="rounded bg-muted px-1 font-mono">{{instructions}}</code>
            as a placeholder for the comment text, or omit it to receive the text via stdin.
          </p>
          <div :class="card">
            <div :class="row">
              <Input
                v-model="newCmdName"
                placeholder="Name (e.g. opencode)"
                class="h-8 w-36 text-sm"
                aria-label="Command name"
                :disabled="addingCmd"
                @keydown.enter="newCmdCommand ? handleAddCommand() : undefined"
              />
              <Input
                v-model="newCmdCommand"
                placeholder="Command (e.g. opencode run --agent reviewer)"
                class="h-8 flex-1 font-mono text-sm"
                aria-label="Command to run"
                :disabled="addingCmd"
                @keydown.enter="handleAddCommand"
              />
              <Button
                size="sm"
                :disabled="addingCmd || !newCmdName.trim() || !newCmdCommand.trim()"
                aria-label="Add command"
                @click="handleAddCommand"
              >
                <Plus class="h-3.5 w-3.5" aria-hidden="true" />
                Add
              </Button>
            </div>
            <p v-if="loadCmdError" class="px-4 py-3 text-xs text-destructive" role="alert">{{ loadCmdError }}</p>
            <div v-else-if="commands.length === 0" class="flex flex-col items-center gap-2 py-6 text-muted-foreground animate-in fade-in-0 animation-duration-fast">
              <Terminal class="h-8 w-8 opacity-30" aria-hidden="true" />
              <p class="text-sm">No commands configured.</p>
            </div>
            <TransitionGroup v-else tag="ul" class="contents" aria-label="Commands list" appear enter-active-class="animate-in fade-in-0 animation-duration-fast" leave-active-class="pointer-events-none animate-out fade-out-0 animation-duration-faster">
              <li v-for="cmd in commands" :key="cmd.name" :class="[row, 'group py-2.5']">
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-1.5">
                    <p class="text-sm font-medium">{{ cmd.name }}</p>
                    <Star
                      v-if="cmd.name === defaultCommandName"
                      class="h-3 w-3 fill-current text-yellow-500 animate-in fade-in-0 zoom-in-75 ease-out animation-duration-fast"
                      aria-label="Default command"
                    />
                  </div>
                  <p class="truncate font-mono text-xs text-muted-foreground">{{ cmd.command }}</p>
                </div>
                <Button
                  v-if="cmd.name !== defaultCommandName"
                  variant="ghost"
                  size="icon"
                  class="h-6 w-6 shrink-0 text-muted-foreground opacity-0 transition-opacity hover:text-foreground group-hover:opacity-100 focus-visible:opacity-100"
                  :disabled="settingDefaultName === cmd.name"
                  :aria-label="`Set ${cmd.name} as default command`"
                  title="Set as default"
                  @click="handleSetDefault(cmd.name)"
                >
                  <Star class="h-3.5 w-3.5" aria-hidden="true" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-6 w-6 shrink-0 text-destructive opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100 focus-visible:opacity-100"
                  :disabled="removingCmdName === cmd.name"
                  :aria-label="`Remove command ${cmd.name}`"
                  @click="handleRemoveCommand(cmd.name)"
                >
                  <Trash2 class="h-3.5 w-3.5" aria-hidden="true" />
                </Button>
              </li>
            </TransitionGroup>
          </div>
          <p v-if="addCmdError" class="text-xs text-destructive animate-in fade-in-0 animation-duration-fast" role="alert">{{ addCmdError }}</p>
        </TabsContent>

        <TabsContent value="keybindings" class="space-y-3">
          <p class="text-xs text-muted-foreground">Click a shortcut, then press the new key. Escape cancels.</p>
          <ul :class="card">
            <li v-for="action in KEYBINDING_ACTIONS" :key="action.id" :class="[row, 'py-2.5']">
              <div class="min-w-0 flex-1">
                <p class="text-sm font-medium">{{ action.label }}</p>
                <p class="text-xs text-muted-foreground">{{ action.view }}</p>
              </div>
              <Button
                variant="outline"
                size="sm"
                class="h-7 min-w-24 font-mono text-xs"
                :aria-label="`Change shortcut for ${action.label}`"
                @click="startRecording(action.id)"
                @keydown="onRecordKeydown($event, action.id)"
                @blur="recording === action.id && (recording = null)"
              >
                {{ recording === action.id ? 'Press a key…' : keyLabel(bindings[action.id]) }}
              </Button>
            </li>
          </ul>
          <p v-if="bindError" class="text-xs text-destructive animate-in fade-in-0 animation-duration-fast" role="alert">{{ bindError }}</p>
        </TabsContent>
      </div>
    </div>
  </Tabs>
</template>
