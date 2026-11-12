<script setup lang="ts">
import { Minus, Square, X } from '@lucide/vue'
import { Application, Window } from '@wailsio/runtime'

defineProps<{ vertical?: boolean }>()
</script>

<template>
  <div
    class="flex shrink-0 self-stretch"
    :class="vertical ? 'flex-col items-center gap-1 py-2' : 'items-stretch'"
    style="--wails-draggable: no-drag"
  >
    <button
      class="grid place-items-center text-muted-foreground hover:bg-accent hover:text-foreground"
      :class="vertical ? 'h-7 w-7 rounded-md' : 'w-11'"
      aria-label="Minimise window"
      @click="Window.Minimise()"
    >
      <Minus class="h-3.5 w-3.5" />
    </button>
    <button
      class="grid place-items-center text-muted-foreground hover:bg-accent hover:text-foreground"
      :class="vertical ? 'h-7 w-7 rounded-md' : 'w-11'"
      aria-label="Maximise or restore window"
      @click="Window.ToggleMaximise()"
    >
      <Square class="h-3 w-3" />
    </button>
    <button
      class="grid place-items-center text-muted-foreground hover:bg-red-600 hover:text-white"
      :class="vertical ? 'h-7 w-7 rounded-md' : 'w-11'"
      aria-label="Close window"
      @click="Application.Quit()"
    >
      <X class="h-3.5 w-3.5" />
    </button>
  </div>
</template>
