<script setup lang="ts">
import { cn, errorMessage } from '@/lib/utils'
import { computed, reactive, ref, onMounted, watch } from 'vue'
import { toast } from 'vue-sonner'
import {
  GetIgnoredCommenters,
  AddIgnoredCommenter,
  RemoveIgnoredCommenter,
  GetCommands,
  AddCommand,
  UpdateCommand,
  RemoveCommand,
  SetDefaultCommand,
  GetProjectDirs,
  SetProjectDirs,
  OpenFolderPicker,
} from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from '@/components/ui/select'
import Reveal from '@/components/motion/Reveal.vue'
import { Switch } from '@/components/ui/switch'
import CommandComposer from '@/components/run/CommandComposer.vue'
import { Trash2, UserX, Terminal, Plus, Pencil, ChevronDown, Palette, Keyboard, RotateCcw, GitMerge, FileDiff, List, FolderGit2 } from '@lucide/vue'
import { TOGGLES, TOGGLE_SECTIONS, setToggle, toggleLock, useToggles, type ToggleId } from '@/composables/useToggles'
import {
  useAppearance,
  setAppearance,
  appearanceOptions,
  loadSystemFonts,
  systemFontOptions,
  type AppearanceKey,
} from '@/composables/useAppearance'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { RUN_KINDS, commands, defaultCommands, loadDefaultCommands, type RunKind } from '@/composables/useCommands'
import { KEYBINDING_ACTIONS, bindKey, keyLabel, useKeybindings, type KeybindingAction } from '@/composables/useKeybindings'
import { reducedMotion } from '@/components/motion/wow'
import { createReusableTemplate } from '@vueuse/core'

const SECTIONS = [
  { value: 'appearance', label: 'Appearance', icon: Palette },
  { value: 'review', label: TOGGLE_SECTIONS.review, icon: GitMerge },
  { value: 'diff', label: TOGGLE_SECTIONS.diff, icon: FileDiff },
  { value: 'prList', label: TOGGLE_SECTIONS.prList, icon: List },
  { value: 'commenters', label: 'Ignored commenters', icon: UserX },
  { value: 'commands', label: 'Commands', icon: Terminal },
  { value: 'projects', label: 'Project folders', icon: FolderGit2 },
  { value: 'keybindings', label: 'Keybindings', icon: Keyboard },
] as const
const section = ref<string>('appearance')
const sectionIndex = (value: string): number => SECTIONS.findIndex((s) => s.value === value)
// A section further down the nav comes up from below and one further up comes down from above.
const down = ref(true)
watch(section, (to, from) => {
  down.value = sectionIndex(to) > sectionIndex(from)
})
const turn = computed(() => ({
  enterActiveClass: `animate-in fade-in-0 zoom-in-90 ease-spring animation-duration-slower ${down.value ? 'slide-in-from-bottom-10' : 'slide-in-from-top-10'}`,
  leaveActiveClass: `pointer-events-none animate-out fade-out-0 zoom-out-90 animation-duration-fast ${down.value ? 'slide-out-to-top-6' : 'slide-out-to-bottom-6'}`,
}))

const box = 'rounded-lg border border-border bg-card/40'
const card = `${box} divide-y divide-border`
const row = 'flex items-center gap-4 px-4 py-3'

const fade = { enterActiveClass: 'animate-in fade-in-0', leaveActiveClass: 'animate-out fade-out-0 animation-duration-fast' }
const alarm = { enterActiveClass: 'wow-shake', leaveActiveClass: fade.leaveActiveClass }
// List rows open and close their own height, so the card and the rows below glide with them.
const collapse = 'transition-[grid-template-rows,opacity]'
// Under the add row; capped so a row added to a long list springs in as its slot finishes opening, not later.
const rowDelay = (index: number): number => Math.min(index + 1, 4)
const rows = {
  enterActiveClass: `${collapse} duration-base`,
  enterFromClass: 'grid-rows-[0fr] opacity-0',
  enterToClass: 'grid-rows-[1fr]',
  leaveActiveClass: `${collapse} duration-fast pointer-events-none`,
  leaveFromClass: 'grid-rows-[1fr]',
  leaveToClass: 'grid-rows-[0fr] opacity-0',
}

