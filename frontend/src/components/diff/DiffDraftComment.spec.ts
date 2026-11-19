import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'

const run = vi.fn(async () => [])
vi.mock('@/bindings/gitura/internal/app/app', () => ({
  RunCommands: (...a: unknown[]) => run(...(a as [])),
  SetToggles: vi.fn(async () => {}),
}))

import DiffDraftComment, { type DraftActions } from './DiffDraftComment.vue'
import { commands } from '@/composables/useCommands'
import { setToggle } from '@/composables/useToggles'
import type * as model from '@/bindings/gitura/internal/model'

const draft = { body: 'Rename this', path: 'a.go', line: 3, side: 'RIGHT' } as model.DraftCommentDTO

function mountDraft() {
  const actions = { indexOf: () => 0, update: vi.fn(), remove: vi.fn(async () => {}), ran: vi.fn() } satisfies DraftActions
  const w = mount(DiffDraftComment, {
    props: { draft },
    global: { provide: { draftActions: actions, viewerLogin: ref('dev') } },
  })
  return { w, actions }
}

async function runIt(w: ReturnType<typeof mountDraft>['w']) {
  await w.find('button[aria-busy]').trigger('click')
  await flushPromises()
}

describe('DiffDraftComment', () => {
  beforeEach(() => {
    commands.value = [{ name: 'fix', command: 'echo' } as model.CommandDTO]
    run.mockClear()
  })

  it('runs the pending comment and keeps it by default', async () => {
    await setToggle('remove_draft_after_run', false)
    const { w, actions } = mountDraft()
    await runIt(w)
    expect(run).toHaveBeenCalledWith(['fix'], 'Rename this', expect.anything())
    expect(actions.ran).toHaveBeenCalled()
    expect(actions.remove).not.toHaveBeenCalled()
  })

  it('removes the pending comment after the run starts when the setting is on', async () => {
    await setToggle('remove_draft_after_run', true)
    const { w, actions } = mountDraft()
    await runIt(w)
    expect(actions.remove).toHaveBeenCalledWith(draft)
  })
})
