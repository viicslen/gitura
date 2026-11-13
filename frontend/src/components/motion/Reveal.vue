<script setup lang="ts">
/**
 * Glides its slot open or shut so the neighbours move with it. The slot stays mounted, so a
 * reversal picks up from where it is. `x` is for small inline pieces, `y` for blocks.
 * In a flex row with a gap, give the closed state a negative margin of that gap.
 */
defineProps<{ open: boolean; axis: 'x' | 'y' }>()
</script>

<template>
  <!-- inert while closing, invisible once closed: neither clickable nor read out. -->
  <component
    :is="axis === 'x' ? 'span' : 'div'"
    class="grid transition-[grid-template-rows,grid-template-columns,opacity,margin,visibility]"
    :class="axis === 'x'
      ? (open ? 'grid-cols-[1fr] duration-fast' : 'invisible grid-cols-[0fr] opacity-0 duration-faster')
      : (open ? 'grid-rows-[1fr] duration-base' : 'invisible grid-rows-[0fr] opacity-0 duration-fast')"
    :inert="!open || undefined"
  >
    <!-- overflow-clip, not overflow-hidden: hidden made the diff repaint slowly on scroll under WebKitGTK. -->
    <component :is="axis === 'x' ? 'span' : 'div'" class="min-h-0 min-w-0 overflow-clip" :class="axis === 'x' ? 'flex items-center whitespace-nowrap' : ''">
      <slot />
    </component>
  </component>
</template>
