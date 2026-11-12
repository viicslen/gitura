import { describe, it, expect, vi, beforeEach } from 'vitest'
import { CancellablePromise } from '@wailsio/runtime'
import * as model from '@/bindings/gitura/internal/model'
import * as App from '@/bindings/gitura/internal/app/app'

const SetFileViewed = vi.fn()
vi.mock('@/bindings/gitura/internal/app/app', () => ({
  SetFileViewed: (...a: unknown[]) => SetFileViewed(...a),
  GetFileDiff: vi.fn().mockResolvedValue({}),
  GetPRFiles: vi.fn(),
  GetViewedFiles: vi.fn(),
}))

const { useDiffReview } = await import('./useDiffReview')

const diffNamed = (filename: string) => model.ParsedDiffDTO.createFrom({ filename, hunks: [] })

function withFile() {
  const review = useDiffReview()
  review.files.value = [model.PRFileDTO.createFrom({ filename: 'a.go', viewed: false })]
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

describe('useDiffReview.setViewedMany', () => {
  beforeEach(() => SetFileViewed.mockReset())

  it('saves one file at a time, skips files already in that state, and rolls back the unsaved rest on failure', async () => {
    const review = useDiffReview()
    review.files.value = ['a.go', 'b.go', 'c.go', 'd.go'].map((filename) => model.PRFileDTO.createFrom({ filename, viewed: filename === 'b.go' }))
    let inFlight = 0
    SetFileViewed.mockImplementation(async (path: string) => {
      expect(++inFlight).toBe(1)
      await Promise.resolve()
      inFlight--
      if (path === 'c.go') throw new Error('403')
    })
    await expect(review.setViewedMany(['a.go', 'b.go', 'c.go', 'd.go'], true)).rejects.toThrow('403')
    expect(SetFileViewed.mock.calls.map((c) => c[0])).toEqual(['a.go', 'c.go'])
    expect(review.files.value.map((f) => f.viewed)).toEqual([true, true, false, false])
    expect(review.viewedSaving.value.size).toBe(0)
  })
})

describe('useDiffReview file filters', () => {
  it('counts unviewed files, leaving out test files only while they are hidden', () => {
    localStorage.clear()
    const review = useDiffReview()
    review.files.value = [
      model.PRFileDTO.createFrom({ filename: 'a.go', viewed: false }),
      model.PRFileDTO.createFrom({ filename: 'a_test.go', viewed: false }),
      model.PRFileDTO.createFrom({ filename: 'b.go', viewed: true }),
    ]
    expect(review.unviewedCount.value).toBe(2)
    review.hideTests.value = true
    expect(review.unviewedCount.value).toBe(1)
    review.viewedError.value = 'rate limited'
    expect(review.unviewedCount.value).toBe(0)
  })

  it('moves between shown files from wherever the open file sits, numbering against all files', async () => {
    localStorage.clear()
    const review = useDiffReview()
    review.files.value = ['a.go', 'a_test.go', 'b.go', 'c.go', 'd.go'].map(
      (filename) => model.PRFileDTO.createFrom({ filename, viewed: filename === 'b.go' }),
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
    let resolveSlow!: (v: model.ParsedDiffDTO) => void
    vi.mocked(App.GetFileDiff).mockImplementationOnce(() => new CancellablePromise((r) => { resolveSlow = r }))
    vi.mocked(App.GetFileDiff).mockResolvedValueOnce(diffNamed('fast'))
    const review = useDiffReview()
    const slow = review.selectFile('a.go')
    await review.selectFile('b.go')
    resolveSlow(diffNamed('slow'))
    await slow
    expect(review.currentFilePath.value).toBe('b.go')
    expect(review.currentDiff.value?.filename).toBe('fast')
  })
})

describe('useDiffReview refetch', () => {
  it('keeps the open diff on screen while it is fetched again', async () => {
    vi.mocked(App.GetFileDiff).mockResolvedValueOnce(diffNamed('old'))
    const review = useDiffReview()
    await review.selectFile('a.go')
    let resolve!: (v: model.ParsedDiffDTO) => void
    vi.mocked(App.GetFileDiff).mockImplementationOnce(() => new CancellablePromise((r) => { resolve = r }))
    const refetch = review.selectFile('a.go', true)
    expect(review.diffLoading.value).toBe(false)
    expect(review.currentDiff.value?.filename).toBe('old')
    resolve(diffNamed('new'))
    await refetch
    expect(review.currentDiff.value?.filename).toBe('new')
  })
})

describe('useDiffReview start file', () => {
  it('opens the first shown file that has not been viewed yet', async () => {
    localStorage.clear()
    vi.mocked(App.GetPRFiles).mockResolvedValueOnce(['a.go', 'b_test.go', 'c.go', 'd.go'].map((filename) => model.PRFileDTO.createFrom({ filename, viewed: false })))
    vi.mocked(App.GetViewedFiles).mockResolvedValueOnce(['a.go'])
    const review = useDiffReview()
    review.hideTests.value = true
    await review.loadFiles()
    expect(review.currentFilePath.value).toBe('c.go')
  })
})
