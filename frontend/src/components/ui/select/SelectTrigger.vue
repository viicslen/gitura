<script setup lang="ts">
import type { SelectTriggerProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { SelectTrigger, useForwardProps } from 'reka-ui'
import { ChevronDown } from '@lucide/vue'
import { cn } from '@/lib/utils'
import { focusGlow } from '../input'

const props = defineProps<SelectTriggerProps & { class?: HTMLAttributes['class'] }>()

const forwarded = useForwardProps(props)
</script>

<template>
  <SelectTrigger
    data-slot="select-trigger"
    v-bind="forwarded"
    :class="cn(
      'flex h-9 w-full items-center justify-between rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs transition-[border-color,box-shadow] duration-slow outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50 [&>span]:line-clamp-1',
      focusGlow,
      props.class,
    )"
  >
    <slot />
    <ChevronDown class="size-4 opacity-50 shrink-0 ml-2" />
  </SelectTrigger>
</template>
