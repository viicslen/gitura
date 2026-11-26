import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, reactive } from 'vue'
import type * as model from '@/bindings/gitura/internal/model'
import type { UnsentComments } from '@/lib/diffLayout'

const api = vi.hoisted(() => ({
  SubmoduleDrafts: vi.fn(),
  SyncPendingReview: vi.fn(),
  SubmitReview: vi.fn(),
  PostImmediateComment: vi.fn(),
  GetPRFiles: vi.fn(),
  GetFileDiff: vi.fn(),
  GetViewedFiles: vi.fn(),
}))
const toastError = vi.hoisted(() => vi.fn())

vi.mock('@/bindings/gitura/internal/app/app', () => ({
  ...api,
  GetToggles: () => Promise.resolve({}),
  GetKeybindings: () => Promise.resolve({}),
}))
vi.mock('vue-sonner', () => ({ toast: Object.assign(vi.fn(), { error: toastError }) }))

const { default: DiffReviewView } = await import('./DiffReviewView.vue')
const { default: DiffFileSidebar } = await import('./DiffFileSidebar.vue')
const { fileKey } = await import('@/lib/fileTree')

const draft = (path: string): model.DraftCommentDTO => ({ path, body: 'x', line: 1, side: 'RIGHT' })
const subDraft = (path: string): model.DraftCommentDTO => ({ ...draft(path), submodule: 'lib' })
const pending = (...comments: model.DraftCommentDTO[]): model.PendingReviewDTO => ({ review_id: 0, comments, has_pending: true, head_sha: '' })
const subDrafts: model.SubmoduleDraftsDTO[] = [{ path: 'lib', pr: 'o/lib#3', pr_url: 'https://github.com/o/lib/pull/3', to_pr: 1, folded: 0 }]

// Renders the slots DiffReviewView fills, so its notices and Viewed label can be read.
const FileViewStub = defineComponent({ setup: (_, { slots }) => () => h('div', [slots.notices?.(), slots['header-actions']?.()]) })

function mountView(headSha = '', extra: { threads?: model.CommentThreadDTO[]; local?: boolean } = {}) {
  return mount(DiffReviewView, {
    props: { threads: [], showThreads: true, isOwnPr: false, headSha, active: true, unsentComments: reactive(new Map()) as UnsentComments, ...extra },
    global: { stubs: { DiffFileSidebar: true, DiffFileView: FileViewStub } },
    attachTo: document.body,
  })
}

async function openDialog(label = 'Finish review') {
  const wrapper = mountView()
  await flushPromises()
  await wrapper.findAll('button').find((b) => b.text().includes(label))?.trigger('click')
  await flushPromises()
  const submit = () => [...document.body.querySelectorAll('button')].find((b) => b.textContent?.includes('Submit'))
  return { wrapper, submit }
}

