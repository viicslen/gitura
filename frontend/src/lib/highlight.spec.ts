import { describe, it, expect } from 'vitest'
import { lineLanguages, highlightLine } from './highlight'

describe('lineLanguages for Vue files', () => {
  it('follows the blocks of a whole component', () => {
    const lines = ['<template>', '  <template v-if="x">', '  </template>', '</template>', '<script setup lang="ts">', 'const a = 1', '</script>', '<style scoped lang="scss">', '.a { color: red; }', '</style>']
    expect(lineLanguages(lines, 'vue')).toEqual(['xml', 'xml', 'xml', 'xml', 'xml', 'typescript', 'xml', 'xml', 'scss', 'xml'])
  })

  it('infers the block a diff slice starts in', () => {
    expect(lineLanguages(["import { Badge } from '@/components/ui/badge'", 'const x = 1'], 'vue')).toEqual(['typescript', 'typescript'])
    expect(lineLanguages(['  color: red;', '</style>'], 'vue')).toEqual(['css', 'xml'])
    expect(lineLanguages(['  <div class="a">', '  </div>'], 'vue')).toEqual(['xml', 'xml'])
    expect(lineLanguages(['foo()', '</script>'], 'vue')).toEqual(['typescript', 'xml'])
  })

  it('reads markup in a template hunk that follows a script hunk', () => {
    const carry = {}
    lineLanguages(['import { ref } from "vue"', 'const a = ref(1)'], 'vue', carry)
    expect(lineLanguages(['    <div class="flex">', '      <span>{{ a }}</span>', '    </div>'], 'vue', carry))
      .toEqual(['xml', 'xml', 'xml'])
  })

  it('carries the block from the previous slice when a slice has no clue of its own', () => {
    const carry = {}
    lineLanguages(['<script setup lang="ts">', 'const a = ref(1)'], 'vue', carry)
    expect(lineLanguages(['  doThing()', '  other()'], 'vue', carry)).toEqual(['typescript', 'typescript'])
    expect(lineLanguages(['  foo.value = bar'], 'vue')).toEqual(['typescript'])
  })

  it('leaves other languages alone', () => {
    expect(lineLanguages(['a', 'b'], 'go')).toEqual(['go', 'go'])
  })

  it('highlights script lines of a Vue file as code', () => {
    const [lang] = lineLanguages(["import { Badge } from '@/components/ui/badge'"], 'vue')
    expect(highlightLine("import { Badge } from '@/components/ui/badge'", lang)).toContain('hljs-keyword')
  })
})
