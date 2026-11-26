import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive } from 'vue'
import ReviewSubmittedNote from './ReviewSubmittedNote.vue'
import SubmoduleReviewChoices, { keepChoices, submitBlocker, submoduleRequest, type SubmoduleChoice } from './SubmoduleReviewChoices.vue'
import type * as model from '@/bindings/gitura/internal/model'

const withPR: model.SubmoduleDraftsDTO = { path: 'vendor/lib', pr: 'so/lib#5', pr_url: 'https://github.com/so/lib/pull/5', to_pr: 2, folded: 1 }
const noPR: model.SubmoduleDraftsDTO = { path: 'vendor/other', to_pr: 0, folded: 3 }
const noDrafts: model.SubmoduleDraftsDTO = { path: 'vendor/idle', pr: 'so/idle#1', pr_url: 'https://github.com/so/idle/pull/1', to_pr: 0, folded: 0 }

function render(drafts: model.SubmoduleDraftsDTO[]) {
  const choices = reactive<Record<string, SubmoduleChoice>>(keepChoices(drafts, {}))
  const wrapper = mount(SubmoduleReviewChoices, { props: { drafts, choices }, attachTo: document.body })
  return { wrapper, choices }
}

describe('SubmoduleReviewChoices', () => {
  it('shows a choice for a submodule with a PR, one line for one without, and nothing for one without drafts', () => {
    const { wrapper } = render([withPR, noPR, noDrafts])
    const rows = wrapper.findAll('[data-submodule]')
    expect(rows.map((r) => r.attributes('data-submodule'))).toEqual(['vendor/lib', 'vendor/other'])
    expect(rows[0].text()).toContain('so/lib#5')
    expect(rows[0].text()).toContain('3 comments')
    expect(rows[0].text()).toContain("1 comment outside its PR's diff goes into this review's text.")
    expect(rows[1].find('[role="tablist"]').exists()).toBe(false)
    expect(rows[1].text()).toContain("3 comments added to this review's text")
    wrapper.unmount()
  })

  it('names both reasons when folded drafts may be outdated or outside the PR diff', () => {
    const { wrapper } = render([{ ...withPR, folded: 2, outdated: 1 }, { ...noPR, pr: 'so/other#2', outdated: 1 }])
    const rows = wrapper.findAll('[data-submodule]')
    expect(rows[0].text()).toContain("2 comments go into this review's text: written on an earlier version of the submodule or outside its PR's diff.")
    expect(rows[1].text()).toContain("(written on an earlier version of the submodule or outside so/other#2's diff)")
    wrapper.unmount()
  })

  it('renders nothing when no submodule has drafts', () => {
    expect(render([]).wrapper.html()).not.toContain('Comments on submodule files')
    expect(render([noDrafts]).wrapper.html()).not.toContain('Comments on submodule files')
  })

  it('sends the picked verdict and text, or the fold choice, with the routing shown', async () => {
    const { wrapper, choices } = render([withPR, noPR])
    const shown = { path: 'vendor/lib', pr: 'so/lib#5', to_pr: 2, folded: 1 }
    const noPRFolds = { path: 'vendor/other', pr: undefined, to_pr: 0, folded: 3, mode: 'fold' }
    expect(submoduleRequest([withPR, noPR], choices)).toEqual([{ ...shown, mode: 'review', verdict: 'COMMENT', body: '' }, noPRFolds])

    await wrapper.find('input[value="REQUEST_CHANGES"]').setValue(true)
    await wrapper.find('textarea').setValue('Please rename this.')
    expect(submoduleRequest([withPR, noPR], choices)).toEqual([
      { ...shown, mode: 'review', verdict: 'REQUEST_CHANGES', body: 'Please rename this.' },
      noPRFolds,
    ])

    const fold = wrapper.findAll('[role="tab"]').find((t) => t.text() === "Add to this review's text")
    await fold?.trigger('mousedown', { button: 0 })
    expect(choices['vendor/lib'].mode).toBe('fold')
    expect(wrapper.find('[data-submodule="vendor/lib"]').text()).toContain("All 3 comments go into this review's text.")
    expect(wrapper.find('[data-submodule="vendor/lib"]').text()).not.toContain("outside its PR's diff")
    expect(wrapper.find('[inert] textarea').exists()).toBe(true)
    expect(submoduleRequest([withPR, noPR], choices)[0].mode).toBe('fold')
    wrapper.unmount()
  })

  it('sends single comments without a verdict or text to fill in', async () => {
    const { wrapper, choices } = render([withPR])
    const tab = (label: string) => wrapper.findAll('[role="tab"]').find((t) => t.text() === label)
    expect(wrapper.findAll('[role="tab"]').map((t) => t.text())).toEqual(['Review', 'Single comments', "Add to this review's text"])

    await tab('Single comments')?.trigger('mousedown', { button: 0 })
    expect(choices['vendor/lib'].mode).toBe('comments')
    expect(wrapper.find('[inert] [role="radiogroup"]').exists()).toBe(true)
    expect(wrapper.find('[inert] textarea').exists()).toBe(true)
    expect(submoduleRequest([withPR], choices)[0].mode).toBe('comments')
    wrapper.unmount()
  })

  it('disables Approve and Request changes on a submodule PR the viewer wrote', () => {
    const { wrapper } = render([{ ...withPR, own_pr: true }])
    const disabled = wrapper.findAll<HTMLInputElement>('input[type="radio"]').filter((r) => r.element.disabled).map((r) => r.attributes('value'))
    expect(disabled).toEqual(['APPROVE', 'REQUEST_CHANGES'])
    const titles = wrapper.findAll('[role="radiogroup"] label').map((l) => l.attributes('title'))
    expect(titles).toEqual([undefined, 'Not available on your own pull request.', 'Not available on your own pull request.'])
    wrapper.unmount()
  })

  it('holds the mode an unsettled earlier submit sent with', () => {
    const held = { ...withPR, in_flight: 'comments' }
    expect(keepChoices([held], { 'vendor/lib': { mode: 'fold', verdict: 'COMMENT', body: 'x' } })['vendor/lib']).toEqual({ mode: 'comments', verdict: 'COMMENT', body: 'x' })
    const { wrapper, choices } = render([held])
    expect(choices['vendor/lib'].mode).toBe('comments')
    const disabled = wrapper.findAll('[role="tab"]').filter((t) => t.attributes('disabled') !== undefined).map((t) => t.text())
    expect(disabled).toEqual(['Review', "Add to this review's text"])
    expect(wrapper.text()).toContain('An earlier submit may have reached so/lib#5; submit again to settle it.')
    wrapper.unmount()
    expect(render([withPR]).wrapper.text()).not.toContain('An earlier submit')
  })

  it('holds the verdict and text of an unsettled earlier review, read-only', () => {
    const held = { ...withPR, in_flight: 'review', in_flight_review: { verdict: 'REQUEST_CHANGES', body: 'sent' } }
    expect(keepChoices([held], { 'vendor/lib': { mode: 'review', verdict: 'COMMENT', body: 'edited' } })['vendor/lib']).toEqual({ mode: 'review', verdict: 'REQUEST_CHANGES', body: 'sent' })
    const { wrapper } = render([held])
    expect(wrapper.findAll<HTMLInputElement>('input[type="radio"]').every((r) => r.element.disabled)).toBe(true)
    expect(wrapper.find<HTMLTextAreaElement>('textarea').element.disabled).toBe(true)
    wrapper.unmount()
  })

  it('says drafts written on an earlier commit fold for that reason, not for being outside the PR', () => {
    const { wrapper } = render([{ path: 'vendor/lib', pr: 'so/lib#5', to_pr: 0, folded: 2, outdated: 2 }])
    const row = wrapper.find('[data-submodule="vendor/lib"]')
    expect(row.text()).toContain('written on an earlier version of the submodule')
    expect(row.text()).not.toContain('outside')
    wrapper.unmount()
  })

  it('says a failed open-PR lookup folds the comments, with a retry', async () => {
    const { wrapper } = render([{ path: 'vendor/lib', to_pr: 0, folded: 2, pr_unknown: true }])
    const row = wrapper.find('[data-submodule="vendor/lib"]')
    expect(row.text()).toContain("Couldn't check vendor/lib for an open pull request — its comments will be added to this review's text")
    expect(row.find('[role="tablist"]').exists()).toBe(false)
    await row.find('button').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
    wrapper.unmount()
  })
})

