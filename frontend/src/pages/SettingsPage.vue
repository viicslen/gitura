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

const box = 'rounded-lg border border-border bg-card/40'
const card = `${box} divide-y divide-border`
const row = 'flex items-center gap-4 px-4 py-3'

const fade = { enterActiveClass: 'animate-in fade-in-0', leaveActiveClass: 'animate-out fade-out-0 animation-duration-fast' }
const pop = {
  enterActiveClass: 'animate-in fade-in-0 zoom-in-75 animation-duration-fast',
  leaveActiveClass: 'animate-out fade-out-0 zoom-out-75 animation-duration-faster',
}
// List rows open and close their own height, so the card and the rows below glide with them.
const collapse = 'transition-[grid-template-rows,opacity]'
const rows = {
  enterActiveClass: `${collapse} duration-base`,
  enterFromClass: 'grid-rows-[0fr] opacity-0',
  enterToClass: 'grid-rows-[1fr]',
  leaveActiveClass: `${collapse} duration-fast pointer-events-none`,
  leaveFromClass: 'grid-rows-[1fr]',
  leaveToClass: 'grid-rows-[0fr] opacity-0',
}

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
// null until loaded, so the empty state is never painted before the list arrives.
const commenters = ref<model.IgnoredCommenterDTO[] | null>(null)
const loadError = ref('')

const newLogin = ref('')
const addError = ref('')
const adding = ref(false)

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
  if (adding.value) return
  if (!login) {
    addError.value = 'Enter a GitHub username.'
    return
  }
  adding.value = true
  try {
    await AddIgnoredCommenter(login)
    commenters.value = await GetIgnoredCommenters()
    newLogin.value = ''
    addError.value = ''
  } catch (e: unknown) {
    const msg = errorMessage(e)
    addError.value = msg.replace('validation:', '').trim() || 'Failed to add commenter.'
  } finally {
    adding.value = false
  }
}

