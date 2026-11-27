<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Minus, Square, X } from '@lucide/vue'
import { Application, Window } from '@wailsio/runtime'
import { WindowButtons } from '@/bindings/gitura/internal/app/app'

defineProps<{ vertical?: boolean }>()

const icon = 'transition-transform duration-slow ease-spring group-active:scale-75'

// Empty until the window manager answers, so buttons it hides never flash in.
const buttons = ref<string[]>([])
onMounted(async () => {
  buttons.value = await WindowButtons().catch(() => ['minimize', 'maximize', 'close'])
})
</script>

<template>
  <div
    v-if="buttons.length"
    class="flex shrink-0 self-stretch animate-in fade-in-0"
    :class="vertical ? 'flex-col items-center gap-1 py-2' : 'items-stretch'"
    style="--wails-draggable: no-drag"
  >
    <button
      v-if="buttons.includes('minimize')"
      class="group grid place-items-center text-muted-foreground hover:bg-accent hover:text-foreground"
      :class="vertical ? 'h-7 w-7 rounded-md' : 'w-11'"
      aria-label="Minimise window"
      @click="Window.Minimise()"
    >
      <Minus :class="[icon, 'h-3.5 w-3.5 group-hover:translate-y-1']" />
    </button>
    <button
      v-if="buttons.includes('maximize')"
      class="group grid place-items-center text-muted-foreground hover:bg-accent hover:text-foreground"
      :class="vertical ? 'h-7 w-7 rounded-md' : 'w-11'"
      aria-label="Maximise or restore window"
      @click="Window.ToggleMaximise()"
    >
      <Square :class="[icon, 'h-3 w-3 group-hover:scale-125']" />
    </button>
    <button
      v-if="buttons.includes('close')"
      class="group grid place-items-center text-muted-foreground hover:bg-red-600 hover:text-white"
      :class="vertical ? 'h-7 w-7 rounded-md' : 'w-11'"
      aria-label="Close window"
      @click="Application.Quit()"
    >
      <X :class="[icon, 'h-3.5 w-3.5 group-hover:rotate-90']" />
    </button>
  </div>
</template>
