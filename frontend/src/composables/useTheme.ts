import { useColorMode } from '@vueuse/core'

/**
 * useTheme wraps @vueuse/core's useColorMode to provide a three-way
 * system / light / dark toggle with localStorage persistence.
 *
 * The mode is stored in localStorage under the key 'gitura-color-mode'.
 * @vueuse/core applies the 'dark' class to <html> automatically.
 */
export function useTheme() {
  const mode = useColorMode({
    attribute: 'class',
    modes: { light: 'light', dark: 'dark' },
    storageKey: 'gitura-color-mode',
    emitAuto: true,
  })

  /** Cycle through: system → light → dark → system */
  function cycleTheme() {
    if (mode.value === 'auto') {
      mode.value = 'light'
    } else if (mode.value === 'light') {
      mode.value = 'dark'
    } else {
      mode.value = 'auto'
    }
  }

  return { mode, cycleTheme }
}
