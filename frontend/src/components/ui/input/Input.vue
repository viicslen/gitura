<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'
import { focusGlow } from '.'

const props = defineProps<{
  class?: HTMLAttributes['class']
  type?: string
  placeholder?: string
  disabled?: boolean
  modelValue?: string | number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()
</script>

<template>
  <input
    data-slot="input"
    :type="type ?? 'text'"
    :placeholder="placeholder"
    :disabled="disabled"
    :value="modelValue"
    :class="cn(
      'flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-[color,border-color,box-shadow,opacity] duration-slow disabled:delay-200 outline-none placeholder:text-muted-foreground aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50',
      focusGlow,
      props.class,
    )"
    @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
  />
</template>
