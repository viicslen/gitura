<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { ListboxContent, ListboxFilter, ListboxGroup, ListboxGroupLabel, ListboxItem, ListboxRoot } from 'reka-ui'
import { GitPullRequest, Hash, LoaderCircle, Search } from '@lucide/vue'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import PRStateBadge from './PRStateBadge.vue'
import { SearchPRs } from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'
import { myOpenPRs, parsePRNumber } from '@/lib/prQuery'
import { errorMessage } from '@/lib/utils'
import type { PRRef } from '@/types/pr'

const open = defineModel<boolean>('open', { required: true })
/** The PR under review, whose repository a bare number opens in first. */
const props = defineProps<{ current: PRRef | null }>()
const emit = defineEmits<{ (e: 'open-review', item: PRRef): void }>()

type Entry = { key: string; pr: PRRef; repo: string; author?: string }

const query = ref('')
const remote = shallowRef<model.PRListItem[]>([])
const searching = ref(false)
const searchError = ref('')

watch(open, (o) => {
  if (o) query.value = ''
})

const key = (pr: { owner: string; repo: string; number: number }) => `${pr.owner}/${pr.repo}#${pr.number}`

function toEntry(pr: model.PRListItem): Entry {
  return {
    key: key(pr),
    pr: { owner: pr.owner, repo: pr.repo, number: pr.number, title: pr.title, state: pr.state || 'open', is_draft: pr.is_draft, html_url: pr.html_url },
    repo: `${pr.owner}/${pr.repo}`,
    author: pr.author_login,
  }
}

const asked = computed(() => parsePRNumber(query.value))

const mine = computed<Entry[]>(() => {
  const n = asked.value
  const words = query.value.toLowerCase().split(/\s+/).filter(Boolean)
  const repo = n?.owner && `${n.owner}/${n.repo}`.toLowerCase()
  return myOpenPRs.value
    .filter((pr) =>
      n
        ? pr.number === n.number && (!repo || `${pr.owner}/${pr.repo}`.toLowerCase() === repo)
        : words.every((w) => `${pr.title} ${pr.owner}/${pr.repo} ${pr.author_login}`.toLowerCase().includes(w)),
    )
    .slice(0, 20)
    .map(toEntry)
})

// The PR may not exist; state stays unknown so the review page offers no merge until it has loaded.
const direct = computed<Entry[]>(() => {
  const n = asked.value
  if (!n) return []
  const repos = n.owner && n.repo
    ? [`${n.owner}/${n.repo}`]
    : [props.current && `${props.current.owner}/${props.current.repo}`, ...myOpenPRs.value.map((pr) => `${pr.owner}/${pr.repo}`)]
  const seen = new Set(mine.value.map((e) => e.key.toLowerCase()))
  return repos.flatMap((r) => {
    const k = `${r}#${n.number}`
    if (!r || seen.has(k.toLowerCase())) return []
    seen.add(k.toLowerCase())
    const [owner, repo] = r.split('/')
    return [{ key: k, pr: { owner, repo, number: n.number, title: '', state: '' }, repo: r }]
  })
})

const others = computed<Entry[]>(() => {
  const seen = new Set(mine.value.map((e) => e.key))
  return remote.value.map(toEntry).filter((e) => !seen.has(e.key))
})

// A number or URL opens directly; anything shorter than two characters would match too much to be worth a search call.
const remoteText = computed(() => {
  const t = query.value.trim()
  return t.length < 2 || asked.value ? '' : t
})

let requestId = 0
watchDebounced(
  remoteText,
  async (text) => {
    const id = ++requestId
    remote.value = []
    searchError.value = ''
    searching.value = !!text
    if (!text) return
    try {
      const res = await SearchPRs(text)
      if (id !== requestId) return
      remote.value = res.items ?? []
      searchError.value = res.error ?? ''
    } catch (e) {
      if (id === requestId) searchError.value = errorMessage(e)
    } finally {
      if (id === requestId) searching.value = false
    }
  },
  { debounce: 350 },
)

const groups = computed(() => [
  { label: query.value.trim() ? 'My open PRs' : 'Recently updated', icon: GitPullRequest, entries: mine.value },
  { label: 'Open by number', icon: Hash, entries: direct.value },
  { label: 'GitHub', icon: Search, entries: others.value },
].filter((g) => g.entries.length))

function choose(entry: Entry) {
  open.value = false
  emit('open-review', entry.pr)
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="top-[15%] translate-y-0 gap-0 overflow-hidden p-0 sm:max-w-xl" :show-close-button="false" :aria-describedby="undefined">
      <DialogTitle class="sr-only">Open a pull request</DialogTitle>
      <ListboxRoot highlight-on-hover class="min-w-0">
        <div class="flex items-center gap-2 border-b border-border px-3">
          <Search class="h-4 w-4 shrink-0 text-muted-foreground" />
          <ListboxFilter
            v-model="query"
            auto-focus
            placeholder="Search pull requests, or type a number, owner/repo#123 or a URL"
            class="h-11 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          />
          <LoaderCircle v-if="searching" class="h-4 w-4 shrink-0 animate-spin text-muted-foreground" aria-label="Searching GitHub" />
        </div>
        <ListboxContent class="max-h-[min(24rem,60vh)] overflow-y-auto p-1">
          <ListboxGroup v-for="g in groups" :key="g.label" class="pb-1">
            <ListboxGroupLabel class="px-2 py-1.5 text-xs font-medium text-muted-foreground">{{ g.label }}</ListboxGroupLabel>
            <ListboxItem
              v-for="e in g.entries"
              :key="e.key"
              :value="e.key"
              class="flex cursor-default items-center gap-2 rounded-md px-2 py-1.5 text-sm outline-none data-highlighted:bg-accent"
              @select="choose(e)"
            >
              <component :is="g.icon" class="h-4 w-4 shrink-0 text-muted-foreground" />
              <span class="min-w-0 flex-1 truncate">{{ e.pr.title || `Open #${e.pr.number}` }}</span>
              <span class="shrink-0 text-xs text-muted-foreground tabular-nums">{{ e.repo }}#{{ e.pr.number }}</span>
              <PRStateBadge v-if="g.label === 'GitHub'" :state="e.pr.state" :is-draft="e.pr.is_draft" />
            </ListboxItem>
          </ListboxGroup>
          <p v-if="searchError" class="px-2 py-1.5 text-xs text-destructive">{{ searchError }}</p>
          <p v-else-if="!groups.length && !searching" class="px-2 py-6 text-center text-sm text-muted-foreground">
            {{ query.trim() ? 'No pull requests found' : 'No open pull requests' }}
          </p>
        </ListboxContent>
      </ListboxRoot>
    </DialogContent>
  </Dialog>
</template>
