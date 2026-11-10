import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import MarkdownBody from './MarkdownBody.vue'

type DocumentEventName = Parameters<Document['addEventListener']>[0]

const countClickEvents = (calls: unknown[][]): number => {
  let clickCount = 0

  for (const call of calls) {
    const [eventName] = call as [DocumentEventName, ...unknown[]]
    if (eventName === 'click') {
      clickCount += 1
    }
  }

  return clickCount
}

describe('MarkdownBody', () => {
  let addSpy: ReturnType<typeof vi.spyOn>
  let removeSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    addSpy = vi.spyOn(document, 'addEventListener')
    removeSpy = vi.spyOn(document, 'removeEventListener')
  })

  afterEach(() => {
    addSpy.mockRestore()
    removeSpy.mockRestore()
  })

  it('registers exactly one document click listener and removes it on unmount', async () => {
    const wrapper = mount(MarkdownBody, {
      props: {
        content: '```ts\nconsole.log("a")\n```\n\n```ts\nconsole.log("b")\n```',
        runCommands: [
          { name: 'cmd-a', command: 'cmd-a {{instructions}}' },
          { name: 'cmd-b', command: 'cmd-b {{instructions}}' },
        ],
        defaultCommandName: 'cmd-a',
      },
    })

    await wrapper.vm.$nextTick()

    expect(countClickEvents(addSpy.mock.calls as unknown[][])).toBe(1)

    await wrapper.setProps({
      content: '```ts\nconsole.log("c")\n```\n\n```ts\nconsole.log("d")\n```',
    })
    await wrapper.vm.$nextTick()

    expect(countClickEvents(addSpy.mock.calls as unknown[][])).toBe(1)

    wrapper.unmount()

    expect(countClickEvents(removeSpy.mock.calls as unknown[][])).toBe(1)
  })
})
