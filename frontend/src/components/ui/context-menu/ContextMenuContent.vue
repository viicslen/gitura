<script setup lang="ts">
import type { ContextMenuContentEmits, ContextMenuContentProps } from "reka-ui"
import type { HTMLAttributes } from "vue"
import { reactiveOmit } from "@vueuse/core"
import { cn } from "@/lib/utils"
import "../overlay.css"
import {
  ContextMenuContent,
  ContextMenuPortal,
  useForwardPropsEmits,
} from "reka-ui"

defineOptions({
  inheritAttrs: false,
})

const props = defineProps<ContextMenuContentProps & { class?: HTMLAttributes["class"] }>()
const emits = defineEmits<ContextMenuContentEmits>()

const delegatedProps = reactiveOmit(props, "class")

const forwarded = useForwardPropsEmits(delegatedProps, emits)
</script>

<template>
  <ContextMenuPortal>
    <ContextMenuContent
      data-slot="context-menu-content"
      v-bind="{ ...$attrs, ...forwarded }"
      :class="cn(
        'bg-popover text-popover-foreground ease-out ui-cascade data-[state=open]:wow-pop data-[state=closed]:animate-out data-[state=closed]:animation-duration-faster data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 z-50 max-h-(--reka-context-menu-content-available-height) origin-(--reka-context-menu-content-transform-origin) min-w-[8rem] overflow-x-hidden overflow-y-auto rounded-md border p-1 shadow-md',
        props.class,
      )"
    >
      <slot />
    </ContextMenuContent>
  </ContextMenuPortal>
</template>
