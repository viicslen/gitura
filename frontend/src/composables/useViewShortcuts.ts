import { toValue, type MaybeRefOrGetter } from 'vue'
import { useEventListener } from '@vueuse/core'

/**
 * Window-level shortcuts for a view, so they work wherever focus is (e.g. still on the side-rail
 * button that opened the view). Ignored while typing, with any modifier held, or under an open dialog or menu.
 */
export function useViewShortcuts(active: MaybeRefOrGetter<boolean>, keys: Record<string, () => void>): void {
  useEventListener(window, 'keydown', (event: KeyboardEvent) => {
    if (!toValue(active) || event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return
    const target = event.target instanceof Element ? event.target : null
    if (target?.closest('input, textarea, select, [contenteditable=""], [contenteditable="true"], [role="dialog"], [role="menu"]')) return
    keys[event.key]?.()
  })
}
