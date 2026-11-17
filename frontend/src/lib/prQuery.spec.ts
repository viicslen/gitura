import { describe, expect, it } from 'vitest'
import { parsePRNumber } from './prQuery'

describe('parsePRNumber', () => {
  it.each([
    ['123', { number: 123 }],
    [' #42 ', { number: 42 }],
    ['acme/web.app#7', { owner: 'acme', repo: 'web.app', number: 7 }],
    ['https://github.com/acme/web/pull/9/files', { owner: 'acme', repo: 'web', number: 9 }],
    ['github.com/acme/web/pull/9', { owner: 'acme', repo: 'web', number: 9 }],
  ])('reads %s', (text, want) => {
    expect(parsePRNumber(text)).toEqual(want)
  })

  it.each(['fix login', '12a', 'acme#1', 'https://github.com/acme/web/issues/9', ''])('ignores %s', (text) => {
    expect(parsePRNumber(text)).toBeNull()
  })
})
