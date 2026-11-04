<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  open: boolean
  label: string
  size?: 'default' | 'small' | 'wide'
}>(), {
  size: 'default',
})

const emit = defineEmits<{ (event: 'close'): void }>()
const dialog = ref<HTMLElement | null>(null)
let previouslyFocused: HTMLElement | null = null

watch(() => props.open, async open => {
  if (open) {
    previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null
    await nextTick()
    dialog.value?.focus()
    return
  }

  previouslyFocused?.focus()
  previouslyFocused = null
})
</script>

<template>
  <div v-if="open" class="modal-backdrop" @click.self="emit('close')">
    <section
      ref="dialog"
      class="modal card"
      :class="`modal--${size}`"
      role="dialog"
      aria-modal="true"
      :aria-label="label"
      tabindex="-1"
      @keydown.esc.stop.prevent="emit('close')"
    >
      <slot />
    </section>
  </div>
</template>
