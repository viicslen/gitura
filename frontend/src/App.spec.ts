import { describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import App from './App.vue'

const listOpenPRs = vi.fn()

vi.mock('@/bindings/gitura/internal/app/app', () => ({
  GetAuthState: () => Promise.resolve({ is_authenticated: true, login: 'me', avatar_url: '' }),
  ListOpenPRs: (...args: unknown[]) => listOpenPRs(...args),
}))

vi.mock('@wailsio/runtime', () => ({
  Browser: { OpenURL: vi.fn() },
  Events: { On: vi.fn() },
}))

// Stands in for the review view reporting the PR's state after a merge.
vi.mock('@/pages/PRReviewPage.vue', () => ({
  default: defineComponent({
    props: { prItem: { type: Object, required: true } },
    emits: ['update-pr-meta'],
    setup(props, { emit }) {
      emit('update-pr-meta', { ...props.prItem, state: 'merged' })
      return () => h('div', 'review')
    },
  }),
}))

function pr(number: number) {
  return {
    number,
    title: `PR ${number}`,
    owner: 'o',
    repo: 'r',
    author_login: 'me',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    html_url: `https://github.com/o/r/pull/${number}`,
    is_draft: false,
    is_author: true,
    is_assignee: false,
    is_reviewer: false,
  }
}

describe('App PR list sync', () => {
  it('hides a PR merged in the review view and refetches on return', async () => {
    // Search still returns the merged PR, as GitHub's index lags behind merges.
    listOpenPRs.mockResolvedValue({ items: [pr(1), pr(2)] })
    const TitleStrip = defineComponent({ emits: ['back'], setup: (_, { emit }) => () => h('button', { id: 'back', onClick: () => emit('back') }) })
    const wrapper = mount(App, { global: { stubs: { SideRail: true, TitleStrip } } })
    await flushPromises()
    const rows = () => wrapper.findAll('ul[role="list"] > li').map((r) => r.text())
    expect(rows()).toHaveLength(2)

    await wrapper.get('ul[role="list"] > li button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('review')

    await wrapper.get('#back').trigger('click')
    await flushPromises()

    expect(listOpenPRs).toHaveBeenCalledTimes(2)
    expect(rows()).toEqual([expect.stringContaining('PR 2')])
  })
})
