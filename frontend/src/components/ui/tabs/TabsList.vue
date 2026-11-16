<script setup lang="ts">
import type { TabsListProps } from "reka-ui"
import type { HTMLAttributes } from "vue"
import { reactiveOmit } from "@vueuse/core"
import { injectTabsRootContext, TabsIndicator, TabsList } from "reka-ui"
import { cn } from "@/lib/utils"

const props = defineProps<TabsListProps & { class?: HTMLAttributes["class"] }>()

const delegatedProps = reactiveOmit(props, "class")
const { orientation } = injectTabsRootContext()
</script>

<template>
  <TabsList
    data-slot="tabs-list"
    v-bind="delegatedProps"
    :class="cn(
      'bg-muted text-muted-foreground relative inline-flex h-9 w-fit items-center justify-center rounded-lg p-[3px]',
      props.class,
    )"
  >
    <!-- The active tab's background, one element so it can travel; first so the triggers paint over it. -->
    <TabsIndicator
      data-slot="tabs-indicator"
      aria-hidden="true"
      class="pointer-events-none absolute rounded-md bg-background shadow-sm transition-[translate,width,height] duration-slower ease-spring after:absolute after:rounded-full after:from-wow-1 after:via-wow-2 after:to-wow-3"
      :class="orientation === 'vertical'
        ? 'top-0 left-1/2 h-(--reka-tabs-indicator-size) w-(--reka-tabs-indicator-thickness) -translate-x-1/2 translate-y-(--reka-tabs-indicator-position) after:inset-y-1.5 after:left-0 after:w-0.5 after:bg-linear-to-b'
        : 'top-1/2 left-0 h-(--reka-tabs-indicator-thickness) w-(--reka-tabs-indicator-size) translate-x-(--reka-tabs-indicator-position) -translate-y-1/2 after:inset-x-2 after:bottom-0 after:h-0.5 after:bg-linear-to-r'"
    />
    <slot />
  </TabsList>
</template>
