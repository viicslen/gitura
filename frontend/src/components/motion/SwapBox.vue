<script setup lang="ts">
import { ref } from 'vue'

// The slot's keyed child crossfades while the box glides between the two heights, which CSS cannot do between `auto` values.
const box = ref<HTMLElement | null>(null)

function pin(): void {
  if (!box.value) return
  box.value.style.height = `${box.value.offsetHeight}px`
  box.value.style.overflow = 'hidden'
}

function grow(el: Element): void {
  if (box.value && el instanceof HTMLElement) box.value.style.height = `${el.offsetHeight}px`
}

function release(): void {
  if (!box.value) return
  box.value.style.height = ''
  box.value.style.overflow = ''
}
</script>

<template>
  <div ref="box" class="relative transition-[height]">
    <Transition
      enter-active-class="transition-opacity duration-base"
      enter-from-class="opacity-0"
      leave-active-class="absolute inset-x-0 top-0 transition-opacity duration-fast pointer-events-none"
      leave-to-class="opacity-0"
      @before-leave="pin"
      @enter="grow"
      @after-enter="release"
    >
      <slot />
    </Transition>
  </div>
</template>
