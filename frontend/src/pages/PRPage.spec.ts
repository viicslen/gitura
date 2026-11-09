import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PRPage from './PRPage.vue'
import { usePRFilters } from '@/composables/usePRFilters'

function pr(owner: string, repo: string, number: number) {
  return {
    number,
    title: `${owner}/${repo} change`,
    owner,
    repo,
    author_login: 'me',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    html_url: `https://github.com/${owner}/${repo}/pull/${number}`,
    is_draft: false,
    is_author: true,
    is_assignee: false,
    is_reviewer: false,
  }
}

vi.mock('../wailsjs/go/main/App', () => ({
  ListOpenPRs: () => Promise.resolve({ items: [pr('FmTod', 'app', 1), pr('FmTod2', 'config', 2)] }),
}))

describe('PRPage org filter', () => {
  beforeEach(() => vi.useFakeTimers({ toFake: ['setTimeout'] }))
  afterEach(() => {
    usePRFilters(() => {}).clearAllFilters()
    vi.useRealTimers()
  })

  it('treats a typed org name as picked, excluding orgs that share its prefix', async () => {
    const wrapper = mount(PRPage)
    await flushPromises()
    expect(wrapper.findAll('li')).toHaveLength(2)

    const input = wrapper.get('input[aria-label="Filter by organisation"]')
    await input.trigger('focus')
    await input.setValue('fmtod')
    await input.trigger('blur')
    vi.advanceTimersByTime(200)
    await flushPromises()

    const rows = wrapper.findAll('ul[role="list"] > li')
    expect(rows.map((r) => r.text())).toEqual([expect.stringContaining('FmTod/app')])
    expect((input.element as HTMLInputElement).value).toBe('FmTod')
  })

  it('keeps substring matching for partial org text', async () => {
    const wrapper = mount(PRPage)
    await flushPromises()

    const input = wrapper.get('input[aria-label="Filter by organisation"]')
    await input.setValue('fmt')
    await input.trigger('blur')
    vi.advanceTimersByTime(200)
    await flushPromises()

    expect(wrapper.findAll('ul[role="list"] > li')).toHaveLength(2)
  })
})
