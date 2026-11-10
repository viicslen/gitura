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
