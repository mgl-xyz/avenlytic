<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Rule, Channel } from '../types'
import ModalDialog from './ModalDialog.vue'
import DropdownSelect from './DropdownSelect.vue'
import { computed, ref, watch } from 'vue'
const { t } = useI18n()
const { showRule, ruleForm, channels, metricTypes, saveRule } = defineProps<{ showRule: boolean; ruleForm: Rule; channels: Channel[]; metricTypes: string[][]; saveRule: () => void }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const errors = ref<Record<string, string>>({})
const channelOptions = computed(() => channels.map(channel => ({ value: channel.id, label: channel.name })))
const operatorOptions = ['>', '>=', '<', '<=', '==', '!='].map(value => ({ value, label: value }))
const severityOptions = computed(() => [{ value: 'warning', label: t('警告') }, { value: 'critical', label: t('严重') }, { value: 'info', label: t('提示') }])
function defaultMessage(metric: string) {
  const label = t(metricTypes.find(([value]) => value === metric)?.[1] || metric)
  if (metric === 'agent.online') return t('{host} 已离线，告警条件已触发。')
  if (metric === 'security.login_failures_15m') return t('{host} 在最近 15 分钟内登录失败 {value} 次，已达到阈值 {operator} {threshold} 次。')
  if (metric === 'system.updates_available') return t('{host} 有 {value} 项系统更新待安装，已达到阈值 {operator} {threshold} 项。')
  if (metric === 'security.vulnerabilities') return t('{host} 检测到 {value} 个公开漏洞，已达到阈值 {operator} {threshold} 个。')
  if (metric === 'process.count') return t('{host} 当前有 {value} 个进程，已达到阈值 {operator} {threshold} 个。')
  if (metric.startsWith('load.')) return t('{host} 的{metric}为 {value}，已达到阈值 {operator} {threshold}。').replace('{metric}', label)
  if (metric.endsWith('usage_percent')) return t('{host} 的{metric}为 {value}%，已达到阈值 {operator} {threshold}%。').replace('{metric}', label)
  return t('{host} 的{metric}当前值为 {value}，已达到阈值 {operator} {threshold}。').replace('{metric}', label)
}
let generatedMessage = ''
watch(() => showRule, visible => {
  if (!visible) return
  generatedMessage = defaultMessage(ruleForm.metric)
  if (!ruleForm.message_template?.trim()) ruleForm.message_template = generatedMessage
})
watch(() => ruleForm.metric, metric => {
  const next = defaultMessage(metric)
  if (!ruleForm.message_template?.trim() || ruleForm.message_template === generatedMessage) {
    ruleForm.message_template = next
  }
  generatedMessage = next
})
function submit() {
  errors.value = {}
  if (!ruleForm.name.trim()) errors.value.name = '请填写规则名称'
  if (Number(ruleForm.duration_seconds) < 0) errors.value.duration = '持续时间不能小于 0'
  if (Object.keys(errors.value).length) return
  saveRule()
}
</script>
<template>
<ModalDialog :open="showRule" :label="t('告警规则')" @close="emit('close')">
  <div class="modal-head">
    <div><h2>{{ruleForm.id ? t('编辑') : t('新建')}}{{t('告警规则')}}</h2><p>{{t('选择指标、触发条件与通知方式。')}}</p></div>
    <button type="button" class="close" :aria-label="t('关闭')" @click="emit('close')">×</button>
  </div>
  <form novalidate @submit.prevent="submit">
    <label>{{t('规则名称')}}<input v-model="ruleForm.name" :aria-invalid="!!errors.name" :placeholder="t('例如：内存使用率过高')" @input="errors.name = ''"><small v-if="errors.name" class="field-error">{{t(errors.name)}}</small></label>
    <div class="form-grid">
      <label>{{t('监控指标')}}<DropdownSelect v-model="ruleForm.metric" :options="metricTypes.map(([value, label]) => ({ value, label: t(label) }))" /></label>
      <label>{{t('比较条件')}}<DropdownSelect v-model="ruleForm.operator" :options="operatorOptions" /></label>
      <label>{{t('阈值')}}<input v-model.number="ruleForm.threshold" type="number" step="0.1"></label>
      <label>{{t('持续时间（秒）')}}<input v-model.number="ruleForm.duration_seconds" type="number" min="0" :aria-invalid="!!errors.duration" @input="errors.duration = ''"><small v-if="errors.duration" class="field-error">{{t(errors.duration)}}</small></label>
      <label>{{t('告警级别')}}<DropdownSelect v-model="ruleForm.severity" :options="severityOptions" /></label>
      <label>{{t('通知通道')}}<DropdownSelect v-model="ruleForm.channel_ids" :options="channelOptions" multiple :placeholder="t('选择通知通道')" /></label>
    </div>
    <label>{{t('告警提示内容')}}<textarea v-model="ruleForm.message_template" maxlength="2000" rows="4" :placeholder="t('输入通知中展示的告警提示内容')"></textarea><small class="hint">{{t('可用变量：{host} 主机，{metric} 指标，{value} 当前值，{operator} 条件，{threshold} 阈值。')}}</small></label>
    <label class="check"><input v-model="ruleForm.enabled" type="checkbox">{{t('启用规则')}}</label>
    <label class="check"><input v-model="ruleForm.notify_on_recovery" type="checkbox">{{t('条件恢复时发送通知')}}</label>
    <div class="modal-actions"><button type="button" class="ghost" @click="emit('close')">{{t('取消')}}</button><button class="primary">{{t('保存规则')}}</button></div>
  </form>
</ModalDialog>
</template>
