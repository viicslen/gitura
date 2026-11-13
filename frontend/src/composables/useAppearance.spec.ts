import { describe, it, expect, beforeEach, vi } from 'vitest'
import { nextTick } from 'vue'

const backend = vi.hoisted(() => ({
  GetAppearance: vi.fn(),
  SetAppearance: vi.fn(),
  ListSystemFonts: vi.fn(),
}))
vi.mock('@/bindings/gitura/internal/app/app', () => backend)
vi.mock('vue-sonner', () => ({ toast: { error: vi.fn() } }))

async function load() {
  vi.resetModules()
  return import('./useAppearance')
}

const root = document.documentElement

describe('useAppearance', () => {
  beforeEach(() => {
    root.removeAttribute('style')
    root.classList.remove('dark', 'light')
    localStorage.clear()
    backend.GetAppearance.mockReset()
    backend.SetAppearance.mockReset().mockResolvedValue(undefined)
  })

  it('leaves style.css defaults in place when nothing is saved', async () => {
    backend.GetAppearance.mockResolvedValue({ text_font: '', code_font: '', text_size: '' })
    const { applyAppearance, loadAppearance, useAppearance } = await load()
    applyAppearance()
    await loadAppearance()
    await nextTick()
    expect(useAppearance().appearance).toEqual({ text_font: 'inter', code_font: 'jetbrains', text_size: 'default', theme: 'auto' })
    expect(root.style.getPropertyValue('--font-sans')).toBe('')
    expect(root.style.getPropertyValue('--font-mono')).toBe('')
    expect(root.style.fontSize).toBe('')
  })

  it('applies saved choices and ignores unknown values', async () => {
    backend.GetAppearance.mockResolvedValue({ text_font: 'atkinson', code_font: 'comic-mono', text_size: 'large' })
    const { applyAppearance, loadAppearance, useAppearance } = await load()
    applyAppearance()
    await loadAppearance()
    await nextTick()
    expect(useAppearance().appearance.code_font).toBe('jetbrains')
    expect(root.style.getPropertyValue('--font-sans')).toContain('Atkinson Hyperlegible Next Variable')
    expect(root.style.fontSize).toBe('112.5%')
  })

  it('applies a saved installed font and lists it even when no longer installed', async () => {
    backend.GetAppearance.mockResolvedValue({ text_font: 'font:Fira Sans', code_font: 'font:', text_size: '' })
    backend.ListSystemFonts.mockResolvedValue(['Noto Sans'])
    const { applyAppearance, loadAppearance, loadSystemFonts, systemFontOptions, useAppearance } = await load()
    applyAppearance()
    await loadAppearance()
    await loadSystemFonts()
    await nextTick()
    expect(useAppearance().appearance.code_font).toBe('jetbrains')
    expect(root.style.getPropertyValue('--font-sans')).toBe('"Fira Sans", ui-sans-serif, system-ui, sans-serif')
    expect(systemFontOptions('text_font').map((o) => o.label)).toEqual(['Fira Sans', 'Noto Sans'])
    expect(systemFontOptions('text_size')).toEqual([])
  })

  it('keeps defaults when loading fails', async () => {
    backend.GetAppearance.mockRejectedValue(new Error('settings: load: bad toml'))
    const { loadAppearance, useAppearance } = await load()
    await expect(loadAppearance()).resolves.toBeUndefined()
    expect(useAppearance().appearance.text_size).toBe('default')
  })

  it('saves the full set and switches back to defaults', async () => {
    const { applyAppearance, setAppearance, useAppearance } = await load()
    applyAppearance()
    await setAppearance('code_font', 'system')
    await nextTick()
    expect(backend.SetAppearance).toHaveBeenLastCalledWith({ text_font: 'inter', code_font: 'system', text_size: 'default', theme: 'auto' })
    expect(root.style.getPropertyValue('--font-mono')).toBe('ui-monospace, monospace')

    await setAppearance('code_font', 'jetbrains')
    await nextTick()
    expect(useAppearance().appearance.code_font).toBe('jetbrains')
    expect(root.style.getPropertyValue('--font-mono')).toBe('')
  })

  it('ignores changes while a save is in flight', async () => {
    let finish = () => {}
    backend.SetAppearance.mockReturnValueOnce(new Promise<void>((resolve) => { finish = resolve }))
    const { setAppearance, useAppearance } = await load()
    const { appearance } = useAppearance()
    const first = setAppearance('text_font', 'atkinson')
    await setAppearance('text_size', 'large')
    finish()
    await first
    expect(backend.SetAppearance).toHaveBeenCalledTimes(1)
    expect(appearance).toEqual({ text_font: 'atkinson', code_font: 'jetbrains', text_size: 'default', theme: 'auto' })
    await setAppearance('text_size', 'large')
    expect(backend.SetAppearance).toHaveBeenCalledTimes(2)
  })

  it('carries a theme kept only in browser storage over to settings once', async () => {
    localStorage.setItem('gitura-color-mode', 'dark')
    backend.GetAppearance.mockResolvedValue({ text_font: '', code_font: '', text_size: '', theme: '' })
    const { applyAppearance, loadAppearance, useAppearance } = await load()
    applyAppearance()
    await loadAppearance()
    await nextTick()
    expect(useAppearance().appearance.theme).toBe('dark')
    expect(backend.SetAppearance).toHaveBeenCalledWith(expect.objectContaining({ theme: 'dark' }))
    expect(root.classList.contains('dark')).toBe(true)
  })

  it('applies the saved theme over the one in browser storage', async () => {
    localStorage.setItem('gitura-color-mode', 'dark')
    backend.GetAppearance.mockResolvedValue({ text_font: '', code_font: '', text_size: '', theme: 'light' })
    const { applyAppearance, loadAppearance } = await load()
    applyAppearance()
    await loadAppearance()
    await nextTick()
    expect(backend.SetAppearance).not.toHaveBeenCalled()
    expect(root.classList.contains('dark')).toBe(false)
  })

  it('reverts and reports when saving fails', async () => {
    backend.SetAppearance.mockRejectedValue(new Error('settings: save: disk full'))
    const { setAppearance, useAppearance } = await load()
    const { toast } = await import('vue-sonner')
    await setAppearance('text_size', 'larger')
    expect(useAppearance().appearance.text_size).toBe('default')
    expect(toast.error).toHaveBeenCalled()
  })
})
