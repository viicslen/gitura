<script setup lang="ts">
import type { PrimitiveProps } from "reka-ui"
import type { HTMLAttributes } from "vue"
import type { ButtonVariants } from "."
import { Primitive } from "reka-ui"
import { cn } from "@/lib/utils"
import { buttonVariants } from "."

interface Props extends PrimitiveProps {
  variant?: ButtonVariants["variant"]
  size?: ButtonVariants["size"]
  class?: HTMLAttributes["class"]
}

const props = withDefaults(defineProps<Props>(), {
  as: "button",
})
</script>

<template>
  <Primitive
    data-slot="button"
    :data-variant="variant"
    :data-size="size"
    :as="as"
    :as-child="asChild"
    :class="cn(buttonVariants({ variant, size }), props.class)"
  >
    <slot />
  </Primitive>
</template>

<!-- A layer below utilities, so a transition-* class on one button still replaces this. -->
<style>
@layer theme, base, components, utilities;

@layer components {
  /* The press squishes fast; letting go springs back on the slower scale transition. */
  [data-slot="button"] {
    transition:
      color var(--transition-duration-fast) var(--ease-out),
      background-color var(--transition-duration-fast) var(--ease-out),
      border-color var(--transition-duration-fast) var(--ease-out),
      box-shadow var(--transition-duration-fast) var(--ease-out),
      opacity var(--transition-duration-fast) var(--ease-out),
      scale var(--transition-duration-slower) var(--ease-spring);
  }
  [data-slot="button"]:active {
    scale: 1.03 0.9;
    transition-duration: var(--transition-duration-faster);
    transition-timing-function: var(--ease-out);
  }
}
</style>
