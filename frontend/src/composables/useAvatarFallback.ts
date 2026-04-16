import { reactive } from 'vue'

type AvatarState = {
  retried: boolean
  failed: boolean
  retryURL: string
}

/**
 * useAvatarFallback handles one retry for failed avatar loads and then marks
 * the avatar as failed so callers can render a placeholder.
 */
export function useAvatarFallback() {
  const states = reactive<Record<string, AvatarState>>({})

  function ensureState(key: string): AvatarState {
    if (!states[key]) {
      states[key] = { retried: false, failed: false, retryURL: '' }
    }
    return states[key]
  }

  function withRetryQuery(url: string): string {
    const stamp = Date.now().toString()
    try {
      const parsed = new URL(url)
      parsed.searchParams.set('retry', stamp)
      return parsed.toString()
    } catch {
      const separator = url.includes('?') ? '&' : '?'
      return `${url}${separator}retry=${stamp}`
    }
  }

  function avatarSrc(key: string, originalURL: string): string {
    if (!key || !originalURL) return ''
    const state = ensureState(key)
    if (state.failed) return ''
    if (state.retried && state.retryURL) return state.retryURL
    return originalURL
  }

  function handleAvatarError(key: string, originalURL: string): void {
    if (!key || !originalURL) return
    const state = ensureState(key)
    if (!state.retried) {
      state.retried = true
      state.retryURL = withRetryQuery(originalURL)
      return
    }
    state.failed = true
  }

  function avatarInitial(login: string): string {
    const trimmed = login.trim()
    if (!trimmed) return '?'
    return trimmed[0].toUpperCase()
  }

  return {
    avatarSrc,
    handleAvatarError,
    avatarInitial,
  }
}
