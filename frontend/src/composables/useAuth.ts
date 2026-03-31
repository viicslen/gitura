import { ref, readonly } from 'vue'
import * as App from '../wailsjs/go/main/App'
import { EventsOn, BrowserOpenURL } from '../wailsjs/runtime/runtime'
import type { model } from '../wailsjs/go/models'

// Shared reactive state (module-level singleton).
const authState = ref<model.AuthState>({ is_authenticated: false, login: '', avatar_url: '' })
const deviceFlowInfo = ref<model.DeviceFlowInfo | null>(null)
const polling = ref(false)
const error = ref<string | null>(null)

let pollTimer: ReturnType<typeof setInterval> | null = null
let pollIntervalMs = 5000

function stopPolling() {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  polling.value = false
}

function restartPolling() {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
  }
  pollTimer = setInterval(() => {
    void pollDeviceFlow()
  }, pollIntervalMs)
}

async function refreshAuthState() {
  try {
    authState.value = await App.GetAuthState()
  } catch (e) {
    error.value = String(e)
  }
}

/** Poll once for a device-flow token. Rescheduled automatically on slow_down. */
async function pollDeviceFlow() {
  try {
    const result = await App.PollDeviceFlow()

    if (result.status === 'complete') {
      stopPolling()
      deviceFlowInfo.value = null
      await refreshAuthState()
    } else if (result.status === 'expired' || result.status === 'error') {
      stopPolling()
      error.value = result.error ?? `Device flow ${result.status}`
    } else if (result.interval) {
      // GitHub sent slow_down: increase interval permanently and reschedule.
      pollIntervalMs += result.interval * 1000
      restartPolling()
    }
    // plain 'pending' → keep existing timer
  } catch (e) {
    stopPolling()
    error.value = String(e)
  }
}

/**
 * useAuth provides auth-related state and actions backed by Wails bindings.
 */
export function useAuth() {
  /** Start GitHub OAuth device flow and begin polling automatically. */
  async function startDeviceFlow() {
    error.value = null
    deviceFlowInfo.value = null
    pollIntervalMs = 5000
    stopPolling()

    try {
      const info = await App.StartDeviceFlow()
      deviceFlowInfo.value = info

      // Open the verification URL in the user's default browser.
      BrowserOpenURL(info.verification_uri)

      // Begin polling at the interval GitHub specifies.
      pollIntervalMs = (info.interval || 5) * 1000
      polling.value = true

      restartPolling()
    } catch (e) {
      error.value = String(e)
    }
  }

  /** Sign out: remove token from keyring. */
  async function logout() {
    error.value = null
    try {
      await App.Logout()
      authState.value = { is_authenticated: false, login: '', avatar_url: '' }
      deviceFlowInfo.value = null
      stopPolling()
    } catch (e) {
      error.value = String(e)
    }
  }

  return {
    authState: readonly(authState),
    deviceFlowInfo: readonly(deviceFlowInfo),
    polling: readonly(polling),
    error: readonly(error),
    refreshAuthState,
    startDeviceFlow,
    pollDeviceFlow,
    logout,
  }
}

/** Subscribe to auth:device-flow-complete at the module level (once). */
EventsOn('auth:device-flow-complete', async () => {
  const info = await App.GetAuthState().catch(
    () => ({ is_authenticated: false, login: '', avatar_url: '' } as model.AuthState),
  )
  authState.value = info
  stopPolling()
  deviceFlowInfo.value = null
})

/**
 * Subscribe to auth:device-flow-expired at the module level (once).
 * Stops polling and surfaces an expired message so the UI can prompt
 * the user to restart the device flow.
 */
EventsOn('auth:device-flow-expired', () => {
  stopPolling()
  deviceFlowInfo.value = null
  error.value =
    'auth:device-flow-expired — the authorization code expired. Please start the sign-in flow again.'
})
