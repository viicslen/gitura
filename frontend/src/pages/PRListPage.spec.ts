import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PRListPage from './PRListPage.vue'
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

const listOpenPRs = vi.hoisted(() => vi.fn())
vi.mock('@/bindings/gitura/internal/app/app', () => ({ ListOpenPRs: listOpenPRs }))

describe('PRListPage org filter', () => {
  beforeEach(() => {
    listOpenPRs.mockImplementation(() => Promise.resolve({ items: [pr('FmTod', 'app', 1), pr('FmTod2', 'config', 2)] }))
  })
  afterEach(() => {
    usePRFilters(() => {}).clearAllFilters()
  })

  it('treats a typed org name as picked, excluding orgs that share its prefix', async () => {
    const wrapper = mount(PRListPage)
    await flushPromises()
    expect(wrapper.findAll('li')).toHaveLength(2)

    const input = wrapper.get('input[aria-label="Filter by organisation"]')
    await input.trigger('focus')
    await input.setValue('fmtod')
    await input.trigger('blur')
    await flushPromises()

    const rows = wrapper.findAll('[role="list"] > li')
    expect(rows.map((r) => r.text())).toEqual([expect.stringContaining('FmTod/app')])
    expect((input.element as HTMLInputElement).value).toBe('FmTod')
  })

  it('keeps substring matching for partial org text', async () => {
    const wrapper = mount(PRListPage)
    await flushPromises()

    const input = wrapper.get('input[aria-label="Filter by organisation"]')
    await input.setValue('fmt')
    await input.trigger('blur')
    await flushPromises()

    expect(wrapper.findAll('[role="list"] > li')).toHaveLength(2)
  })

  it('applies review-view updates until search reflects them', async () => {
    const prUpdates = new Map([
      ['FmTod/app#1', { owner: 'FmTod', repo: 'app', number: 1, title: 'x', state: 'merged' }],
      ['FmTod2/config#2', { owner: 'FmTod2', repo: 'config', number: 2, title: 'Renamed', state: 'open' }],
      ['gone/repo#9', { owner: 'gone', repo: 'repo', number: 9, title: 'x', state: 'closed' }],
    ])
    const wrapper = mount(PRListPage, { props: { prUpdates } })
    await flushPromises()

    expect(wrapper.findAll('[role="list"] > li').map((r) => r.text())).toEqual([expect.stringContaining('Renamed')])
    expect([...prUpdates.keys()]).toEqual(['FmTod/app#1', 'FmTod2/config#2'])
  })

  it('shows the skeleton on first load only and keeps the list on screen during a refresh', async () => {
    const wrapper = mount(PRListPage)
    expect(wrapper.find('[aria-label="Loading pull requests"]').exists()).toBe(true)
    await flushPromises()

    listOpenPRs.mockReturnValueOnce(new Promise(() => {}))
    await wrapper.get('button[aria-label="Refresh pull request list"]').trigger('click')

    expect(wrapper.find('[aria-label="Loading pull requests"]').exists()).toBe(false)
    expect(wrapper.findAll('[role="list"] > li')).toHaveLength(2)
  })

  it('keeps the error on screen while a retry is in flight', async () => {
    listOpenPRs.mockRejectedValueOnce(new Error('boom'))
    const wrapper = mount(PRListPage)
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('boom')

    listOpenPRs.mockReturnValueOnce(new Promise(() => {}))
    await wrapper.get('button[aria-label="Refresh pull request list"]').trigger('click')
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })

  it('marks each row and its title for the shell, and rings only the rows a refresh added or changed', async () => {
    const wrapper = mount(PRListPage)
    await flushPromises()
    const row = wrapper.get('[data-pr-key="FmTod/app#1"]')
    expect(row.element.tagName).toBe('LI')
    expect(row.get('[data-pr-title]').text()).toBe('FmTod/app change')
    expect(wrapper.find('.wow-ring').exists()).toBe(false)

    listOpenPRs.mockResolvedValueOnce({ items: [pr('FmTod', 'app', 1), { ...pr('FmTod2', 'config', 2), title: 'Renamed' }, pr('FmTod', 'app', 3)] })
    await wrapper.get('button[aria-label="Refresh pull request list"]').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.wow-ring').map((r) => r.attributes('data-pr-key'))).toEqual(['FmTod2/config#2', 'FmTod/app#3'])
  })

  it('keeps the list when a refresh answers with an error', async () => {
    const wrapper = mount(PRListPage)
    await flushPromises()

    listOpenPRs.mockResolvedValueOnce({ items: [], error: 'rate limited', rate_limit_reset: '2026-01-01T00:10:00Z' })
    await wrapper.get('button[aria-label="Refresh pull request list"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('rate limit')
    expect(wrapper.findAll('[role="list"] > li')).toHaveLength(2)
  })
})
