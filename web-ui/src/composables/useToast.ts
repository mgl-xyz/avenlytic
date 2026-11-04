import { onUnmounted, ref } from 'vue'
import { i18n } from '../i18n'

export type ToastLevel = 'success' | 'error' | 'warning' | 'info'

const toastDuration = 12_000

export function useToast() {
  const message = ref('')
  const level = ref<ToastLevel>('info')
  let timer: number | undefined

  function stopTimer() {
    if (timer !== undefined) window.clearTimeout(timer)
    timer = undefined
  }

  function startTimer() {
    stopTimer()
    timer = window.setTimeout(() => {
      message.value = ''
      timer = undefined
    }, toastDuration)
  }

  function notify(text: string, nextLevel: ToastLevel = 'info') {
    message.value = i18n.global.t(text)
    level.value = nextLevel
    startTimer()
  }

  function clear() {
    stopTimer()
    message.value = ''
  }

  function resume() {
    if (message.value) startTimer()
  }

  onUnmounted(stopTimer)

  return {
    message,
    level,
    notify,
    pause: stopTimer,
    resume,
    clear,
  }
}
