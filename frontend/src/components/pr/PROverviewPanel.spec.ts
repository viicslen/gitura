import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PROverviewPanel from './PROverviewPanel.vue'
import * as model from '@/bindings/gitura/internal/model'

const setText = vi.hoisted(() => vi.fn(() => Promise.resolve()))
vi.mock('@wailsio/runtime', async (importOriginal) => ({ ...(await importOriginal<object>()), Clipboard: { SetText: setText } }))

function overview(review: Partial<model.ReviewDTO>): model.PROverviewDTO {
  return model.PROverviewDTO.createFrom({
    summary: { body: '' },
    general_comments: [],
    reviews: [{ review_id: 1, author_login: 'a', state: 'COMMENTED', body: 'x', ...review }],
  })
}

function sectionHeadings(o: model.PROverviewDTO): string[] {
  return mount(PROverviewPanel, { props: { overview: o, timeline: null, threads: [] } }).findAll('section h2').map((h) => h.text().split('·')[0].trim())
}

describe('PROverviewPanel labels', () => {
  it('shows each label in its GitHub color with readable text', () => {
    const o = overview({})
    o.summary.labels = [
      { name: 'bug', color: 'd73a4a', description: 'Broken' },
      { name: 'docs', color: 'fef2c0', description: '' },
      { name: 'green', color: '00ff00', description: '' },
    ]
    const chips = mount(PROverviewPanel, { props: { overview: o, timeline: null, threads: [] } }).findAll('[aria-label="Labels"] li')
    expect(chips.map((c) => [c.text(), c.attributes('style')])).toEqual([
      ['bug', 'background-color: rgb(215, 58, 74); color: rgb(255, 255, 255);'],
      ['docs', 'background-color: rgb(254, 242, 192); color: rgb(31, 35, 40);'],
      ['green', 'background-color: rgb(0, 255, 0); color: rgb(31, 35, 40);'],
    ])
    expect(chips[0].attributes('title')).toBe('Broken')
  })
})

describe('PROverviewPanel branches', () => {
  it('shows base and head, marks a fork head with its owner, and copies the bare branch name', async () => {
    const o = overview({})
    Object.assign(o.summary, { owner: 'acme', base_branch: 'main', head_branch: 'fix/x', head_owner: 'someone' })
    const buttons = mount(PROverviewPanel, { props: { overview: o, timeline: null, threads: [] } }).findAll('[aria-label="Branches"] button')
    expect(buttons.map((b) => b.text())).toEqual(['main', 'someone:fix/x'])
    expect(buttons[1].attributes('aria-label')).toBe('Copy head branch someone:fix/x')

    await buttons[0].trigger('click')
    await buttons[1].trigger('click')
    expect(setText.mock.calls).toEqual([['main'], ['fix/x']])
  })

  it('omits the owner when the head is in the same repository, whatever casing the PR was opened with', () => {
    const o = overview({})
    Object.assign(o.summary, { owner: 'ACME', base_branch: 'main', head_branch: 'fix/x', head_owner: 'acme' })
    expect(mount(PROverviewPanel, { props: { overview: o, timeline: null, threads: [] } }).findAll('[aria-label="Branches"] button')[1].text()).toBe('fix/x')
  })
})

describe('PROverviewPanel review placement', () => {
  it('puts reviews above general comments while a review has unresolved conversations', () => {
    expect(sectionHeadings(overview({ has_threads: true, all_threads_resolved: false }))).toEqual(['Reviews', 'General comments'])
  })

  it('puts reviews last once every review is resolved or has no conversations', () => {
    expect(sectionHeadings(overview({ has_threads: true, all_threads_resolved: true }))).toEqual(['General comments', 'Reviews'])
    expect(sectionHeadings(overview({ has_threads: false, all_threads_resolved: false }))).toEqual(['General comments', 'Reviews'])
  })
})

describe('PROverviewPanel activity layout', () => {
  it('swaps the detail sections for the timeline and reports the choice', async () => {
    const wrapper = mount(PROverviewPanel, { props: { overview: overview({}), timeline: null, threads: [] } })
    await wrapper.get('button[aria-pressed="false"]').trigger('click')
    expect(wrapper.emitted('update:mode')).toEqual([['timeline']])

    await wrapper.setProps({ mode: 'timeline' })
    expect(wrapper.findAll('section')).toHaveLength(0)
    expect(wrapper.text()).toContain('Loading timeline')
  })
})

describe('PROverviewPanel review numbers', () => {
  it('labels reviews with the PR-wide number, not their position in the filtered list', () => {
    const o = overview({ review_id: 22 })
    o.review_numbers = { 11: 1, 22: 2 }
    expect(mount(PROverviewPanel, { props: { overview: o, timeline: null, threads: [] } }).text()).toContain('Review 2')
  })
})
