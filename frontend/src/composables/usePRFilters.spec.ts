import { afterEach, describe, expect, it } from 'vitest'
import { usePRFilters } from './usePRFilters'
import type { model } from '@/wailsjs/go/models'

function pr(owner: string, repo: string, number = 1): model.PRListItem {
  return {
    number,
    title: `${owner}/${repo}`,
    owner,
    repo,
    author_login: 'me',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    html_url: `https://github.com/${owner}/${repo}/pull/${number}`,
    is_draft: false,
    is_author: true,
    is_assignee: false,
    is_reviewer: false,
  }
}

const items = [pr('FmTod', 'app'), pr('FmTod', 'app-two'), pr('FmTod2', 'config')]

describe('usePRFilters.applyFilters', () => {
  const filters = usePRFilters(() => {})
  afterEach(() => filters.clearAllFilters())

  it('matches a picked org exactly, not orgs sharing its prefix', () => {
    filters.pickOrg('FmTod')
    expect(filters.applyFilters(items).map((i) => i.owner)).toEqual(['FmTod', 'FmTod'])
  })

  it('keeps a picked org exact when it is absent from the results', () => {
    filters.pickOrg('FmTod')
    expect(filters.applyFilters([pr('FmTod2', 'config')])).toEqual([])
  })

  it('matches a picked repo exactly', () => {
    filters.pickRepo('FmTod/app')
    expect(filters.applyFilters(items).map((i) => i.repo)).toEqual(['app'])
  })

  it('substring-matches typed text', () => {
    filters.org.value = 'fmtod'
    expect(filters.applyFilters(items)).toHaveLength(3)
    filters.clearOrg()
    filters.repo.value = 'app'
    expect(filters.applyFilters(items).map((i) => i.repo)).toEqual(['app', 'app-two'])
  })
})
