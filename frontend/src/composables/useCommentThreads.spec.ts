import { describe, it, expect, vi } from 'vitest'
import { nextTick } from 'vue'

const GetCommentThreads = vi.fn()
const LoadPR = vi.fn().mockResolvedValue({ number: 1 })
vi.mock('@/bindings/gitura/internal/app/app', () => ({
  LoadPR: (...a: unknown[]) => LoadPR(...a),
  GetCommentThreads: (...a: unknown[]) => GetCommentThreads(...a),
}))
vi.mock('@wailsio/runtime', () => ({ Events: { On: () => () => {} } }))

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

  it('keeps what is loaded when a refresh fails, and shows the error only when nothing is loaded yet', async () => {
    GetCommentThreads.mockResolvedValue([thread(1), thread(2)])
    const review = useCommentThreads({ owner: 'o', repo: 'r', number: 1 } as never)
    LoadPR.mockRejectedValueOnce('github: 502')
    expect(await review.loadPR()).toBe(false)
    expect(review.error.value).toBe('github: 502')

    expect(await review.loadPR()).toBe(true)
    expect(review.error.value).toBe('')

    LoadPR.mockRejectedValueOnce('github: 502')
    expect(await review.loadPR()).toBe(false)
    expect(review.error.value).toBe('')
    expect(review.threads.value).toHaveLength(2)
  })
})

describe('useCommentThreads filter', () => {
  it('keeps the open thread open when the other filter lists it too', async () => {
    GetCommentThreads.mockResolvedValue([thread(1, true), thread(2), thread(3, true), thread(4)])
    const review = useCommentThreads({ owner: 'o', repo: 'r', number: 1 } as never)
    await review.loadPR()
    review.goNext()
    expect(review.currentThread.value?.root_id).toBe(4)

    review.toggleShowResolved()
    expect(review.currentThread.value?.root_id).toBe(4)
    review.goPrev()
    expect(review.currentThread.value?.root_id).toBe(3)

    review.toggleShowResolved()
    await nextTick()
    expect(review.currentThread.value?.root_id).toBe(4)
  })
})
