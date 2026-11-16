<script lang="ts">
import { errorMessage } from '@/lib/utils'
import { reactive } from 'vue'

// Shared by every view of a comment, so switching tabs keeps an unsaved edit.
// `base` is the body the edit started from; saving fails if GitHub no longer holds it.
const edits = reactive<Record<string, { base: string; text: string; saving?: boolean }>>({})
</script>

<script setup lang="ts">
import { computed, inject, ref, type Ref } from 'vue'
import { Pencil, Trash2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DeleteComment, UpdateBodyIfUnchanged } from '@/bindings/gitura/internal/app/app'
import type { TaskSource } from '@/lib/taskList'
import BusyLabel from '@/components/motion/BusyLabel.vue'
import SwapBox from '@/components/motion/SwapBox.vue'
import { collapse, collapsible, crossfade } from '@/components/motion/motion'

const props = defineProps<{
  kind: Exclude<TaskSource, 'pr_body'> | 'draft'
  id: number
  target: { body: string; author_login?: string }
  /** Overrides how an edit and a delete reach the backend, e.g. for local drafts. */
  save?: (base: string, next: string) => Promise<string>
  remove?: () => Promise<void>
}>()

const viewerLogin = inject<Ref<string>>('viewerLogin', ref(''))
const canEdit = computed(() => props.kind === 'draft' || (!!viewerLogin.value && viewerLogin.value === props.target.author_login))
const key = computed(() => `${props.kind}:${props.id}`)
const edit = computed(() => edits[key.value])
const error = ref('')
const onDeleted = inject<() => Promise<void> | void>('commentDeleted', () => {})
const confirmingDelete = ref(false)
const deleting = ref(false)
const deleteError = ref('')

async function remove(): Promise<void> {
  if (props.kind === 'review' || deleting.value) return
  deleting.value = true
  try {
    if (props.remove) {
      await props.remove()
    } else {
      await DeleteComment(props.kind, props.id)
      confirmingDelete.value = false
      await onDeleted()
    }
  } catch (err) {
    deleteError.value = errorMessage(err)
  } finally {
    deleting.value = false
  }
}

function start(): void {
  edits[key.value] = { base: props.target.body, text: props.target.body }
  error.value = ''
}

function cancel(): void {
  if (!edit.value?.saving) delete edits[key.value]
}

async function save(): Promise<void> {
  const e = edit.value
  if (!e || e.saving || !e.text.trim() || e.text === e.base) return
  e.saving = true
  try {
    props.target.body = await (props.save ?? ((base, next) => UpdateBodyIfUnchanged(props.kind, props.id, base, next)))(e.base, e.text)
    delete edits[key.value]
    error.value = ''
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    e.saving = false
  }
}

function onKeydown(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    void save()
  } else if (event.key === 'Escape') {
    event.preventDefault()
    cancel()
  }
}
</script>

<template>
  <SwapBox>
    <div v-if="edit" class="space-y-2">
      <Textarea
        v-model="edit.text"
        rows="5"
        aria-label="Edit comment"
       
        :disabled="edit.saving"
        @keydown="onKeydown"
      />
      <div class="flex flex-wrap items-center gap-2">
        <span class="grid min-w-0">
          <Transition v-bind="crossfade">
            <p v-if="error" :key="error" class="[grid-area:1/1] truncate text-xs text-destructive wow-shake" :title="error">{{ error }}</p>
            <span v-else class="[grid-area:1/1] text-xs text-muted-foreground">Ctrl+Enter to save · Esc to cancel</span>
          </Transition>
        </span>
        <div class="ml-auto flex gap-2">
          <Button variant="outline" size="sm" :disabled="edit.saving" @click="cancel">Cancel</Button>
          <Button size="sm" :disabled="edit.saving || !edit.text.trim() || edit.text === edit.base" @click="save">
            <BusyLabel :busy="!!edit.saving">Save</BusyLabel>
          </Button>
        </div>
      </div>
    </div>
    <div v-else class="group/edit relative">
      <slot />
      <div v-if="canEdit" class="absolute right-1 bottom-1 flex gap-0.5 opacity-0 transition-opacity duration-faster group-hover/edit:opacity-100 group-hover/edit:duration-0 focus-within:opacity-100 focus-within:duration-0">
        <Button variant="ghost" size="icon" class="h-6 w-6" aria-label="Edit comment" title="Edit comment" @click="start">
          <Pencil class="h-3.5 w-3.5" />
        </Button>
        <!-- GitHub can't delete a submitted review, only its comments. -->
        <Button
          v-if="kind !== 'review'"
          variant="ghost"
          size="icon"
          class="h-6 w-6 hover:text-destructive"
          aria-label="Delete comment"
          title="Delete comment"
          @click="confirmingDelete = true; deleteError = ''"
        >
          <Trash2 class="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  </SwapBox>

  <Dialog :open="confirmingDelete" @update:open="confirmingDelete = deleting || $event">
    <DialogContent class="max-w-sm">
      <DialogHeader>
        <DialogTitle>Delete comment?</DialogTitle>
      </DialogHeader>
      <div>
        <p class="text-sm text-muted-foreground">
          {{ kind === 'draft' ? 'This removes the comment from your pending review.' : 'This deletes the comment on GitHub and cannot be undone.' }}
        </p>
        <Transition v-bind="collapse">
          <div v-if="deleteError" :class="collapsible">
            <div class="min-h-0"><p class="pt-2 text-sm text-destructive wow-shake">{{ deleteError }}</p></div>
          </div>
        </Transition>
      </div>
      <DialogFooter>
        <Button variant="ghost" :disabled="deleting" @click="confirmingDelete = false">Cancel</Button>
        <Button variant="destructive" :disabled="deleting" @click="remove">
          <BusyLabel :busy="deleting">Delete</BusyLabel>
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
