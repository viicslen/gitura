<script setup lang="ts" generic="T">
import { shallowRef, watch } from 'vue'
import { ROW_GLIDE } from './motion'

/**
 * A keyed list whose rows glide open when added and shut when removed. Unlike TransitionGroup it
 * never measures its rows, which would force a layout of the whole diff in the middle of a render.
 * The slot renders one row: a `grid` element with a single `min-h-0` child that holds the content.
 */
const props = defineProps<{
  items: T[]
  itemKey: (item: T) => PropertyKey
  /** Rows the list starts with glide in too. */
  appear?: boolean
}>()

defineSlots<{ default(props: { item: T }): unknown }>()

interface Row { key: PropertyKey; item: T; gone: boolean }
const fresh = (items: T[]): Row[] => items.map((item) => ({ key: props.itemKey(item), item, gone: false }))
const rows = shallowRef(fresh(props.items))

// A removed row stays in place, marked gone, until its leave transition ends.
watch(() => props.items, (items) => {
  const next = fresh(items)
  rows.value.forEach((row, i) => {
    if (!next.some((r) => r.key === row.key)) next.splice(Math.min(i, next.length), 0, { ...row, gone: true })
  })
  rows.value = next
}, { flush: 'sync' })

function drop(key: PropertyKey): void {
  rows.value = rows.value.filter((r) => !(r.gone && r.key === key))
}
</script>

<template>
  <Transition v-for="row in rows" :key="row.key" :appear="appear" v-bind="ROW_GLIDE" @after-leave="drop(row.key)">
    <slot v-if="!row.gone" :item="row.item" />
  </Transition>
</template>
