import { onUnmounted, ref, watch, type Ref } from 'vue'

const monitoringViews = new Set(['overview', 'hosts', 'containers', 'alerts'])

type LiveUpdatesOptions = {
  active: Ref<boolean>
  view: Ref<string>
  refreshInterval: Ref<number>
  refresh: (queueIfBusy: boolean, monitoringOnly: boolean) => Promise<void>
  onMonitoring: (event: Event) => void
  onAuthExpired: () => void
}

export function useLiveUpdates({
  active,
  view,
  refreshInterval,
  refresh,
  onMonitoring,
  onAuthExpired,
}: LiveUpdatesOptions) {
  const connected = ref(false)
  let eventSource: EventSource | undefined
  let refreshTimer: number | undefined

  function stop() {
    eventSource?.close()
    eventSource = undefined
    connected.value = false
  }

  function start() {
    stop()
    if (!active.value) return

    eventSource = new EventSource('/api/v1/events')
    eventSource.onopen = () => {
      connected.value = true
    }
    eventSource.onerror = () => {
      connected.value = false
    }
    eventSource.addEventListener('monitoring', onMonitoring)
    eventSource.addEventListener('auth-expired', () => {
      stop()
      onAuthExpired()
    })
  }

  function resetRefreshTimer() {
    if (refreshTimer !== undefined) window.clearInterval(refreshTimer)
    refreshTimer = undefined

    if (!active.value || connected.value || refreshInterval.value <= 0 || !monitoringViews.has(view.value)) return

    refreshTimer = window.setInterval(() => {
      if (document.visibilityState === 'visible') void refresh(false, true)
    }, refreshInterval.value * 1000)
  }

  watch(active, isActive => {
    if (isActive) start()
    else stop()
    resetRefreshTimer()
  })
  watch([view, connected, refreshInterval], resetRefreshTimer)
  onUnmounted(() => {
    stop()
    if (refreshTimer !== undefined) window.clearInterval(refreshTimer)
  })

  return { connected }
}
