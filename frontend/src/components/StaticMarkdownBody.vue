<script setup lang="ts">
import { computed } from 'vue'
import { renderGitHubMarkdown } from '@/lib/markdown'
import { onTaskChange, type TaskToggle } from '@/lib/taskList'

const props = defineProps<{
  content: string
  baseUrl?: string
  repoOwner?: string
  repoName?: string
  repoRef?: string
  toggleTask?: TaskToggle
}>()

const html = computed(() => renderGitHubMarkdown(props.content, { ...props, tasks: !!props.toggleTask }))
</script>

<template>
  <div class="markdown-body text-sm text-foreground" @change="onTaskChange($event, toggleTask)" v-html="html" />
</template>

<style scoped>
.markdown-body :deep(> :first-child) {
  margin-top: 0;
}

.markdown-body :deep(> :last-child) {
  margin-bottom: 0;
}

.markdown-body :deep(p) {
  margin: 0.4em 0;
}
.markdown-body :deep(p:first-child) {
  margin-top: 0;
}
.markdown-body :deep(p:last-child) {
  margin-bottom: 0;
}
.markdown-body :deep(a) {
  color: var(--color-primary);
  text-decoration: underline;
  cursor: pointer;
}
.markdown-body :deep(summary) {
  cursor: pointer;
  user-select: none;
}
.markdown-body :deep(details) {
  cursor: default;
}
.markdown-body :deep(code) {
  font-family: ui-monospace, monospace;
  font-size: 0.85em;
  background: var(--color-muted);
  border: 1px solid var(--color-border);
  border-radius: 3px;
  padding: 0.1em 0.35em;
}
.markdown-body :deep(pre) {
  background: var(--color-muted);
  border: 1px solid var(--color-border);
  border-radius: 6px;
  padding: 0.75rem 1rem;
  overflow-x: auto;
  font-size: 0.8em;
  margin: 0.5em 0;
}
.markdown-body :deep(pre code) {
  background: transparent;
  border: none;
  padding: 0;
  display: block;
  overflow-x: auto;
}
.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 0.4em 0;
  padding-left: 1.4em;
}
.markdown-body :deep(li) {
  margin: 0.15em 0;
}
.markdown-body :deep(blockquote) {
  margin: 0.5em 0;
  padding-left: 0.9em;
  border-left: 3px solid var(--color-border);
  color: var(--color-muted-foreground);
}
.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4) {
  margin: 0.75em 0 0.35em;
  font-weight: 600;
  line-height: 1.25;
}
.markdown-body :deep(hr) {
  border: 0;
  border-top: 1px solid var(--color-border);
  margin: 0.8em 0;
}
</style>