describe('ReviewSubmittedNote', () => {
  it('counts the single comments per submodule PR', () => {
    const url = (n: number) => `https://github.com/so/lib/pull/5#discussion_r${n}`
    const wrapper = mount(ReviewSubmittedNote, { props: { result: { review_id: 1, html_url: '', submodule_comment_urls: [url(1), url(2), url(3)] } } })
    expect(wrapper.text()).toContain('3 comments posted on so/lib#5')
    expect(wrapper.find('a').attributes('href')).toBe(url(1))
  })
})

describe('submitBlocker', () => {
  const onSub: model.DraftCommentDTO = { path: 'vendor/lib/a.c', body: 'x', line: 1, side: 'RIGHT', submodule: 'vendor/lib' }
  const onMain: model.DraftCommentDTO = { path: 'main.go', body: 'x', line: 1, side: 'RIGHT' }
  const routed = { ...withPR, folded: 0 }
  const review = keepChoices([routed], {})
  const fold = { 'vendor/lib': { ...review['vendor/lib'], mode: 'fold' as const } }

  it('lets an approval, a review with text, or one with a comment for this PR through', () => {
    expect(submitBlocker('APPROVE', '', [], [], {})).toBe('')
    expect(submitBlocker('REQUEST_CHANGES', 'fix it', [], [], {})).toBe('')
    expect(submitBlocker('REQUEST_CHANGES', '', [onMain], [], {})).toBe('')
  })

  it('counts folded comments and ones a fold choice keeps here as comments for this PR', () => {
    expect(submitBlocker('REQUEST_CHANGES', '', [onSub], [{ ...routed, to_pr: 0, folded: 1 }], {})).toBe('')
    expect(submitBlocker('REQUEST_CHANGES', '', [onSub, onSub], [routed], fold)).toBe('')
  })

  it('stops a request for changes with no text when every comment goes to a submodule PR', () => {
    expect(submitBlocker('REQUEST_CHANGES', '  ', [onSub, onSub], [routed], review)).toContain('every comment goes to a submodule PR')
    expect(submitBlocker('REQUEST_CHANGES', '', [], [], {})).toBe('Requesting changes needs text or a comment.')
  })

  it('lets a comment review through when something goes to a submodule PR, and stops one with nothing to send', () => {
    expect(submitBlocker('COMMENT', '', [onSub, onSub], [routed], review)).toBe('')
    expect(submitBlocker('COMMENT', ' ', [], [], {})).toBe('Add text or a comment to submit.')
  })
})
