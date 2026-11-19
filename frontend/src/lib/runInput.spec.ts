import { describe, it, expect } from 'vitest'
import type * as model from '@/bindings/gitura/internal/model'
import { prInput, reviewInput, threadInput } from './runInput'

const thread = (review_id: number, path: string, ...bodies: string[]) =>
  ({ review_id, path, line: 4, comments: bodies.map((body, i) => ({ author_login: `u${i}`, body })) }) as unknown as model.CommentThreadDTO

const review = { review_id: 1, author_login: 'rev', body: 'Looks off' } as model.ReviewDTO

describe('run input', () => {
  it('puts each thread comment under its author, after the location', () => {
    expect(threadInput(thread(1, 'a.go', 'Fix this', ' Done '))).toBe('a.go:4\n\n@u0:\nFix this\n\n---\n\n@u1:\nDone')
  })

  it('takes only the threads the review opened, and skips an empty review body', () => {
    const threads = [thread(1, 'a.go', 'x'), thread(2, 'b.go', 'y')]
    expect(reviewInput(review, threads)).toBe('@rev:\nLooks off\n\n---\n\na.go:4\n\n@u0:\nx')
    expect(reviewInput({ ...review, body: '' } as model.ReviewDTO, threads)).toBe('a.go:4\n\n@u0:\nx')
  })

  it('covers the description, general comments, reviews and every thread', () => {
    const overview = {
      summary: { title: 'Move OAuth', body: '' },
      general_comments: [{ author_login: 'g', body: 'Nice' }],
      reviews: [review],
    } as unknown as model.PROverviewDTO
    expect(prInput(overview, [thread(2, 'b.go', 'y')])).toBe(
      '# Move OAuth\n\n---\n\n@g:\nNice\n\n---\n\n@rev:\nLooks off\n\n---\n\nb.go:4\n\n@u0:\ny',
    )
  })
})
