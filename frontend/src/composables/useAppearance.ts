import { reactive, readonly, ref, watchEffect } from 'vue'
import { useColorMode } from '@vueuse/core'
import { toast } from 'vue-sonner'
import { GetAppearance, ListSystemFonts, SetAppearance } from '@/bindings/gitura/internal/app/app'
import type * as model from '@/bindings/gitura/internal/model'
import { errorMessage } from '@/lib/utils'

interface Option {
  value: string
  label: string
  /** CSS value to apply; '' keeps the default from style.css. */
  css: string
}

export type AppearanceKey = keyof model.AppearanceDTO

export const appearanceOptions: Record<AppearanceKey, Option[]> = {
  text_font: [
    { value: 'inter', label: 'Inter', css: '' },
    {
      value: 'atkinson',
      label: 'Atkinson Hyperlegible',
      css: '"Atkinson Hyperlegible Next Variable", ui-sans-serif, system-ui, sans-serif',
    },
    { value: 'system', label: 'System', css: 'ui-sans-serif, system-ui, sans-serif' },
  ],
  code_font: [
    { value: 'jetbrains', label: 'JetBrains Mono', css: '' },
    { value: 'system', label: 'System', css: 'ui-monospace, monospace' },
  ],
  text_size: [
    { value: 'small', label: 'Small', css: '93.75%' },
    { value: 'default', label: 'Default', css: '' },
    { value: 'large', label: 'Large', css: '112.5%' },
    { value: 'larger', label: 'Larger', css: '125%' },
  ],
  theme: [
    { value: 'auto', label: 'System', css: '' },
    { value: 'light', label: 'Light', css: '' },
    { value: 'dark', label: 'Dark', css: '' },
  ],
}

/** Installed fonts are saved as this prefix plus the family name. */
const FONT_PREFIX = 'font:'
const fontFallbacks: Partial<Record<AppearanceKey, string>> = {
  text_font: 'ui-sans-serif, system-ui, sans-serif',
  code_font: 'ui-monospace, monospace',
}
const systemFonts = ref<string[]>([])

function fontOption(key: AppearanceKey, family: string): Option {
  return { value: FONT_PREFIX + family, label: family, css: `${JSON.stringify(family)}, ${fontFallbacks[key]}` }
}

const defaults: model.AppearanceDTO = { text_font: 'inter', code_font: 'jetbrains', text_size: 'default', theme: 'auto' }

// Browser storage keeps the last theme so the first paint uses it before settings.toml is read.
const colorMode = useColorMode({
  attribute: 'class',
  modes: { light: 'light', dark: 'dark' },
  storageKey: 'gitura-color-mode',
  emitAuto: true,
})

const state = reactive<model.AppearanceDTO>({ ...defaults, theme: colorMode.value })
/** One save at a time, since each sends the whole set; guarded here because disabling the selects drops keyboard focus. */
const saving = ref(false)

function option(key: AppearanceKey, value: string): Option | undefined {
  const family = value.startsWith(FONT_PREFIX) ? value.slice(FONT_PREFIX.length) : ''
  if (family && fontFallbacks[key]) return fontOption(key, family)
  return appearanceOptions[key].find((o) => o.value === value)
}

/** Keeps the root element's fonts and text size in sync with the current settings. */
export function applyAppearance(root: HTMLElement = document.documentElement) {
  watchEffect(() => {
    root.style.setProperty('--font-sans', option('text_font', state.text_font)?.css ?? '')
    root.style.setProperty('--font-mono', option('code_font', state.code_font)?.css ?? '')
    // Tailwind sizes are rem-based, so this scales text and spacing together.
    root.style.fontSize = option('text_size', state.text_size)?.css ?? ''
    colorMode.value = state.theme === 'light' || state.theme === 'dark' ? state.theme : 'auto'
  })
}

/** Reads the saved settings; unknown or missing values (settings.toml is hand-editable) fall back to defaults. */
export async function loadAppearance() {
  try {
    const saved = await GetAppearance()
    // Before the theme lived in settings.toml it was only in browser storage; carry it over once.
    const migrateTheme = !saved.theme && state.theme !== defaults.theme
    for (const key of Object.keys(defaults) as AppearanceKey[]) {
      if (key === 'theme' && migrateTheme) continue
      state[key] = option(key, saved[key]) ? saved[key] : defaults[key]
    }
    if (migrateTheme) await SetAppearance({ ...state })
  } catch {
    // Keep defaults rather than block startup on a settings read failure.
  }
}

export async function loadSystemFonts() {
  if (systemFonts.value.length) return
  try {
    systemFonts.value = (await ListSystemFonts()) ?? []
  } catch {
    // The built-in fonts still work without the installed list.
  }
}

/** Installed families for a font setting, plus the saved one if it is no longer installed so the select can still show it. */
export function systemFontOptions(key: AppearanceKey): Option[] {
  if (!fontFallbacks[key]) return []
  const families = new Set(systemFonts.value)
  if (state[key].startsWith(FONT_PREFIX)) families.add(state[key].slice(FONT_PREFIX.length))
  return [...families].sort((a, b) => a.localeCompare(b)).map((f) => fontOption(key, f))
}

export async function setAppearance(key: AppearanceKey, value: string) {
  if (saving.value || !option(key, value)) return
  const previous = state[key]
  state[key] = value
  saving.value = true
  try {
    await SetAppearance({ ...state })
  } catch (e: unknown) {
    state[key] = previous
    toast.error('Could not save appearance: ' + errorMessage(e))
  } finally {
    saving.value = false
  }
}

export function useAppearance() {
  return { appearance: readonly(state) }
}
