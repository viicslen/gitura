<script setup lang="ts">
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
} from '../wailsjs/go/main/App'
import type { model } from '../wailsjs/go/models'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Trash2, UserX, Terminal, Plus, Star } from 'lucide-vue-next'

// ── Ignored Commenters ─────────────────────────────────────────────────────
const commenters = ref<model.IgnoredCommenterDTO[]>([])
const loadError = ref('')

const newLogin = ref('')
const addError = ref('')
const adding = ref(false)

const removingLogin = ref<string | null>(null)

onMounted(async () => {
  try {
    commenters.value = await GetIgnoredCommenters()
  } catch (e: unknown) {
    loadError.value = e instanceof Error ? e.message : String(e)
  }
  try {
    commands.value = await GetCommands()
  } catch (e: unknown) {
    loadCmdError.value = e instanceof Error ? e.message : String(e)
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
    const msg = e instanceof Error ? e.message : String(e)
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
    const msg = e instanceof Error ? e.message : String(e)
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
  <div class="p-6 max-w-lg overflow-x-hidden space-y-6">
    <header>
      <h1 class="text-2xl font-semibold tracking-tight">Settings</h1>
      <p class="text-sm text-muted-foreground">
        Manage application preferences.
      </p>
    </header>

        <!-- Ignored Commenters section -->
        <section aria-labelledby="ignored-commenters-heading">
          <h2
            id="ignored-commenters-heading"
            class="text-sm font-semibold mb-1"
          >
            Ignored Commenters
          </h2>
          <p class="text-xs text-muted-foreground mb-3">
            Comments from these GitHub usernames will be hidden in the review view.
          </p>

          <!-- Add form -->
          <div class="flex gap-2 mb-3">
            <Input
              v-model="newLogin"
              placeholder="GitHub username"
              class="h-8 text-sm flex-1"
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
              Add
            </Button>
          </div>
          <p v-if="addError" class="text-xs text-destructive mb-2" role="alert">{{ addError }}</p>

          <Separator class="mb-3" />

          <!-- Load error -->
          <p v-if="loadError" class="text-xs text-destructive" role="alert">{{ loadError }}</p>

          <!-- Empty state -->
          <div
            v-else-if="commenters.length === 0"
            class="flex flex-col items-center gap-2 py-6 text-muted-foreground"
          >
            <UserX class="h-8 w-8 opacity-40" aria-hidden="true" />
            <p class="text-sm">No ignored commenters.</p>
          </div>

          <!-- List -->
          <ul
            v-else
            class="space-y-1"
            aria-label="Ignored commenters list"
          >
            <li
              v-for="item in commenters"
              :key="item.login"
              class="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50 group"
            >
              <span class="text-sm font-mono flex-1">{{ item.login }}</span>
              <Button
                variant="ghost"
                size="icon"
                class="h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity text-destructive hover:text-destructive"
                :disabled="removingLogin === item.login"
                :aria-label="`Remove ${item.login} from ignored commenters`"
                @click="handleRemove(item.login)"
              >
                <Trash2 class="h-3.5 w-3.5" aria-hidden="true" />
              </Button>
            </li>
          </ul>
        </section>

        <Separator />

        <!-- Commands section -->
        <section aria-labelledby="commands-heading">
          <h2
            id="commands-heading"
            class="text-sm font-semibold mb-1 flex items-center gap-2"
          >
            <Terminal class="h-4 w-4" aria-hidden="true" />
            Commands
          </h2>
          <p class="text-xs text-muted-foreground mb-3">
            CLI commands to run against PR comment content. Use
            <code v-pre class="font-mono bg-muted px-1 rounded">{{instructions}}</code>
            as a placeholder for the comment text, or omit it to receive the text via stdin.
          </p>

          <!-- Add form -->
          <div class="space-y-2 mb-3">
            <Input
              v-model="newCmdName"
              placeholder="Name (e.g. opencode)"
              class="h-8 text-sm"
              aria-label="Command name"
              :disabled="addingCmd"
              @keydown.enter="newCmdCommand ? handleAddCommand() : undefined"
            />
            <div class="flex gap-2">
              <Input
                v-model="newCmdCommand"
                placeholder="Command (e.g. opencode run --agent reviewer)"
                class="h-8 text-sm flex-1 font-mono"
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
                <Plus class="h-3.5 w-3.5 mr-1" />
                Add
              </Button>
            </div>
          </div>
          <p v-if="addCmdError" class="text-xs text-destructive mb-2" role="alert">{{ addCmdError }}</p>

          <Separator class="mb-3" />

          <!-- Load error -->
          <p v-if="loadCmdError" class="text-xs text-destructive" role="alert">{{ loadCmdError }}</p>

          <!-- Empty state -->
          <div
            v-else-if="commands.length === 0"
            class="flex flex-col items-center gap-2 py-6 text-muted-foreground"
          >
            <Terminal class="h-8 w-8 opacity-30" aria-hidden="true" />
            <p class="text-sm">No commands configured.</p>
          </div>

          <!-- List -->
          <ul
            v-else
            class="space-y-1"
            aria-label="Commands list"
          >
            <li
              v-for="cmd in commands"
              :key="cmd.name"
              class="flex items-start gap-2 rounded-md px-2 py-2 hover:bg-muted/50 group"
            >
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-1.5">
                  <p class="text-sm font-medium">{{ cmd.name }}</p>
                  <span
                    v-if="cmd.name === defaultCommandName"
                    class="text-xs text-muted-foreground"
                    title="Default command"
                  >
                    <Star class="h-3 w-3 fill-current text-yellow-500" aria-hidden="true" />
                  </span>
                </div>
                <p class="text-xs text-muted-foreground font-mono truncate">{{ cmd.command }}</p>
              </div>
              <!-- Set default button -->
              <Button
                v-if="cmd.name !== defaultCommandName"
                variant="ghost"
                size="icon"
                class="h-6 w-6 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-foreground"
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
                class="h-6 w-6 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity text-destructive hover:text-destructive"
                :disabled="removingCmdName === cmd.name"
                :aria-label="`Remove command ${cmd.name}`"
                @click="handleRemoveCommand(cmd.name)"
              >
                <Trash2 class="h-3.5 w-3.5" aria-hidden="true" />
              </Button>
            </li>
          </ul>
        </section>
  </div>
</template>
