<script lang="ts">
export type Verdict = 'APPROVE' | 'REQUEST_CHANGES' | 'COMMENT'

export const isVerdict = (v: string): v is Verdict => v === 'APPROVE' || v === 'REQUEST_CHANGES' || v === 'COMMENT'
</script>

<script setup lang="ts">
/** A review's verdict as radio cards; `compact` puts them in one row, without descriptions. */
defineProps<{ name: string; label: string; ownPr?: boolean; compact?: boolean; disabled?: boolean }>()
const verdict = defineModel<Verdict>({ required: true })

const OPTIONS: { value: Verdict; label: string; desc: string; authorAllowed: boolean }[] = [
  { value: 'COMMENT', label: 'Comment', desc: 'Submit general feedback without explicit approval.', authorAllowed: true },
  { value: 'APPROVE', label: 'Approve', desc: 'Submit feedback and approve merging these changes.', authorAllowed: false },
  { value: 'REQUEST_CHANGES', label: 'Request changes', desc: 'Submit feedback that must be addressed before merging.', authorAllowed: false },
]
const OWN_PR = 'Not available on your own pull request.'
</script>

<template>
  <div class="flex gap-2" :class="compact ? 'flex-row' : 'flex-col'" role="radiogroup" :aria-label="label">
    <label
      v-for="option in OPTIONS"
      :key="option.value"
      class="flex gap-2.5 rounded-md border border-border transition-colors"
      :class="[
        compact ? 'flex-1 items-center gap-1.5 px-2 py-1.5' : 'items-start px-3 py-2',
        verdict === option.value ? 'border-primary bg-accent' : '',
        disabled || (ownPr && !option.authorAllowed) ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer hover:bg-accent',
      ]"
      :title="compact && ownPr && !option.authorAllowed ? OWN_PR : undefined"
    >
      <input
        type="radio"
        :name="name"
        :value="option.value"
        :checked="verdict === option.value"
        :disabled="disabled || (ownPr && !option.authorAllowed)"
        :class="compact ? '' : 'mt-0.5'"
        @change="verdict = option.value"
      />
      <div>
        <div class="font-medium" :class="compact ? 'text-xs whitespace-nowrap' : 'text-sm'">{{ option.label }}</div>
        <div v-if="!compact" class="text-xs text-muted-foreground">
          {{ ownPr && !option.authorAllowed ? OWN_PR : option.desc }}
        </div>
      </div>
    </label>
  </div>
</template>
