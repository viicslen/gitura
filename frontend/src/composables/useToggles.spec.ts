import { beforeEach, describe, expect, it, vi } from 'vitest'

const backend = vi.hoisted(() => ({ GetToggles: vi.fn(), SetToggles: vi.fn() }))
vi.mock('@/bindings/gitura/internal/app/app', () => backend)
vi.mock('vue-sonner', () => ({ toast: { error: vi.fn() } }))

async function load() {
  vi.resetModules()
  return import('./useToggles')
}

describe('useToggles', () => {
  beforeEach(() => {
    localStorage.clear()
    backend.GetToggles.mockReset().mockResolvedValue({})
    backend.SetToggles.mockReset().mockResolvedValue(undefined)
  })

  it('carries values kept only in browser storage over to settings once', async () => {
    localStorage.setItem('gitura.diffWrapLines', 'false')
    localStorage.setItem('gitura.diffHideTests', 'true')
    backend.GetToggles.mockResolvedValue({ hide_tests: false })
    const { loadToggles, useToggles } = await load()
    await loadToggles()
    const { toggles } = useToggles()
    expect(toggles.value.wrap_lines).toBe(false)
    expect(toggles.value.hide_tests).toBe(false)
    expect(backend.SetToggles).toHaveBeenCalledWith(expect.objectContaining({ wrap_lines: false, hide_tests: false }))
    expect(localStorage.getItem('gitura.diffWrapLines')).toBeNull()
    expect(localStorage.getItem('gitura.diffHideTests')).toBeNull()
  })

  it('reverts and reports when saving fails', async () => {
    backend.SetToggles.mockRejectedValue(new Error('settings: save: disk full'))
    const { setToggle, useToggles } = await load()
    const { toast } = await import('vue-sonner')
    await setToggle('hide_viewed', true)
    expect(useToggles().toggles.value.hide_viewed).toBe(false)
    expect(toast.error).toHaveBeenCalled()
  })

  it('locks the last involvement filter on and toggles whose requirement is off', async () => {
    const { setToggle, toggleLock } = await load()
    expect(toggleLock('auto_view_tests')).not.toBe('')
    await setToggle('hide_tests', true)
    expect(toggleLock('auto_view_tests')).toBe('')

    await setToggle('pr_list_author', false)
    expect(toggleLock('pr_list_assignee')).toBe('')
    await setToggle('pr_list_assignee', false)
    expect(toggleLock('pr_list_reviewer')).not.toBe('')
    expect(toggleLock('pr_list_author')).toBe('')
  })

  it('seeds the PR list filters from the saved defaults, and clearing returns to them', async () => {
    backend.GetToggles.mockResolvedValue({ pr_list_author: false, pr_list_assignee: false, pr_list_drafts: true })
    const { loadToggles } = await load()
    await loadToggles()
    const { usePRFilters } = await import('./usePRFilters')
    const filters = usePRFilters(() => {})
    expect([filters.includeAuthor.value, filters.includeAssignee.value, filters.includeReviewer.value, filters.includeDrafts.value]).toEqual([false, false, true, true])
    expect(filters.hasActiveFilters.value).toBe(false)

    filters.toggleAuthor()
    expect(filters.hasActiveFilters.value).toBe(true)
    filters.clearAllFilters()
    expect(filters.includeAuthor.value).toBe(false)
  })
})
