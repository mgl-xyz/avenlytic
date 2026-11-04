<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Card from './Card.vue'
const { t } = useI18n()
const { initialized, busy, error, setup, login, submitSetup, submitLogin, theme, toggleTheme, locale, toggleLocale } = defineProps<{ initialized: boolean; busy: boolean; error: string; setup: Record<string, string>; login: Record<string, string>; submitSetup: () => void; submitLogin: () => void; theme: string; toggleTheme: () => void; locale: string; toggleLocale: () => void }>()
const errors = ref<Record<string, string>>({})
function handleSetup() {
  errors.value = {}
  if (!setup.username.trim()) errors.value.username = '请填写用户名'
  else if (setup.username.trim().length < 3) errors.value.username = '用户名至少需要 3 个字符'
  if (!setup.password) errors.value.password = '请填写密码'
  else if (Array.from(setup.password).length < 6) errors.value.password = '新密码至少 6 位。'
  if (!setup.confirm) errors.value.confirm = '请再次输入密码'
  else if (setup.confirm !== setup.password) errors.value.confirm = '两次输入的密码不一致'
  if (!Object.keys(errors.value).length) submitSetup()
}
function handleLogin() {
  errors.value = {}
  if (!login.username.trim()) errors.value.username = '请填写用户名'
  if (!login.password) errors.value.password = '请填写密码'
  if (!Object.keys(errors.value).length) submitLogin()
}
</script>
<template>
<div class="auth-shell"><div class="auth-brand"><div class="brand-mark">◉</div><div><b>AVENLYTIC</b><small>{{t("基础设施监控")}}</small></div></div><div class="appearance-controls"><button class="appearance-toggle" @click="toggleTheme" :aria-label="t(theme==='light'?'切换深色主题':'切换浅色主题')" :aria-pressed="theme==='dark'">{{theme==='light'?t('深色'):t('浅色')}}</button><button class="appearance-toggle" @click="toggleLocale">{{locale==='zh-CN'?'EN':'中文'}}</button></div><Card as="section" class="auth-card"><div class="eyebrow">{{ initialized?t('安全登录'):t('创建管理账户') }}</div><h1>{{ initialized?t("欢迎回来"):t("创建管理员账号") }}</h1><p>{{ initialized?t("登录以管理你的基础设施。"):t("完成初始化后即可通过浏览器管理所有监控配置。") }}</p><form v-if="!initialized" novalidate @submit.prevent="handleSetup"><label>{{t("管理员用户名")}}<input v-model="setup.username" :aria-invalid="!!errors.username" autocomplete="username" @input="errors.username=''"><small v-if="errors.username" class="field-error">{{t(errors.username)}}</small></label><label>{{t("设置密码")}}<input v-model="setup.password" type="password" minlength="6" :aria-invalid="!!errors.password" autocomplete="new-password" @input="errors.password='';errors.confirm=''"><small v-if="errors.password" class="field-error">{{t(errors.password)}}</small></label><label>{{t("确认密码")}}<input v-model="setup.confirm" type="password" minlength="6" :aria-invalid="!!errors.confirm" autocomplete="new-password" @input="errors.confirm=''"><small v-if="errors.confirm" class="field-error">{{t(errors.confirm)}}</small></label><button class="primary full" :disabled="busy">{{busy?t("正在初始化…"):t("创建管理员并进入系统")}}</button></form><form v-else novalidate @submit.prevent="handleLogin"><label>{{t("用户名")}}<input v-model="login.username" :aria-invalid="!!errors.username" autocomplete="username" @input="errors.username=''"><small v-if="errors.username" class="field-error">{{t(errors.username)}}</small></label><label>{{t("密码")}}<input v-model="login.password" type="password" :aria-invalid="!!errors.password" autocomplete="current-password" @input="errors.password=''"><small v-if="errors.password" class="field-error">{{t(errors.password)}}</small></label><button class="primary full" :disabled="busy">{{busy?t("正在登录…"):t("登录控制台")}}</button></form><div v-if="error" class="error">{{error}}</div><div class="auth-foot"><span class="dot"></span> {{t("登录后可安全管理您的服务器")}}</div></Card><div class="auth-bottom">{{t("Avenlytic · 轻松掌握服务器运行状态")}}</div></div>
</template>
