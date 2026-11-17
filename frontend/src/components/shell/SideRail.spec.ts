import { describe, it, expect, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import SideRail from './SideRail.vue'
import { useReviewTabs } from '@/composables/useReviewTabs'

const { tabBadges } = useReviewTabs()

const meta = { owner: 'acme', repo: 'web', number: 1, title: 'Fix it' }

describe('SideRail view badges', () => {
  afterEach(() => Object.assign(tabBadges, { overview: 0, conversation: 0, files: 0 }))

  it('shows counts on the review views and names them for screen readers', () => {
    Object.assign(tabBadges, { overview: 0, conversation: 3, files: 120 })
    const wrapper = mount(SideRail, { props: { meta, login: 'me' }, global: { stubs: { UserMenu: true } } })

    const overview = wrapper.get('[aria-label^="Overview"]')
    expect(overview.attributes('aria-label')).toBe('Overview')
    expect(overview.find('span').exists()).toBe(false)

    const conversation = wrapper.get('[aria-label^="Conversation"]')
    expect(conversation.attributes('aria-label')).toBe('Conversation (3 unresolved threads)')
    expect(conversation.get('span').text()).toBe('3')

    expect(wrapper.get('[aria-label^="Files changed"]').get('span').text()).toBe('99+')
  })
})
