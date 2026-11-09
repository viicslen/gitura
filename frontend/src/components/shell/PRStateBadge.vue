<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
defineProps<{ state?: string; isDraft?: boolean }>()
</script>

<template>
  <Badge
    v-if="state === 'merged'"
    class="text-xs shrink-0 bg-violet-500/15 text-violet-700 dark:text-violet-300 border-violet-500/30"
  >Merged</Badge>
  <Badge v-else-if="state === 'closed'" variant="destructive" class="text-xs shrink-0">Closed</Badge>
  <Badge v-else-if="isDraft" variant="secondary" class="text-xs shrink-0">Draft</Badge>
  <Badge v-else class="text-xs shrink-0 bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border-emerald-500/30">Open</Badge>
</template>
