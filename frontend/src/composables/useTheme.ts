import { computed } from 'vue'
import { setAppearance, useAppearance } from '@/composables/useAppearance'

export type ThemeMode = 'auto' | 'light' | 'dark'

/** The system / light / dark theme, saved in settings.toml as appearance.theme. */
export function useTheme() {
  const { appearance } = useAppearance()
  const mode = computed<ThemeMode>({
    get: () => (appearance.theme === 'light' || appearance.theme === 'dark' ? appearance.theme : 'auto'),
    set: (value) => void setAppearance('theme', value),
  })

  /** Cycle through: system → light → dark → system */
  function cycleTheme() {
    mode.value = mode.value === 'auto' ? 'light' : mode.value === 'light' ? 'dark' : 'auto'
  }

  return { mode, cycleTheme }
}
