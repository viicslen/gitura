import { ref, readonly } from 'vue'
import * as App from '@/bindings/gitura/internal/app/app'
import { Browser, Events } from '@wailsio/runtime'
import type * as model from '@/bindings/gitura/internal/model'
import { errorMessage } from '@/lib/utils'
import { morph } from '@/components/shell/morph'

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
    error.value = errorMessage(e)
  }
}

let entering = false

/**
 * Applies a finished sign-in as the view transition `signin`. The poll result and the backend's event both report it;
 * the second report finds the first under way or done and leaves it at that.
 */
async function enterApp() {
  stopPolling()
  if (entering || authState.value.is_authenticated) return
  entering = true
  try {
    const state = await App.GetAuthState()
    // One step, so the sign-in button never reappears between the two.
    const apply = () => {
      authState.value = state
      deviceFlowInfo.value = null
    }
    if (state.is_authenticated) await morph('signin', apply)
    else apply()
  } catch (e) {
    error.value = errorMessage(e)
    deviceFlowInfo.value = null
  } finally {
    entering = false
  }
}

/** Poll once for a device-flow token. Rescheduled automatically on slow_down. */
async function pollDeviceFlow() {
  try {
    const result = await App.PollDeviceFlow()

    if (result.status === 'complete') {
      await enterApp()
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
    error.value = errorMessage(e)
  }
}

/**
 * useAuth provides auth-related state and actions backed by Wails bindings.
 */
export function useAuth() {
  /** Start GitHub OAuth device flow and begin polling automatically. */
  async function startDeviceFlow() {
    deviceFlowInfo.value = null
    pollIntervalMs = 5000
    stopPolling()

    try {
      const info = await App.StartDeviceFlow()
      // Cleared only now, so a failed attempt's message stays up until the retry answers.
      error.value = null
      deviceFlowInfo.value = info

      // Open the verification URL in the user's default browser.
      void Browser.OpenURL(info.verification_uri)

      // Begin polling at the interval GitHub specifies.
      pollIntervalMs = (info.interval || 5) * 1000
      polling.value = true

      restartPolling()
    } catch (e) {
      error.value = errorMessage(e)
    }
  }

  /** Abandon the device flow and return to the sign-in prompt. */
  function cancelDeviceFlow() {
    stopPolling()
    deviceFlowInfo.value = null
    error.value = null
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
      error.value = errorMessage(e)
    }
  }

  return {
    authState: readonly(authState),
    deviceFlowInfo: readonly(deviceFlowInfo),
    polling: readonly(polling),
    error: readonly(error),
    refreshAuthState,
    startDeviceFlow,
    cancelDeviceFlow,
    pollDeviceFlow,
    logout,
  }
}

/** Subscribe to auth:device-flow-complete at the module level (once). */
Events.On('auth:device-flow-complete', enterApp)

/**
 * Subscribe to auth:device-flow-expired at the module level (once).
 * Stops polling and surfaces an expired message so the UI can prompt
 * the user to restart the device flow.
 */
Events.On('auth:device-flow-expired', () => {
  stopPolling()
  deviceFlowInfo.value = null
  error.value =
    'auth:device-flow-expired — the authorization code expired. Please start the sign-in flow again.'
})
