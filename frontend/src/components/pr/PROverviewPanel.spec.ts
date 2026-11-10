import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PROverviewPanel from './PROverviewPanel.vue'
import { model } from '@/wailsjs/go/models'

function overview(review: Partial<model.ReviewDTO>): model.PROverviewDTO {
  return model.PROverviewDTO.createFrom({
    summary: { body: '' },
    general_comments: [],
    reviews: [{ review_id: 1, author_login: 'a', state: 'COMMENTED', body: 'x', ...review }],
  })
}

function sectionHeadings(o: model.PROverviewDTO): string[] {
  return mount(PROverviewPanel, { props: { overview: o } }).findAll('section h2').map((h) => h.text().split('·')[0].trim())
}

describe('PROverviewPanel labels', () => {
  it('shows each label in its GitHub color with readable text', () => {
    const o = overview({})
    o.summary.labels = [
      { name: 'bug', color: 'd73a4a', description: 'Broken' },
      { name: 'docs', color: 'fef2c0', description: '' },
      { name: 'green', color: '00ff00', description: '' },
    ]
    const chips = mount(PROverviewPanel, { props: { overview: o } }).findAll('[aria-label="Labels"] li')
    expect(chips.map((c) => [c.text(), c.attributes('style')])).toEqual([
      ['bug', 'background-color: rgb(215, 58, 74); color: rgb(255, 255, 255);'],
      ['docs', 'background-color: rgb(254, 242, 192); color: rgb(31, 35, 40);'],
      ['green', 'background-color: rgb(0, 255, 0); color: rgb(31, 35, 40);'],
    ])
    expect(chips[0].attributes('title')).toBe('Broken')
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

describe('PROverviewPanel review numbers', () => {
  it('labels reviews with the PR-wide number, not their position in the filtered list', () => {
    const o = overview({ review_id: 22 })
    o.review_numbers = { 11: 1, 22: 2 }
    expect(mount(PROverviewPanel, { props: { overview: o } }).text()).toContain('Review 2')
  })
})
