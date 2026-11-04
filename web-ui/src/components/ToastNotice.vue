<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ToastLevel } from '../composables/useToast'

const { t } = useI18n()
const { toast, toastLevel, pauseToast, resumeToast } = defineProps<{
  toast: string
  toastLevel: ToastLevel
  pauseToast: () => void
  resumeToast: () => void
}>()
const emit = defineEmits<{ (event: 'close'): void }>()
</script>

<template>
  <div
    v-if="toast"
    class="toast"
    :class="`toast--${toastLevel}`"
    :role="toastLevel === 'error' ? 'alert' : 'status'"
    :aria-live="toastLevel === 'error' ? 'assertive' : 'polite'"
    aria-atomic="true"
    @mouseenter="pauseToast"
    @mouseleave="resumeToast"
  >
    <span class="toast-icon" aria-hidden="true">{{toastLevel === 'success' ? '✓' : toastLevel === 'error' ? '×' : toastLevel === 'warning' ? '!' : 'i'}}</span>
    <span>{{toast}}</span>
    <button :aria-label="t('关闭提示')" @click="emit('close')">×</button>
  </div>
</template>
