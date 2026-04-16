<script setup lang="ts">
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

const props = defineProps<{
  modelValue: string
  options: { value: string; label: string }[]
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()
</script>

<template>
  <Tabs
    :model-value="props.modelValue"
    @update:model-value="(v) => emit('update:modelValue', v as string)"
  >
    <TabsList role="tablist" aria-label="PR view">
      <TabsTrigger
        v-for="opt in props.options"
        :key="opt.value"
        :value="opt.value"
        :aria-selected="props.modelValue === opt.value"
        :aria-label="opt.label"
      >
        {{ opt.label }}
      </TabsTrigger>
    </TabsList>
  </Tabs>
</template>