// Ring bursts for a setting that was just saved; each count is the key of the ring it mounts.
const bursts = reactive<Record<string, number>>({})
function burst(key: string): void {
  if (!reducedMotion()) bursts[key] = (bursts[key] ?? 0) + 1
}
const ring = 'wow-ring pointer-events-none absolute inset-0 rounded-[inherit]'

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
type ToggleSection = keyof typeof TOGGLE_SECTIONS
// Commands has its own tab content; its toggles sit under the command list.
const toggleSections = (Object.keys(TOGGLE_SECTIONS) as ToggleSection[]).filter((s) => s !== 'commands')
const [DefineToggles, ReuseToggles] = createReusableTemplate<{ section: ToggleSection }>()

async function saveToggle(id: ToggleId, value: boolean): Promise<void> {
  await setToggle(id, value)
  // A failed save has put the old value back.
  if (toggles.value[id] === value) burst(id)
}

// ── Ignored Commenters ─────────────────────────────────────────────────────
// null until loaded, so the empty state is never painted before the list arrives.
const commenters = ref<model.IgnoredCommenterDTO[] | null>(null)
const loadError = ref('')

const composingCommenter = ref(false)
const newLogin = ref('')
const addError = ref('')
const adding = ref(false)
const vFocus = { mounted: (el: HTMLElement) => el.focus() }

function closeCommenterInput(): void {
  composingCommenter.value = false
  newLogin.value = ''
  addError.value = ''
}

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
  await loadDefaultCommands()
  perKindOpen.value = overrideCount.value > 0
  try {
    projectDirs.value = await GetProjectDirs()
  } catch (e: unknown) {
    projectError.value = errorMessage(e)
  }
})

// ── Project folders ────────────────────────────────────────────────────────
const projectDirs = ref<string[] | null>(null)
const projectError = ref('')

async function saveProjectDirs(dirs: string[]) {
  try {
    await SetProjectDirs(dirs)
    projectDirs.value = dirs
    projectError.value = ''
  } catch (e: unknown) {
    projectError.value = errorMessage(e)
  }
}

async function handleAddProjectDir() {
  try {
    const dir = await OpenFolderPicker('Select a folder with your projects', '')
    const dirs = projectDirs.value ?? []
    if (dir && !dirs.includes(dir)) await saveProjectDirs([...dirs, dir])
  } catch (e: unknown) {
    projectError.value = errorMessage(e)
  }
}

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
    closeCommenterInput()
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
  if (bindError.value) return
  recording.value = null
  if (event.key !== 'Escape') burst(action)
}

async function handleResetBindings(): Promise<void> {
  bindError.value = (await resetBindings()) ?? ''
}

// ── Commands ───────────────────────────────────────────────────────────────
const loadCmdError = ref('')

const composingCmd = ref(false)
const editingCmd = ref<model.CommandDTO | null>(null)
const addCmdError = ref('')
// Bumped whenever the composer opens or closes, so a slow save only closes the session that started it.
let composerSession = 0

function openComposer(cmd: model.CommandDTO | null): void {
  composerSession++
  editingCmd.value = cmd
  composingCmd.value = true
  addCmdError.value = ''
}

function closeComposer(): void {
  composerSession++
  composingCmd.value = false
  editingCmd.value = null
  addCmdError.value = ''
}

// Command names are trimmed, so no command can be called this.
const INHERIT = ' '
const allDefaultName = computed(() => defaultCommands.value.all || commands.value?.[0]?.name || '')
const overrideCount = computed(() => RUN_KINDS.filter((k) => defaultCommands.value[k.id]).length)
const perKindOpen = ref(false)
const [DefineDefaultRow, ReuseDefaultRow] = createReusableTemplate<{ kind: 'all' | RunKind; label: string; hint: string }>()

