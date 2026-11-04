<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref } from 'vue'
import type { ChannelForm } from '../types'
import ModalDialog from './ModalDialog.vue'
import DropdownSelect from './DropdownSelect.vue'
const { t } = useI18n()
const { showChannel, channelForm, channelTypes, channelFields, saveChannel, testChannel } = defineProps<{ showChannel: boolean; channelForm: ChannelForm; channelTypes: string[][]; channelFields: (type: string) => string[][]; saveChannel: () => void; testChannel: () => void }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const errors = ref<Record<string, string>>({})
function submit() {
  errors.value = {}
  if (!channelForm.name.trim()) errors.value.name = '请填写通道名称'
  if (Object.keys(errors.value).length) return
  saveChannel()
}
</script>
<template>
<ModalDialog :open="showChannel" :label="channelForm.id ? t('编辑通知通道') : t('添加通知通道')" @close="emit('close')">
  <div class="modal-head">
    <div><h2>{{channelForm.id ? t('编辑') : t('添加')}}{{t('通知通道')}}</h2><p>{{t('密钥保存在中心端状态文件中。')}}</p></div>
    <button type="button" class="close" :aria-label="t('关闭')" @click="emit('close')">×</button>
  </div>
  <form novalidate @submit.prevent="submit">
    <label>{{t('通道名称')}}<input v-model="channelForm.name" :aria-invalid="!!errors.name" @input="errors.name = ''"><small v-if="errors.name" class="field-error">{{t(errors.name)}}</small></label>
    <label>{{t('通道类型')}}<DropdownSelect v-model="channelForm.type" :options="channelTypes.map(([value, label]) => ({ value, label: t(label) }))" /></label>
    <label v-for="field in channelFields(channelForm.type)" :key="field[0]">
      {{t(field[1])}}
      <input
        v-model="channelForm.settings[field[0]]"
        :type="field[0].includes('password') || ['secret', 'bot_token', 'authorization'].includes(field[0]) ? 'password' : 'text'"
        :placeholder="channelForm.id && channelForm.settings[field[0]] === undefined ? t('留空则保留已保存的值') : ''"
      >
    </label>
    <label class="check"><input v-model="channelForm.enabled" type="checkbox">{{t('启用通道')}}</label>
    <div class="modal-actions">
      <button type="button" class="ghost" @click="testChannel">{{t('测试发送')}}</button>
      <button type="button" class="ghost" @click="emit('close')">{{t('取消')}}</button>
      <button class="primary">{{t('保存通道')}}</button>
    </div>
  </form>
</ModalDialog>
</template>
