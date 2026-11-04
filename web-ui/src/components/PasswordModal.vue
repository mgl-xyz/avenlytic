<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ModalDialog from './ModalDialog.vue'
const { t } = useI18n()
const { showPassword, passwordForm, changePassword } = defineProps<{ showPassword: boolean; passwordForm: Record<string, string>; changePassword: () => void }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const errors = ref<Record<string, string>>({})
function submit() {
  errors.value = {}
  if (!passwordForm.current) errors.value.current = '请填写当前密码'
  if (!passwordForm.password) errors.value.password = '请填写新密码'
  else if (Array.from(passwordForm.password).length < 6) errors.value.password = '新密码至少 6 位。'
  if (!passwordForm.confirm) errors.value.confirm = '请再次输入密码'
  else if (passwordForm.confirm !== passwordForm.password) errors.value.confirm = '两次输入的密码不一致'
  if (!Object.keys(errors.value).length) changePassword()
}
</script>
<template>
<ModalDialog :open="showPassword" :label="t('修改管理员密码')" size="small" @close="emit('close')">
  <div class="modal-head">
    <div><h2>{{t('修改管理员密码')}}</h2><p>{{t('新密码至少 6 位。')}}</p></div>
    <button type="button" class="close" :aria-label="t('关闭')" @click="emit('close')">×</button>
  </div>
  <form novalidate @submit.prevent="submit">
    <label>{{t('当前密码')}}<input v-model="passwordForm.current" type="password" :aria-invalid="!!errors.current" @input="errors.current = ''"><small v-if="errors.current" class="field-error">{{t(errors.current)}}</small></label>
    <label>{{t('新密码')}}<input v-model="passwordForm.password" type="password" minlength="6" :aria-invalid="!!errors.password" @input="errors.password = ''; errors.confirm = ''"><small v-if="errors.password" class="field-error">{{t(errors.password)}}</small></label>
    <label>{{t('确认新密码')}}<input v-model="passwordForm.confirm" type="password" minlength="6" :aria-invalid="!!errors.confirm" @input="errors.confirm = ''"><small v-if="errors.confirm" class="field-error">{{t(errors.confirm)}}</small></label>
    <div class="modal-actions"><button type="button" class="ghost" @click="emit('close')">{{t('取消')}}</button><button class="primary">{{t('更新密码')}}</button></div>
  </form>
</ModalDialog>
</template>
