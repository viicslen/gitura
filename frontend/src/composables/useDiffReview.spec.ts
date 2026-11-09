import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { model } from '@/wailsjs/go/models'

const SetFileViewed = vi.fn()
vi.mock('../wailsjs/go/main/App', () => ({
  SetFileViewed: (...a: unknown[]) => SetFileViewed(...a),
  GetFileDiff: vi.fn().mockResolvedValue({}),
  GetPRFiles: vi.fn(),
  GetViewedFiles: vi.fn(),
}))

const { useDiffReview } = await import('./useDiffReview')

function withFile() {
  const review = useDiffReview()
  review.files.value = [{ filename: 'a.go', viewed: false } as model.PRFileDTO]
  return review
}

describe('useDiffReview.setViewed', () => {
  beforeEach(() => SetFileViewed.mockReset())

  it('marks the file viewed on GitHub', async () => {
    SetFileViewed.mockResolvedValueOnce(undefined)
    const review = withFile()
    await review.setViewed('a.go', true)
    expect(SetFileViewed).toHaveBeenCalledWith('a.go', true)
    expect(review.files.value[0].viewed).toBe(true)
  })

  it('ignores another toggle of the same file until GitHub answers', async () => {
    let finish!: () => void
    SetFileViewed.mockImplementationOnce(() => new Promise<void>((r) => { finish = r }))
    const review = withFile()
    const first = review.setViewed('a.go', true)
    expect(review.viewedSaving.value.has('a.go')).toBe(true)
    await review.setViewed('a.go', false)
    expect(SetFileViewed).toHaveBeenCalledTimes(1)
    finish()
    await first
    expect(review.viewedSaving.value.has('a.go')).toBe(false)
    expect(review.files.value[0].viewed).toBe(true)
  })

  it('rolls back when GitHub rejects it', async () => {
    SetFileViewed.mockRejectedValueOnce(new Error('403'))
    const review = withFile()
    await expect(review.setViewed('a.go', true)).rejects.toThrow('403')
    expect(review.files.value[0].viewed).toBe(false)
  })
})

describe('useDiffReview file filters', () => {
  it('moves between shown files from wherever the open file sits, numbering against all files', async () => {
    localStorage.clear()
    const review = useDiffReview()
    review.files.value = ['a.go', 'a_test.go', 'b.go', 'c.go', 'd.go'].map(
      (filename) => ({ filename, viewed: filename === 'b.go' }) as model.PRFileDTO,
    )
    review.hideTests.value = true
    review.hideViewed.value = true
    await review.selectFile('a.go')
    expect(review.visibleFiles.value.map((f) => f.filename)).toEqual(['a.go', 'c.go', 'd.go'])

    await review.nextFile()
    expect(review.currentFilePath.value).toBe('c.go')
    expect(review.currentFileIndex.value).toBe(3)

    // The open file gets marked viewed: it leaves the list, but moving still works both ways.
    review.files.value[3].viewed = true
    expect(review.visibleFiles.value.map((f) => f.filename)).toEqual(['a.go', 'd.go'])
    expect(review.canGoPrev.value).toBe(true)
    await review.nextFile()
    expect(review.currentFilePath.value).toBe('d.go')
    await review.prevFile()
    expect(review.currentFilePath.value).toBe('a.go')
  })

  it('keeps only the latest diff when files are switched quickly', async () => {
    const { GetFileDiff } = await import('../wailsjs/go/main/App') as unknown as { GetFileDiff: ReturnType<typeof vi.fn> }
    let resolveSlow!: (v: unknown) => void
    GetFileDiff.mockImplementationOnce(() => new Promise((r) => { resolveSlow = r }))
    GetFileDiff.mockResolvedValueOnce({ hunks: ['fast'] })
    const review = useDiffReview()
    const slow = review.selectFile('a.go')
    await review.selectFile('b.go')
    resolveSlow({ hunks: ['slow'] })
    await slow
    expect(review.currentFilePath.value).toBe('b.go')
    expect(review.currentDiff.value).toEqual({ hunks: ['fast'] })
  })
})

describe('useDiffReview refetch', () => {
  it('keeps the open diff on screen while it is fetched again', async () => {
    const { GetFileDiff } = await import('../wailsjs/go/main/App') as unknown as { GetFileDiff: ReturnType<typeof vi.fn> }
    GetFileDiff.mockResolvedValueOnce({ hunks: ['old'] })
    const review = useDiffReview()
    await review.selectFile('a.go')
    let resolve!: (v: unknown) => void
    GetFileDiff.mockImplementationOnce(() => new Promise((r) => { resolve = r }))
    const refetch = review.selectFile('a.go', true)
    expect(review.diffLoading.value).toBe(false)
    expect(review.currentDiff.value).toEqual({ hunks: ['old'] })
    resolve({ hunks: ['new'] })
    await refetch
    expect(review.currentDiff.value).toEqual({ hunks: ['new'] })
  })
})

describe('useDiffReview start file', () => {
  it('opens the first shown file that has not been viewed yet', async () => {
    localStorage.clear()
    const { GetPRFiles, GetViewedFiles } = await import('../wailsjs/go/main/App') as unknown as Record<string, ReturnType<typeof vi.fn>>
    GetPRFiles.mockResolvedValueOnce(['a.go', 'b_test.go', 'c.go', 'd.go'].map((filename) => ({ filename, viewed: false })))
    GetViewedFiles.mockResolvedValueOnce(['a.go'])
    const review = useDiffReview()
    review.hideTests.value = true
    await review.loadFiles()
    expect(review.currentFilePath.value).toBe('c.go')
  })
})