async function handleSetDefault(kind: 'all' | RunKind, name: string): Promise<void> {
  try {
    await SetDefaultCommand(kind, name)
    const next = { ...defaultCommands.value, [kind]: name }
    if (!name) delete next[kind]
    defaultCommands.value = next
    burst(`default:${kind}`)
  } catch (e: unknown) {
    toast.error('Could not save the default command: ' + errorMessage(e))
  }
}

async function saveCommand(cmd: model.CommandDTO): Promise<void> {
  const editing = editingCmd.value
  const session = composerSession
  try {
    commands.value = editing ? await UpdateCommand(editing.name, cmd) : await AddCommand(cmd)
    if (editing) void loadDefaultCommands()
    if (session === composerSession) closeComposer()
  } catch (e: unknown) {
    if (session !== composerSession) return
    const msg = errorMessage(e)
    addCmdError.value = msg.replace(/^(validation|not_found):/, '').trim() || 'Failed to save command.'
  }
}

async function handleRemoveCommand(name: string) {
  try {
    commands.value = await RemoveCommand(name)
    if (editingCmd.value?.name === name) closeComposer()
    void loadDefaultCommands()
  } catch {
    // ignore
  }
}

// Each section's create/reset action sits in the header; hidden ones stay mounted so they crossfade in one cell.
const headerActions = computed(() => [
  { section: 'commenters', label: 'Add commenter', icon: Plus, run: () => (composingCommenter.value = true), hidden: composingCommenter.value },
  { section: 'commands', label: 'New command', icon: Plus, run: () => openComposer(null), hidden: composingCmd.value && !editingCmd.value },
  { section: 'projects', label: 'Add folder', icon: Plus, run: handleAddProjectDir, hidden: false },
  { section: 'keybindings', label: 'Restore defaults', icon: RotateCcw, run: handleResetBindings, hidden: false },
])
</script>

