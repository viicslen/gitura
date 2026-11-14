import { afterEach, describe, expect, it, vi } from 'vitest'
import { morph, morphing } from './morph'

const flag = () => document.documentElement.dataset.vt

describe('morph', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    Reflect.deleteProperty(document, 'startViewTransition')
  })

  it('makes the change at once and sets no flag where there are no view transitions', () => {
    const change = vi.fn()
    void morph('page', change)
    expect(change).toHaveBeenCalledOnce()
    expect(morphing.value).toBeNull()
    expect(flag()).toBeUndefined()
  })

  it('flags the running transition and leaves a newer one its flags when the older one settles', async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    Object.defineProperty(document, 'startViewTransition', {
      configurable: true,
      value: (update: () => Promise<void>) => ({ finished: update() }),
    })
    const releases: (() => void)[] = []
    const hang = () => new Promise<void>((release) => releases.push(release))

    const older = morph('page', hang)
    expect([morphing.value, flag()]).toEqual(['page', 'page'])
    const newer = morph('signin', hang)
    releases[0]()
    await older
    expect([morphing.value, flag()]).toEqual(['signin', 'signin'])

    releases[1]()
    await newer
    expect([morphing.value, flag()]).toEqual([null, undefined])
  })
})