async function handleRemove(login: string) {
  try {
    await RemoveIgnoredCommenter(login)
    commenters.value = await GetIgnoredCommenters()
  } catch {
    // ignore — list will be stale at worst
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
const commands = ref<model.CommandDTO[] | null>(null)
const loadCmdError = ref('')
const defaultCommandName = ref('')

const newCmdName = ref('')
const newCmdCommand = ref('')
const addCmdError = ref('')
const addingCmd = ref(false)

async function handleSetDefault(name: string): Promise<void> {
  try {
    await SetDefaultCommandName(name)
    defaultCommandName.value = name
  } catch {
    // ignore
  }
}

async function handleAddCommand() {
  const name = newCmdName.value.trim()
  const command = newCmdCommand.value.trim()
  if (addingCmd.value) return
  if (!name) {
    addCmdError.value = 'Enter a command name.'
    return
  }
  if (!command) {
    addCmdError.value = 'Enter a command to run.'
    return
  }
  addingCmd.value = true
  try {
    commands.value = await AddCommand({ name, command })
    newCmdName.value = ''
    newCmdCommand.value = ''
    addCmdError.value = ''
  } catch (e: unknown) {
    const msg = errorMessage(e)
    addCmdError.value = msg.replace('validation:', '').trim() || 'Failed to add command.'
  } finally {
    addingCmd.value = false
  }
}

async function handleRemoveCommand(name: string) {
  try {
    commands.value = await RemoveCommand(name)
    // If the removed command was the default, clear local state
    if (defaultCommandName.value === name) {
      defaultCommandName.value = ''
    }
  } catch {
    // ignore
  }
}
</script>

<template>
  <Tabs v-model="section" orientation="vertical" class="h-full min-h-0 w-full flex-row gap-0">
    <TabsList class="relative h-full w-48 shrink-0 flex-col items-stretch justify-start gap-0.5 rounded-none border-r border-border bg-muted/30 p-2">
      <!-- The selection slides between sections: h-8 triggers, gap-0.5 apart. -->
      <span
        class="absolute inset-x-2 top-2 h-8 rounded-md bg-accent transition-transform duration-fast ease-in-out"
        :style="{ transform: `translateY(calc(${SECTIONS.findIndex((s) => s.value === section)} * 2.125rem))` }"
        aria-hidden="true"
      />
      <TabsTrigger
        v-for="s in SECTIONS"
        :key="s.value"
        :value="s.value"
        class="relative h-8 flex-none justify-start gap-2 px-2 font-normal text-muted-foreground transition-none hover:bg-accent/60 hover:text-foreground data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none dark:data-[state=active]:bg-transparent dark:data-[state=active]:border-transparent"
      >
        <component :is="s.icon" aria-hidden="true" />
        {{ s.label }}
      </TabsTrigger>
    </TabsList>

    <div class="flex min-w-0 flex-1 flex-col">
      <header class="flex h-12 shrink-0 items-center gap-2 border-b border-border pl-5 pr-12 text-sm">
        <DialogTitle class="text-sm font-normal text-muted-foreground">Settings</DialogTitle>
        <span class="text-muted-foreground" aria-hidden="true">/</span>
        <span class="grid font-medium">
          <Transition v-bind="fade">
            <span :key="section" class="col-start-1 row-start-1 justify-self-start">{{ SECTIONS.find((s) => s.value === section)?.label }}</span>
          </Transition>
        </span>
        <Button
          variant="ghost"
          size="sm"
          class="ml-auto h-7 text-xs transition-[opacity,visibility]"
          :class="section !== 'keybindings' && 'invisible opacity-0'"
          @click="handleResetBindings"
        >
          <RotateCcw class="h-3.5 w-3.5" aria-hidden="true" />
          Restore defaults
        </Button>
      </header>

      <div class="relative min-h-0 flex-1">
      <Transition v-bind="fade">
      <div :key="section" class="absolute inset-0 overflow-y-auto overflow-x-hidden px-6 py-5">
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
                <p class="grid text-xs text-muted-foreground">
                  <Transition v-bind="fade">
                    <span :key="toggleLock(t.id) || t.hint" class="col-start-1 row-start-1">{{ toggleLock(t.id) || t.hint }}</span>
                  </Transition>
                </p>
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
          <div :class="box">
            <div :class="row">
              <Input
                v-model="newLogin"
                placeholder="GitHub username"
                class="h-8 flex-1 text-sm"
                aria-label="GitHub username to ignore"
                @keydown.enter="handleAdd"
              />
              <Button
                size="sm"
                :disabled="!newLogin.trim()"
                aria-label="Add ignored commenter"
                @click="handleAdd"
              >
                <Plus class="h-3.5 w-3.5" aria-hidden="true" />
                Add
              </Button>
            </div>
            <TransitionGroup tag="ul" v-bind="rows" aria-label="Ignored commenters list">
              <li v-if="loadError" key="error" class="grid" role="alert">
                <div class="min-h-0 overflow-hidden">
                  <p class="border-t border-border px-4 py-3 text-xs text-destructive">{{ loadError }}</p>
                </div>
              </li>
              <li v-else-if="commenters?.length === 0" key="empty" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div class="flex flex-col items-center gap-2 border-t border-border py-6 text-muted-foreground">
                    <UserX class="h-8 w-8 opacity-40" aria-hidden="true" />
                    <p class="text-sm">No ignored commenters.</p>
                  </div>
                </div>
              </li>
              <li v-for="item in loadError ? [] : commenters ?? []" :key="`item:${item.login}`" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div :class="[row, 'group border-t border-border py-2']">
                    <span class="flex-1 font-mono text-sm">{{ item.login }}</span>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-6 w-6 text-destructive opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100 focus-visible:opacity-100"
                      :aria-label="`Remove ${item.login} from ignored commenters`"
                      @click="handleRemove(item.login)"
                    >
                      <Trash2 class="h-3.5 w-3.5" aria-hidden="true" />
                    </Button>
                  </div>
                </div>
              </li>
            </TransitionGroup>
          </div>
          <Transition v-bind="fade">
            <p v-if="addError" class="text-xs text-destructive" role="alert">{{ addError }}</p>
          </Transition>
        </TabsContent>

        <TabsContent value="commands" class="space-y-3">
          <p class="text-xs text-muted-foreground">
            CLI commands to run against PR comment content. Use
            <code v-pre class="rounded bg-muted px-1 font-mono">{{instructions}}</code>
            as a placeholder for the comment text, or omit it to receive the text via stdin.
          </p>
          <div :class="box">
            <div :class="row">
              <Input
                v-model="newCmdName"
                placeholder="Name (e.g. opencode)"
                class="h-8 w-36 text-sm"
                aria-label="Command name"
                @keydown.enter="newCmdCommand ? handleAddCommand() : undefined"
              />
              <Input
                v-model="newCmdCommand"
                placeholder="Command (e.g. opencode run --agent reviewer)"
                class="h-8 flex-1 font-mono text-sm"
                aria-label="Command to run"
                @keydown.enter="handleAddCommand"
              />
              <Button
                size="sm"
                :disabled="!newCmdName.trim() || !newCmdCommand.trim()"
                aria-label="Add command"
                @click="handleAddCommand"
              >
                <Plus class="h-3.5 w-3.5" aria-hidden="true" />
                Add
              </Button>
            </div>
            <TransitionGroup tag="ul" v-bind="rows" aria-label="Commands list">
              <li v-if="loadCmdError" key="error" class="grid" role="alert">
                <div class="min-h-0 overflow-hidden">
                  <p class="border-t border-border px-4 py-3 text-xs text-destructive">{{ loadCmdError }}</p>
                </div>
              </li>
              <li v-else-if="commands?.length === 0" key="empty" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div class="flex flex-col items-center gap-2 border-t border-border py-6 text-muted-foreground">
                    <Terminal class="h-8 w-8 opacity-30" aria-hidden="true" />
                    <p class="text-sm">No commands configured.</p>
                  </div>
                </div>
              </li>
              <li v-for="cmd in loadCmdError ? [] : commands ?? []" :key="`item:${cmd.name}`" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div :class="[row, 'group border-t border-border py-2.5']">
                    <div class="min-w-0 flex-1">
                      <div class="flex items-center gap-1.5">
                        <p class="text-sm font-medium">{{ cmd.name }}</p>
                        <Transition v-bind="pop">
                          <Star
                            v-if="cmd.name === defaultCommandName"
                            class="h-3 w-3 fill-current text-yellow-500"
                            aria-label="Default command"
                          />
                        </Transition>
                      </div>
                      <p class="truncate font-mono text-xs text-muted-foreground">{{ cmd.command }}</p>
                    </div>
                    <!-- Stays mounted and hidden on the default command, so it fades out under the pointer and the row keeps its layout. -->
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-6 w-6 shrink-0 text-muted-foreground opacity-0 transition-[opacity,visibility] hover:text-foreground group-hover:opacity-100 focus-visible:opacity-100"
                      :class="cmd.name === defaultCommandName && 'invisible opacity-0!'"
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
                      :aria-label="`Remove command ${cmd.name}`"
                      @click="handleRemoveCommand(cmd.name)"
                    >
                      <Trash2 class="h-3.5 w-3.5" aria-hidden="true" />
                    </Button>
                  </div>
                </div>
              </li>
            </TransitionGroup>
          </div>
          <Transition v-bind="fade">
            <p v-if="addCmdError" class="text-xs text-destructive" role="alert">{{ addCmdError }}</p>
          </Transition>
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
                class="grid h-7 min-w-24 justify-items-center font-mono text-xs"
                :aria-label="`Change shortcut for ${action.label}`"
                @click="startRecording(action.id)"
                @keydown="onRecordKeydown($event, action.id)"
                @blur="recording === action.id && (recording = null)"
              >
                <!-- Both labels share one cell, so the button keeps one width while they crossfade. -->
                <Transition v-bind="fade">
                  <span
                    :key="bindings[action.id]"
                    class="col-start-1 row-start-1 transition-[opacity,visibility] duration-fast"
                    :class="recording === action.id && 'invisible opacity-0'"
                  >{{ keyLabel(bindings[action.id]) }}</span>
                </Transition>
                <span
                  class="col-start-1 row-start-1 transition-[opacity,visibility] duration-fast"
                  :class="recording !== action.id && 'invisible opacity-0'"
                >Press a key…</span>
              </Button>
            </li>
          </ul>
          <Transition v-bind="fade">
            <p v-if="bindError" class="text-xs text-destructive" role="alert">{{ bindError }}</p>
          </Transition>
        </TabsContent>
      </div>
      </Transition>
      </div>
    </div>
  </Tabs>
</template>
