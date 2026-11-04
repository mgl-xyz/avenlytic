<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

export type SelectValue = string | number
export type SelectOption = { value: SelectValue; label: string }

const props = withDefaults(defineProps<{
  modelValue: SelectValue | SelectValue[]
  options: SelectOption[]
  multiple?: boolean
  placeholder?: string
}>(), { multiple: false, placeholder: '' })
const emit = defineEmits<{ (event: 'update:modelValue', value: SelectValue | SelectValue[]): void }>()
const root = ref<HTMLElement | null>(null)
const trigger = ref<HTMLElement | null>(null)
const menu = ref<HTMLElement | null>(null)
const open = ref(false)
const activeIndex = ref(0)
const menuStyle = ref<Record<string, string>>({})
const selectedValues = computed<SelectValue[]>(() => Array.isArray(props.modelValue) ? props.modelValue : [props.modelValue])
const selectedOptions = computed(() => props.options.filter(option => selectedValues.value.includes(option.value)))
const displayValue = computed(() => selectedOptions.value.map(option => option.label).join(', ') || props.placeholder)

function closeOnOutside(event: PointerEvent) {
  if (event.target instanceof Node && !root.value?.contains(event.target) && !menu.value?.contains(event.target)) open.value = false
}

function updateMenuPosition() {
  const triggerRect = trigger.value?.getBoundingClientRect()
  if (!triggerRect) return

  const below = window.innerHeight - triggerRect.bottom
  const above = triggerRect.top
  const opensUp = below < 180 && above > below
  const available = Math.max(80, (opensUp ? above : below) - 18)
  const maxHeight = Math.min(250, available)
  menuStyle.value = {
    left: `${triggerRect.left}px`,
    width: `${triggerRect.width}px`,
    maxHeight: `${maxHeight}px`,
    ...(opensUp
      ? { bottom: `${window.innerHeight - triggerRect.top + 6}px` }
      : { top: `${triggerRect.bottom + 6}px` }),
  }
}

function toggle() {
  open.value = !open.value
  if (open.value) {
    activeIndex.value = Math.max(0, props.options.findIndex(option => selectedValues.value.includes(option.value)))
    nextTick(updateMenuPosition)
  }
}

function choose(index: number) {
  const option = props.options[index]
  if (!option) return
  if (props.multiple) {
    const next = selectedValues.value.includes(option.value)
      ? selectedValues.value.filter(value => value !== option.value)
      : [...selectedValues.value, option.value]
    emit('update:modelValue', next)
  } else {
    emit('update:modelValue', option.value)
    open.value = false
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!props.options.length) return
    if (!open.value) open.value = true
    const direction = event.key === 'ArrowDown' ? 1 : -1
    activeIndex.value = (activeIndex.value + direction + props.options.length) % props.options.length
  } else if (event.key === 'Home' || event.key === 'End') {
    event.preventDefault()
    activeIndex.value = event.key === 'Home' ? 0 : Math.max(0, props.options.length - 1)
  } else if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    if (open.value) choose(activeIndex.value)
    else toggle()
  } else if (event.key === 'Escape' && open.value) {
    event.preventDefault()
    open.value = false
  }
}

onMounted(() => {
  document.addEventListener('pointerdown', closeOnOutside)
  document.addEventListener('scroll', updateMenuPosition, true)
  window.addEventListener('resize', updateMenuPosition)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', closeOnOutside)
  document.removeEventListener('scroll', updateMenuPosition, true)
  window.removeEventListener('resize', updateMenuPosition)
})
</script>

<template>
  <div ref="root" class="select-field" @keydown="onKeydown">
    <button
      type="button"
      ref="trigger"
      class="select-trigger"
      role="combobox"
      aria-haspopup="listbox"
      :aria-expanded="open"
      :aria-multiselectable="multiple"
      @click="toggle"
    >
      <span class="select-value" :class="{ 'select-placeholder': !selectedOptions.length }">{{ displayValue }}</span>
      <span class="select-chevron" :class="{ 'is-open': open }" aria-hidden="true"></span>
    </button>
    <Teleport to="body">
      <Transition name="select-menu">
        <div v-if="open" ref="menu" class="select-menu select-menu--floating" :style="menuStyle" role="listbox" :aria-multiselectable="multiple">
          <button
            v-for="(option, index) in options"
            :key="option.value"
            type="button"
            role="option"
            class="select-option"
            :class="{ 'is-active': activeIndex === index, 'is-selected': selectedValues.includes(option.value) }"
            :aria-selected="selectedValues.includes(option.value)"
            @mouseenter="activeIndex = index"
            @click="choose(index)"
          >
            <span>{{ option.label }}</span>
            <span v-if="selectedValues.includes(option.value)" class="select-check" aria-hidden="true">✓</span>
          </button>
          <div v-if="!options.length" class="select-empty">—</div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
