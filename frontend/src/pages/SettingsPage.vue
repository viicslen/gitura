<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { GetIgnoredCommenters, AddIgnoredCommenter, RemoveIgnoredCommenter } from '../wailsjs/go/main/App'
import type { model } from '../wailsjs/go/models'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Trash2, UserX } from 'lucide-vue-next'

// ── State ──────────────────────────────────────────────────────────────────
const commenters = ref<model.IgnoredCommenterDTO[]>([])
const loadError = ref('')

const newLogin = ref('')
const addError = ref('')
const adding = ref(false)

const removingLogin = ref<string | null>(null)

// ── Helpers ────────────────────────────────────────────────────────────────
function formatDate(raw: unknown): string {
  if (!raw) return ''
  const d = new Date(String(raw))
  if (isNaN(d.getTime())) return ''
  return d.toLocaleDateString([], { year: 'numeric', month: 'short', day: 'numeric' })
}

// ── Lifecycle ──────────────────────────────────────────────────────────────
onMounted(async () => {
  try {
    commenters.value = await GetIgnoredCommenters()
  } catch (e: unknown) {
    loadError.value = e instanceof Error ? e.message : String(e)
  }
})

// ── Actions ────────────────────────────────────────────────────────────────
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
</script>

<template>
  <div class="p-6 max-w-lg">
    <Card>
      <CardHeader>
        <CardTitle>Settings</CardTitle>
        <CardDescription>
          Manage application preferences.
        </CardDescription>
      </CardHeader>

      <CardContent class="space-y-6">
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
              <span
                v-if="item.added_at"
                class="text-xs text-muted-foreground shrink-0"
              >
                {{ formatDate(item.added_at) }}
              </span>
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
      </CardContent>
    </Card>
  </div>
</template>