<template>
  <Tabs v-model="section" orientation="vertical" class="h-full min-h-0 w-full flex-row gap-0">
    <TabsList class="isolate h-full w-48 shrink-0 flex-col items-stretch justify-start gap-0.5 overflow-hidden rounded-none border-r border-border bg-muted/30 p-2 *:data-[slot=tabs-indicator]:bg-accent *:data-[slot=tabs-indicator]:shadow-none">
      <TabsTrigger
        v-for="(s, i) in SECTIONS"
        :key="s.value"
        :value="s.value"
        class="wow-rise h-8 flex-none justify-start gap-2 px-2 font-normal text-muted-foreground transition-none hover:bg-accent/60 hover:text-foreground data-[state=active]:text-foreground"
        :style="{ '--i': i }"
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
        <span class="ml-auto grid justify-items-end *:col-start-1 *:row-start-1">
          <Button
            v-for="a in headerActions"
            :key="a.section"
            variant="ghost"
            size="sm"
            class="h-7 text-xs transition-[opacity,visibility]"
            :class="(section !== a.section || a.hidden) && 'invisible opacity-0'"
            @click="a.run"
          >
            <component :is="a.icon" class="h-3.5 w-3.5" aria-hidden="true" />
            {{ a.label }}
          </Button>
        </span>
      </header>

      <div class="relative min-h-0 flex-1 overflow-hidden">
      <Transition v-bind="turn">
      <div :key="section" class="absolute inset-0 overflow-y-auto overflow-x-hidden px-6 py-5">
        <TabsContent value="appearance">
          <div :class="card">
            <div v-for="(r, i) in appearanceRows" :key="r.key" :class="[row, 'wow-rise']" :style="{ '--i': i }">
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
            <div :class="[row, 'wow-rise']" :style="{ '--i': appearanceRows.length }">
              <div class="min-w-0 flex-1">
                <p id="appearance-effects-label" class="text-sm font-medium">Effects</p>
                <p class="text-xs text-muted-foreground">Springs, glows and confetti. Off keeps only quiet fades.</p>
              </div>
              <Switch
                :model-value="appearance.effects !== 'off'"
                aria-labelledby="appearance-effects-label"
                @update:model-value="setAppearance('effects', $event ? 'on' : 'off')"
              />
            </div>
          </div>
        </TabsContent>

        <DefineToggles v-slot="{ section: key }">
          <ul :class="card">
            <li v-for="(t, i) in TOGGLES.filter((t) => t.section === key)" :key="t.id" :class="[row, 'wow-rise']" :style="{ '--i': i }">
              <div class="min-w-0 flex-1">
                <p :id="`toggle-${t.id}-label`" class="text-sm font-medium">{{ t.label }}</p>
                <p class="grid text-xs text-muted-foreground">
                  <Transition v-bind="fade">
                    <span :key="toggleLock(t.id) || t.hint" class="col-start-1 row-start-1">{{ toggleLock(t.id) || t.hint }}</span>
                  </Transition>
                </p>
              </div>
              <span class="relative inline-flex shrink-0 rounded-full">
                <Switch
                  :model-value="toggles[t.id]"
                  :disabled="!!toggleLock(t.id)"
                  :aria-labelledby="`toggle-${t.id}-label`"
                  @update:model-value="saveToggle(t.id, $event)"
                />
                <span v-if="bursts[t.id]" :key="bursts[t.id]" :class="ring" aria-hidden="true" />
              </span>
            </li>
          </ul>
        </DefineToggles>

        <TabsContent v-for="key in toggleSections" :key="key" :value="key">
          <ReuseToggles :section="key" />
        </TabsContent>

        <TabsContent value="commenters" class="space-y-3">
          <p class="text-xs text-muted-foreground">
            Comments from these GitHub usernames are hidden in the review view.
          </p>
          <div :class="box">
            <Transition v-bind="rows">
              <div v-if="composingCommenter" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div :class="[row, 'border-b border-border']">
                    <Input
                      v-focus
                      v-model="newLogin"
                      placeholder="GitHub username"
                      class="h-8 flex-1 text-sm"
                      aria-label="GitHub username to ignore"
                      @keydown.enter="handleAdd"
                    />
                    <Button variant="ghost" size="sm" :disabled="adding" @click="closeCommenterInput">Cancel</Button>
                    <Button size="sm" :disabled="!newLogin.trim()" aria-label="Add ignored commenter" @click="handleAdd">
                      Add
                    </Button>
                  </div>
                </div>
              </div>
            </Transition>
            <TransitionGroup tag="ul" v-bind="rows" class="divide-y divide-border" aria-label="Ignored commenters list">
              <li v-if="loadError" key="error" class="grid" role="alert">
                <div class="min-h-0 overflow-hidden">
                  <p class="px-4 py-3 text-xs text-destructive">{{ loadError }}</p>
                </div>
              </li>
              <li v-else-if="commenters?.length === 0" key="empty" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div class="flex flex-col items-center gap-2 py-6 text-muted-foreground">
                    <UserX class="h-8 w-8 opacity-40" aria-hidden="true" />
                    <p class="text-sm">No ignored commenters.</p>
                  </div>
                </div>
              </li>
              <li v-for="(item, i) in loadError ? [] : commenters ?? []" :key="`item:${item.login}`" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div :class="[row, 'group wow-rise py-2']" :style="{ '--i': rowDelay(i) }">
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
          <Transition v-bind="alarm">
            <p v-if="addError" class="text-xs text-destructive" role="alert">{{ addError }}</p>
          </Transition>
        </TabsContent>

        <TabsContent value="commands" class="space-y-3">
          <p class="text-xs text-muted-foreground">
            CLI commands to run on a comment.
            <code v-pre class="rounded bg-muted px-1 font-mono">{{instructions}}</code>
            is its text; leave it out to pipe the text to stdin.
          </p>
          <div :class="box">
            <Transition v-bind="rows">
              <div v-if="composingCmd" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <CommandComposer
                    :key="editingCmd?.name ?? ':new'"
                    class="border-b border-border"
                    :editing="editingCmd"
                    :save="saveCommand"
                    @cancel="closeComposer"
                  />
                </div>
              </div>
            </Transition>
            <TransitionGroup tag="ul" v-bind="rows" class="divide-y divide-border" aria-label="Commands list">
              <li v-if="loadCmdError" key="error" class="grid" role="alert">
                <div class="min-h-0 overflow-hidden">
                  <p class="px-4 py-3 text-xs text-destructive">{{ loadCmdError }}</p>
                </div>
              </li>
              <li v-else-if="commands?.length === 0" key="empty" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div class="flex flex-col items-center gap-2 py-6 text-muted-foreground">
                    <Terminal class="h-8 w-8 opacity-30" aria-hidden="true" />
                    <p class="text-sm">No commands configured.</p>
                  </div>
                </div>
              </li>
              <li v-for="(cmd, i) in loadCmdError ? [] : commands ?? []" :key="`item:${cmd.name}`" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div :class="[row, 'group wow-rise py-2.5 transition-colors', editingCmd?.name === cmd.name && 'bg-accent/40']" :style="{ '--i': rowDelay(i) }">
                    <div class="min-w-0 flex-1">
                      <p class="text-sm font-medium">{{ cmd.name }}</p>
                      <p class="truncate font-mono text-xs text-muted-foreground">{{ cmd.command }}</p>
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-6 w-6 shrink-0 text-muted-foreground opacity-0 transition-opacity hover:text-foreground group-hover:opacity-100 focus-visible:opacity-100"
                      :aria-label="`Edit command ${cmd.name}`"
                      title="Edit"
                      @click="openComposer(cmd)"
                    >
                      <Pencil class="h-3.5 w-3.5" aria-hidden="true" />
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
          <Transition v-bind="alarm">
            <p v-if="addCmdError" class="text-xs text-destructive" role="alert">{{ addCmdError }}</p>
          </Transition>
          <DefineDefaultRow v-slot="{ kind, label, hint }">
            <div :class="cn(row, kind !== 'all' && 'py-2')">
              <p class="min-w-0 flex-1 text-sm" :class="kind === 'all' ? 'flex flex-col' : 'truncate'">
                <span :id="`default-${kind}-label`" class="font-medium">{{ label }}</span>
                <span v-if="hint" :id="`default-${kind}-hint`" class="text-xs text-muted-foreground" :class="kind !== 'all' && 'ml-2'">{{ hint }}</span>
              </p>
              <span class="relative inline-flex shrink-0 rounded-md">
                <Select
                  :model-value="defaultCommands[kind] || (kind === 'all' ? allDefaultName : INHERIT)"
                  @update:model-value="handleSetDefault(kind, $event === INHERIT ? '' : String($event))"
                >
                  <SelectTrigger
                    :aria-labelledby="`default-${kind}-label default-${kind}-value`"
                    :aria-describedby="hint ? `default-${kind}-hint` : undefined"
                    class="h-8 w-52"
                  >
                    <!-- Custom content: the item text would show "Same as all targets" instead of the command it resolves to. -->
                    <SelectValue :id="`default-${kind}-value`" :class="!defaultCommands[kind] && kind !== 'all' && 'text-muted-foreground'">
                      {{ defaultCommands[kind] || allDefaultName }}
                      <span v-if="!defaultCommands[kind] && kind !== 'all'" class="sr-only">(same as all targets)</span>
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-if="kind !== 'all'" :value="INHERIT">Same as all targets</SelectItem>
                    <SelectItem v-for="c in commands" :key="c.name" :value="c.name">{{ c.name }}</SelectItem>
                  </SelectContent>
                </Select>
                <span v-if="bursts[`default:${kind}`]" :key="bursts[`default:${kind}`]" :class="ring" aria-hidden="true" />
              </span>
            </div>
          </DefineDefaultRow>
          <div v-if="commands?.length" :class="[card, 'wow-rise']">
            <ReuseDefaultRow kind="all" label="Default command" hint="Fires unless a target below has its own" />
            <div>
              <button
                type="button"
                :class="cn(row, 'w-full py-2 text-left text-sm transition-colors hover:bg-accent/40', !perKindOpen && 'rounded-b-[inherit]')"
                :aria-expanded="perKindOpen"
                aria-controls="per-target-defaults"
                @click="perKindOpen = !perKindOpen"
              >
                <span class="flex-1 font-medium">Customize per target</span>
                <span v-if="overrideCount" class="text-xs text-muted-foreground">{{ overrideCount }} set</span>
                <ChevronDown class="h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-fast" :class="perKindOpen && 'rotate-180'" aria-hidden="true" />
              </button>
              <Reveal id="per-target-defaults" :open="perKindOpen" axis="y">
                <div class="divide-y divide-border border-t border-border">
                  <ReuseDefaultRow v-for="k in RUN_KINDS" :key="k.id" :kind="k.id" :label="k.label" :hint="k.hint" />
                </div>
              </Reveal>
            </div>
          </div>
          <ReuseToggles section="commands" />
        </TabsContent>

        <TabsContent value="projects" class="space-y-3">
          <p class="text-xs text-muted-foreground">
            When a pull request has no local path, gitura looks for its clone up to three levels deep in these
            folders, preferring a worktree on the PR's branch.
          </p>
          <div :class="box">
            <TransitionGroup tag="ul" v-bind="rows" class="divide-y divide-border" aria-label="Project folders list">
              <li v-if="projectDirs?.length === 0" key="empty" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div class="flex flex-col items-center gap-2 py-6 text-muted-foreground">
                    <FolderGit2 class="h-8 w-8 opacity-40" aria-hidden="true" />
                    <p class="text-sm">No project folders.</p>
                  </div>
                </div>
              </li>
              <li v-for="(dir, i) in projectDirs ?? []" :key="`dir:${dir}`" class="grid">
                <div class="min-h-0 overflow-hidden">
                  <div :class="[row, 'group wow-rise py-2']" :style="{ '--i': rowDelay(i) }">
                    <span class="min-w-0 flex-1 truncate font-mono text-sm">{{ dir }}</span>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-6 w-6 text-destructive opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100 focus-visible:opacity-100"
                      :aria-label="`Remove ${dir} from project folders`"
                      @click="saveProjectDirs((projectDirs ?? []).filter((d) => d !== dir))"
                    >
                      <Trash2 class="h-3.5 w-3.5" aria-hidden="true" />
                    </Button>
                  </div>
                </div>
              </li>
            </TransitionGroup>
          </div>
          <Transition v-bind="alarm">
            <p v-if="projectError" class="text-xs text-destructive" role="alert">{{ projectError }}</p>
          </Transition>
        </TabsContent>

        <TabsContent value="keybindings" class="space-y-3">
          <p class="text-xs text-muted-foreground">Click a shortcut, then press the new key. Escape cancels.</p>
          <ul :class="card">
            <li v-for="(action, i) in KEYBINDING_ACTIONS" :key="action.id" :class="[row, 'wow-rise py-2.5']" :style="{ '--i': i }">
              <div class="min-w-0 flex-1">
                <p class="text-sm font-medium">{{ action.label }}</p>
                <p class="text-xs text-muted-foreground">{{ action.view }}</p>
              </div>
              <Button
                variant="outline"
                size="sm"
                class="relative grid h-7 min-w-24 justify-items-center font-mono text-xs"
                :class="recording === action.id && 'wow-glow'"
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
                <span v-if="bursts[action.id]" :key="bursts[action.id]" :class="ring" aria-hidden="true" />
              </Button>
            </li>
          </ul>
          <Transition v-bind="alarm">
            <p v-if="bindError" class="text-xs text-destructive" role="alert">{{ bindError }}</p>
          </Transition>
        </TabsContent>
      </div>
      </Transition>
      </div>
    </div>
  </Tabs>
</template>
