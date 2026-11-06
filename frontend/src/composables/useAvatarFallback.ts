import { reactive } from 'vue'

type AvatarState = {
  retried: boolean
  failed: boolean
  retryURL: string
}

const AVATAR_PROXY_PATH = '/__avatar'

/**
 * buildAvatarProxyURL rewrites a GitHub avatar URL to an in-app route so the
 * Wails webview fetches avatars from the app origin.
 */
export function buildAvatarProxyURL(originalURL: string): string {
  const trimmed = originalURL.trim()
  if (!trimmed) return ''

  try {
    const url = new URL(trimmed)
    if (url.protocol !== 'https:') return ''
    const host = url.hostname.toLowerCase()
    if (host !== 'avatars.githubusercontent.com' && !host.endsWith('.githubusercontent.com')) {
      return ''
    }

    const proxied = new URL(AVATAR_PROXY_PATH, 'http://localhost')
    proxied.searchParams.set('url', url.toString())
    return `${proxied.pathname}${proxied.search}`
  } catch {
    return ''
  }
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
    return buildAvatarProxyURL(originalURL)
  }

  function handleAvatarError(key: string, originalURL: string): void {
    if (!key || !originalURL) return
    const state = ensureState(key)
    if (!state.retried) {
      const proxied = buildAvatarProxyURL(originalURL)
      if (!proxied) {
        state.failed = true
        return
      }
      state.retried = true
      state.retryURL = withRetryQuery(proxied)
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
