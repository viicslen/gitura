import { describe, it, expect, vi } from 'vitest'
import { nextTick } from 'vue'

const GetCommentThreads = vi.fn()
vi.mock('@/wailsjs/go/app/App', () => ({
  LoadPR: vi.fn().mockResolvedValue({ number: 1 }),
  GetCommentThreads: (...a: unknown[]) => GetCommentThreads(...a),
}))
vi.mock('@/wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))

const { useCommentThreads } = await import('./useCommentThreads')

const thread = (root_id: number, resolved = false) => ({ root_id, resolved })

describe('useCommentThreads refresh', () => {
  it('keeps the filter and the open thread when the PR is loaded again', async () => {
    GetCommentThreads.mockResolvedValue([thread(1), thread(2, true), thread(3), thread(4)])
    const review = useCommentThreads({ owner: 'o', repo: 'r', number: 1 } as never)
    await review.loadPR()
    review.toggleShowResolved()
    review.goNext(); review.goNext()
    expect(review.currentThread.value?.root_id).toBe(3)

    GetCommentThreads.mockResolvedValue([thread(0), thread(1), thread(2, true), thread(3), thread(4)])
    await review.loadPR()
    await nextTick()

    expect(review.showResolved.value).toBe(true)
    expect(review.currentThread.value?.root_id).toBe(3)
  })
})