describe('DiffReviewView submit dialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.SyncPendingReview.mockResolvedValue(pending(subDraft('lib/a.c'), draft('main.go')))
    api.SubmoduleDrafts.mockResolvedValue(subDrafts)
    api.GetPRFiles.mockResolvedValue([])
    api.GetViewedFiles.mockResolvedValue({ files: [] })
    api.GetFileDiff.mockResolvedValue({ hunks: [] })
  })

  it('keeps Submit off while the submodule choices load, and after a failed load until a retry gets them', async () => {
    let fail!: (err: Error) => void
    api.SubmoduleDrafts.mockImplementationOnce(() => new Promise((_, reject) => (fail = reject)))
    const { wrapper, submit } = await openDialog()
    expect(submit()?.disabled).toBe(true)

    fail(new Error('github: 502'))
    await flushPromises()
    expect(document.body.querySelector('[role="alert"]')?.textContent).toContain('github: 502')
    expect(submit()?.disabled).toBe(true)

    const retry = [...document.body.querySelectorAll<HTMLButtonElement>('[role="alert"] button')].find((b) => b.textContent?.includes('Retry'))
    retry?.click()
    await flushPromises()
    expect(document.body.querySelector('[role="alert"]')).toBeNull()
    expect(submit()?.disabled).toBe(false)
    expect(api.SubmoduleDrafts.mock.calls).toEqual([[false], [true]])
    wrapper.unmount()
  })

  it('waits for the first sync of the pending review before listing drafts and their submodule choices', async () => {
    let sync!: (r: model.PendingReviewDTO) => void
    api.SyncPendingReview.mockImplementationOnce(() => new Promise((resolve) => (sync = resolve)))
    const { wrapper, submit } = await openDialog('Review changes')
    expect(api.SubmoduleDrafts).not.toHaveBeenCalled()
    expect(submit()?.disabled).toBe(true)

    sync(pending(subDraft('lib/a.c'), draft('main.go')))
    await flushPromises()
    expect(api.SubmoduleDrafts).toHaveBeenCalledTimes(1)
    expect(document.body.textContent).toContain('2 pending comments')
    expect(document.body.querySelector('[data-submodule="lib"]')).not.toBeNull()
    expect(submit()?.disabled).toBe(false)
    wrapper.unmount()
  })

  it('looks up a submodule PR again past the backoff when the user retries a failed lookup', async () => {
    api.SubmoduleDrafts.mockResolvedValueOnce([{ path: 'lib', to_pr: 0, folded: 1, pr_unknown: true }])
    const { wrapper } = await openDialog()
    document.body.querySelector<HTMLButtonElement>('[data-submodule="lib"] button')?.click()
    await flushPromises()
    expect(api.SubmoduleDrafts.mock.calls).toEqual([[false], [true]])
    wrapper.unmount()
  })

  it('neither checks submodule drafts nor holds Submit when no pending draft is on a submodule file', async () => {
    api.SyncPendingReview.mockResolvedValue(pending(draft('main.go')))
    const { wrapper, submit } = await openDialog()
    expect(api.SubmoduleDrafts).not.toHaveBeenCalled()
    expect(document.body.textContent).not.toContain('Checking the comments on submodule files')
    expect(submit()?.disabled).toBe(false)
    wrapper.unmount()
  })

  it('reloads the files after a submit, for the Viewed marks checking the submodule PRs again moved', async () => {
    const { wrapper, submit } = await openDialog()
    api.SubmitReview.mockResolvedValueOnce({})
    const before = api.GetPRFiles.mock.calls.length
    submit()?.click()
    await flushPromises()
    expect(api.GetPRFiles).toHaveBeenCalledTimes(before + 1)
    wrapper.unmount()
  })

  it('reloads the drafts after a failed submit and says when some were already posted', async () => {
    const { wrapper, submit } = await openDialog()
    api.SubmitReview.mockRejectedValueOnce(new Error('github: 422'))
    api.SyncPendingReview.mockResolvedValueOnce(pending(draft('main.go')))

    submit()?.click()
    await flushPromises()
    expect(api.SubmitReview).toHaveBeenCalledWith(expect.objectContaining({
      submodules: [{ path: 'lib', pr: 'o/lib#3', to_pr: 1, folded: 0, mode: 'review', verdict: 'COMMENT', body: '' }],
    }))
    // Read after the reload: the submodule drafts all went, so there is nothing left to look up.
    expect(api.SubmoduleDrafts).toHaveBeenCalledTimes(1)
    expect(toastError).toHaveBeenCalledWith('github: 422', { description: expect.stringContaining('already posted') })
    expect(document.body.textContent).toContain('1 pending comment')
    expect(document.body.textContent).not.toContain('Comments on submodule files')
    wrapper.unmount()
  })

  it('labels a draft on a submodule pointer row apart from one on the file at its path', async () => {
    api.SyncPendingReview.mockResolvedValue(pending({ ...draft('vendor'), pointer: true }, draft('vendor')))
    const { wrapper } = await openDialog()
    const rows = [...document.body.querySelectorAll('[role="dialog"] ul li')].map((li) => li.textContent ?? '')
    expect(rows.map((r) => r.includes('(submodule pointer)'))).toEqual([true, false])
    wrapper.unmount()
  })

  it('scrolls the pending comments with the dialog body, not in a box of their own', async () => {
    const { wrapper } = await openDialog()
    const scrollers = [...document.body.querySelectorAll('[role="dialog"] .overflow-y-auto')]
    expect(scrollers).toHaveLength(1)
    expect(scrollers[0].querySelector('ul')?.textContent).toContain('main.go')
    wrapper.unmount()
  })

  it('lays out the text box under the verdict, then the submodule choices, then the pending comments', async () => {
    const { wrapper } = await openDialog()
    const parts = document.body.querySelectorAll('[role="dialog"] :is(textarea[aria-label="Review body"], [data-submodule="lib"], ul)')
    expect([...parts].map((el) => el.tagName === 'TEXTAREA' ? 'text' : el.tagName === 'UL' ? 'pending' : 'submodule')).toEqual(['text', 'submodule', 'pending'])
    wrapper.unmount()
  })

  it('holds the verdict and text an unsettled earlier submit sent, read-only', async () => {
    api.SyncPendingReview.mockResolvedValue({ ...pending(draft('main.go')), in_flight_review: { verdict: 'APPROVE', body: 'sent' } })
    const { wrapper } = await openDialog()
    const body = document.body.querySelector<HTMLTextAreaElement>('textarea[aria-label="Review body"]')
    expect([body?.value, body?.disabled]).toEqual(['sent', true])
    const radios = [...document.body.querySelectorAll<HTMLInputElement>('input[name="verdict"]')]
    expect(radios.filter((r) => r.checked).map((r) => r.value)).toEqual(['APPROVE'])
    expect(radios.every((r) => r.disabled)).toBe(true)
    expect(document.body.textContent).toContain('An earlier submit may have reached GitHub; submit again to settle it.')
    wrapper.unmount()
  })

  it('offers Discard for an unsettled earlier submit even with no drafts left', async () => {
    const discard = (w: ReturnType<typeof mountView>) => w.find('button[aria-label="Discard pending review"]').element.closest('[inert]')
    api.SyncPendingReview.mockResolvedValue({ review_id: 0, comments: [], has_pending: false, head_sha: '' })
    const idle = mountView()
    await flushPromises()
    expect(discard(idle)).not.toBeNull()
    idle.unmount()

    api.SyncPendingReview.mockResolvedValue({ review_id: 0, comments: [], has_pending: false, head_sha: '', in_flight_review: { verdict: 'APPROVE', body: '' } })
    const wrapper = mountView()
    await flushPromises()
    expect(discard(wrapper)).toBeNull()
    wrapper.unmount()
  })

  it('keeps Submit off with a reason when the review would reach GitHub empty', async () => {
    api.SyncPendingReview.mockResolvedValue(pending({ ...draft('lib/a.c'), submodule: 'lib' }))
    const { wrapper, submit } = await openDialog()
    expect(submit()?.disabled).toBe(false)

    document.body.querySelector<HTMLInputElement>('input[name="verdict"][value="REQUEST_CHANGES"]')?.click()
    await flushPromises()
    expect(submit()?.disabled).toBe(true)
    expect(document.body.querySelector('[data-slot="dialog-footer"]')?.textContent).toContain('every comment goes to a submodule PR')

    const body = document.body.querySelector<HTMLTextAreaElement>('textarea[aria-label="Review body"]')
    if (body) body.value = 'Please fix'
    body?.dispatchEvent(new Event('input'))
    await flushPromises()
    expect(submit()?.disabled).toBe(false)
    expect(document.body.textContent).not.toContain('every comment goes to a submodule PR')
    wrapper.unmount()
  })

  it('reloads the files once a retried lookup finds the submodule PR the file list could not', async () => {
    api.GetPRFiles.mockResolvedValue([{ filename: 'lib', status: 'submodule', submodule: 'lib', submodule_pr_unknown: true, additions: 0, deletions: 0, viewed: false }])
    const { wrapper } = await openDialog()
    expect(api.GetPRFiles).toHaveBeenCalledTimes(2)

    api.SubmoduleDrafts.mockResolvedValueOnce([{ ...subDrafts[0], pr: undefined, pr_unknown: true }])
    await wrapper.findAll('button').find((b) => b.text().includes('Finish review'))?.trigger('click')
    await flushPromises()
    expect(api.GetPRFiles).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('reloads the files once the drafts find a submodule PR opened after the file list loaded', async () => {
    api.GetPRFiles.mockResolvedValue([{ filename: 'lib', status: 'submodule', submodule: 'lib', additions: 0, deletions: 0, viewed: false }])
    const { wrapper } = await openDialog()
    expect(api.GetPRFiles).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('reloads the files once a recheck finds the submodule PR retargeted, its Viewed routing moved', async () => {
    api.GetPRFiles.mockResolvedValue([{ filename: 'lib', status: 'submodule', submodule: 'lib', submodule_pr: 'o/lib#3', additions: 0, deletions: 0, viewed: false }])
    api.SubmoduleDrafts.mockResolvedValue([{ ...subDrafts[0], rerouted: true }])
    const { wrapper } = await openDialog()
    expect(api.GetPRFiles).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('reloads the files once a retried lookup confirms the submodule has no PR', async () => {
    api.GetPRFiles.mockResolvedValue([{ filename: 'lib', status: 'submodule', submodule: 'lib', submodule_pr_unknown: true, additions: 0, deletions: 0, viewed: false }])
    api.SubmoduleDrafts.mockResolvedValue([{ path: 'lib', to_pr: 0, folded: 1 }])
    const { wrapper } = await openDialog()
    expect(api.GetPRFiles).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
})

describe('DiffReviewView Viewed label', () => {
  const p = { filename: 'lib', status: 'submodule', submodule: 'lib', submodule_pr: 'o/lib#3', additions: 0, deletions: 0, viewed: false }
  const file = (filename: string, extra: Partial<model.PRFileDTO> = {}) => ({ filename, repo: 'lib', submodule: 'lib', status: 'modified', additions: 1, deletions: 0, viewed: false, ...extra })

  beforeEach(() => {
    vi.clearAllMocks()
    api.SyncPendingReview.mockResolvedValue(pending())
    api.GetFileDiff.mockResolvedValue({ hunks: [] })
    // The pointer is viewed, so the view opens on the submodule's file.
    api.GetViewedFiles.mockResolvedValue({ files: [{ path: 'lib', pointer: true }] })
  })

  it('says gitura only for a file whose Viewed mark the backend keeps locally, not for one in the submodule PR', async () => {
    api.GetPRFiles.mockResolvedValue([file('lib/out.c', { viewed_local: true }), p])
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('Viewed (gitura only)')
    expect(wrapper.find('[title="Kept in gitura only: this file is outside o/lib#3"]').exists()).toBe(true)
    wrapper.unmount()

    api.GetPRFiles.mockResolvedValue([file('lib/in.c'), p])
    const inPR = mountView()
    await flushPromises()
    expect(inPR.text()).not.toContain('gitura only')
    expect(inPR.text()).toContain('Comments can go to o/lib#3 when you submit; Viewed syncs there')
    inPR.unmount()
  })
})

describe('DiffReviewView files sharing a path', () => {
  it('shows each file only the drafts written on it', async () => {
    vi.clearAllMocks()
    const own = { filename: 'lib/a.txt', status: 'removed', additions: 0, deletions: 1, viewed: false }
    const inner = { filename: 'lib/a.txt', repo: 'lib', submodule: 'lib', status: 'added', additions: 1, deletions: 0, viewed: false }
    api.GetPRFiles.mockResolvedValue([own, { filename: 'lib', status: 'submodule', submodule: 'lib', additions: 1, deletions: 1, viewed: false }, inner])
    api.GetViewedFiles.mockResolvedValue({ files: [] })
    api.GetFileDiff.mockResolvedValue({ hunks: [] })
    api.SyncPendingReview.mockResolvedValue(pending({ ...draft('lib/a.txt'), body: 'own' }, { ...draft('lib/a.txt'), submodule: 'lib', body: 'inner' }))
    const wrapper = mountView()
    await flushPromises()
    const shownDrafts = () => (wrapper.findComponent(FileViewStub).vm.$attrs.drafts as model.DraftCommentDTO[]).map((d) => d.body)
    expect(shownDrafts()).toEqual(['own'])

    wrapper.findComponent(DiffFileSidebar).vm.$emit('select', fileKey(inner))
    await flushPromises()
    expect(api.GetFileDiff).toHaveBeenLastCalledWith('lib', 'lib/a.txt', false)
    expect(shownDrafts()).toEqual(['inner'])
    wrapper.unmount()
  })
})

describe('DiffReviewView pending review on an earlier head', () => {
  const main = { filename: 'main.go', status: 'modified', additions: 1, deletions: 0, viewed: false }
  const inner = { filename: 'lib/a.c', repo: 'lib', submodule: 'lib', status: 'modified', additions: 1, deletions: 0, viewed: false }

  beforeEach(() => {
    vi.clearAllMocks()
    api.GetPRFiles.mockResolvedValue([main, { filename: 'lib', status: 'submodule', submodule: 'lib', additions: 0, deletions: 0, viewed: false }, inner])
    api.GetViewedFiles.mockResolvedValue({ files: [] })
    api.GetFileDiff.mockResolvedValue({ hunks: [] })
    api.SyncPendingReview.mockResolvedValue({ ...pending(draft('main.go'), subDraft('lib/a.c')), head_sha: 'old' })
  })

  it('marks only the PR\'s own files stale, leaving submodule drafts to their range', async () => {
    const wrapper = mountView('new')
    await flushPromises()
    const stale = () => wrapper.findComponent(FileViewStub).vm.$attrs['drafts-stale']
    expect(stale()).toBe(true)

    wrapper.findComponent(DiffFileSidebar).vm.$emit('select', fileKey(inner))
    await flushPromises()
    expect(stale()).toBe(false)
    wrapper.unmount()
  })

  it('reloads the files after a comment is posted now', async () => {
    api.PostImmediateComment.mockResolvedValue({ html_url: '' })
    const wrapper = mountView('new')
    await flushPromises()
    const before = api.GetPRFiles.mock.calls.length
    const post = wrapper.findComponent(FileViewStub).vm.$attrs['post-comment'] as (c: model.DraftCommentDTO, m: 'immediate') => Promise<boolean>
    expect(await post(subDraft('lib/a.c'), 'immediate')).toBe(true)
    await flushPromises()
    expect(api.GetPRFiles).toHaveBeenCalledTimes(before + 1)
    wrapper.unmount()
  })
})

describe('DiffReviewView nested submodule pointer', () => {
  const lib = { filename: 'lib', status: 'submodule', submodule: 'lib', submodule_range: 'aaaaaaa..bbbbbbb', additions: 0, deletions: 0, viewed: false }
  const vendor = { filename: 'lib/vendor', repo: 'lib', status: 'submodule', submodule: 'lib/vendor', submodule_range: 'ccccccc..ddddddd', additions: 0, deletions: 0, viewed: false }

  beforeEach(() => {
    vi.clearAllMocks()
    api.GetPRFiles.mockResolvedValue([lib, vendor])
    api.GetViewedFiles.mockResolvedValue({ files: [] })
    api.GetFileDiff.mockResolvedValue({ hunks: [] })
    api.SubmoduleDrafts.mockResolvedValue([])
    api.SyncPendingReview.mockResolvedValue(pending({ ...draft('lib/vendor'), submodule: 'lib', pointer: true }))
  })

  it('keeps forms pinned to the head and range the shown diff was loaded at until a refresh reloads it', async () => {
    const wrapper = mountView('old')
    await flushPromises()
    wrapper.findComponent(DiffFileSidebar).vm.$emit('select', fileKey(vendor))
    await flushPromises()
    const pin = () => [wrapper.findComponent(FileViewStub).vm.$attrs['head-sha'], wrapper.findComponent(FileViewStub).vm.$attrs['submodule-range']]
    expect(pin()).toEqual(['old', 'aaaaaaa..bbbbbbb'])

    let diff!: (d: object) => void
    api.GetFileDiff.mockImplementationOnce(() => new Promise((resolve) => (diff = resolve)))
    api.GetPRFiles.mockResolvedValue([{ ...lib, submodule_range: 'aaaaaaa..eeeeeee' }, vendor])
    await wrapper.setProps({ headSha: 'new' })
    const reloading = wrapper.vm.reload()
    await flushPromises()
    expect(pin()).toEqual(['old', 'aaaaaaa..bbbbbbb'])

    diff({ hunks: [] })
    await reloading
    await flushPromises()
    expect(pin()).toEqual(['new', 'aaaaaaa..eeeeeee'])
    wrapper.unmount()
  })

  it('pins comments on the pointer row to the range of the submodule holding it', async () => {
    const wrapper = mountView()
    await flushPromises()
    wrapper.findComponent(DiffFileSidebar).vm.$emit('select', fileKey(vendor))
    await flushPromises()
    expect(wrapper.findComponent(FileViewStub).vm.$attrs['submodule-range']).toBe('aaaaaaa..bbbbbbb')
    wrapper.unmount()
  })

  async function openVendor(extra: Parameters<typeof mountView>[1] = {}) {
    const wrapper = mountView('', extra)
    await flushPromises()
    wrapper.findComponent(DiffFileSidebar).vm.$emit('select', fileKey(vendor))
    await flushPromises()
    return wrapper
  }

  it('explains a Viewed mark kept in gitura by the PR of the submodule holding the pointer', async () => {
    api.GetPRFiles.mockResolvedValue([{ ...lib, submodule_pr: 'o/lib#3' }, { ...vendor, viewed_local: true, submodule_pr_unknown: true }])
    const wrapper = await openVendor()
    expect(wrapper.find('[title="Kept in gitura only: this file is outside o/lib#3"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('shows the parent PR\'s threads on a top-level pointer row but not on a nested one', async () => {
    const thread = (path: string): model.CommentThreadDTO => ({ root_id: 1, review_id: 1, node_id: '', comments: [], resolved: false, path, line: 1, side: 'RIGHT', outdated: false })
    const wrapper = await openVendor({ threads: [thread('lib'), thread('lib/vendor')] })
    const shownThreads = () => (wrapper.findComponent(FileViewStub).vm.$attrs.threads as model.CommentThreadDTO[]).map((t) => t.path)
    expect(shownThreads()).toEqual([])

    wrapper.findComponent(DiffFileSidebar).vm.$emit('select', fileKey(lib))
    await flushPromises()
    expect(shownThreads()).toEqual(['lib'])
    wrapper.unmount()
  })

  it("counts the parent PR's unresolved threads only on rows of the parent repository", async () => {
    const thread = (path: string): model.CommentThreadDTO => ({ root_id: 1, review_id: 1, node_id: '', comments: [], resolved: false, path, line: 1, side: 'RIGHT', outdated: false })
    const wrapper = mountView('', { threads: [thread('lib'), thread('lib/vendor')] })
    await flushPromises()
    const counts = wrapper.findComponent(DiffFileSidebar).props('commentCounts') as Record<string, number>
    expect([lib, vendor].map((f) => counts[fileKey(f)] ?? 0)).toEqual([1, 0])
    wrapper.unmount()
  })

  it('lists a draft on the pointer row as still in the PR', async () => {
    const { wrapper } = await openDialog()
    expect(document.body.querySelector('[role="dialog"] ul')?.textContent).toContain('lib/vendor')
    expect(document.body.textContent).not.toContain('file no longer in this PR')
    wrapper.unmount()
  })
})
