import { reactive, readonly, ref, watchEffect } from 'vue'
import { toast } from 'vue-sonner'
import { GetAppearance, SetAppearance } from '@/wailsjs/go/main/App'
import type { model } from '@/wailsjs/go/models'

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
}

const defaults: model.AppearanceDTO = { text_font: 'inter', code_font: 'jetbrains', text_size: 'default' }

const state = reactive<model.AppearanceDTO>({ ...defaults })
/** One save at a time, since each sends the whole set; guarded here because disabling the selects drops keyboard focus. */
const saving = ref(false)

function option(key: AppearanceKey, value: string): Option | undefined {
  return appearanceOptions[key].find((o) => o.value === value)
}

/** Keeps the root element's fonts and text size in sync with the current settings. */
export function applyAppearance(root: HTMLElement = document.documentElement) {
  watchEffect(() => {
    root.style.setProperty('--font-sans', option('text_font', state.text_font)?.css ?? '')
    root.style.setProperty('--font-mono', option('code_font', state.code_font)?.css ?? '')
    // Tailwind sizes are rem-based, so this scales text and spacing together.
    root.style.fontSize = option('text_size', state.text_size)?.css ?? ''
  })
}

/** Reads the saved settings; unknown or missing values (settings.toml is hand-editable) fall back to defaults. */
export async function loadAppearance() {
  try {
    const saved = await GetAppearance()
    for (const key of Object.keys(defaults) as AppearanceKey[]) {
      state[key] = option(key, saved[key]) ? saved[key] : defaults[key]
    }
  } catch {
    // Keep defaults rather than block startup on a settings read failure.
  }
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
    toast.error('Could not save appearance: ' + String(e))
  } finally {
    saving.value = false
  }
}

export function useAppearance() {
  return { appearance: readonly(state) }
}
