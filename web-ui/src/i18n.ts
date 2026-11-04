import { createI18n } from 'vue-i18n'
import enUS from './locales/en-US.json'
import zhCN from './locales/zh-CN.json'

export type Locale = 'zh-CN' | 'en-US'

const initialLocale: Locale =
  typeof localStorage !== 'undefined' && localStorage.getItem('avenlytic.locale') === 'en-US'
    ? 'en-US'
    : 'zh-CN'

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale,
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
})

export const locale = i18n.global.locale

export function setLocale(value: Locale): void {
  locale.value = value
  localStorage.setItem('avenlytic.locale', value)
}
