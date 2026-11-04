<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { AlertEvent } from '../types'
import Card from './Card.vue'
const { locale, t } = useI18n()
const { alerts, severity, eventStatus, alertMessage, ackAlert, refresh } = defineProps<{ alerts: AlertEvent[], severity: (value: string) => string, eventStatus: (value: string) => string, alertMessage: (value: string) => string, ackAlert: (id: string) => void, refresh: () => void }>()
</script>
<template>
<div class="section-head"><div><h2>{{t("告警事件")}}</h2><p>{{t("触发、确认和恢复的告警记录。")}}</p></div><button class="ghost" @click="refresh">{{t("刷新事件 ↻")}}</button></div><Card class="panel"><div class="table-wrap"><table><thead><tr><th>{{t("级别")}}</th><th>{{t("规则")}}</th><th>{{t("主机")}}</th><th>{{t("事件内容")}}</th><th>{{t("状态")}}</th><th>{{t("触发时间")}}</th><th>{{t("操作")}}</th></tr></thead><tbody><tr v-for="a in alerts" :key="a.id"><td>{{severity(a.severity)}}</td><td>{{a.rule_name}}</td><td>{{a.host}}</td><td>{{alertMessage(a.message)}}</td><td><span class="badge" :class="a.status">{{eventStatus(a.status)}}</span></td><td>{{new Date(a.created_at).toLocaleString(locale)}}</td><td><button v-if="a.status==='firing'" class="link" @click="ackAlert(a.id)">{{t("确认")}}</button></td></tr><tr v-if="!alerts.length"><td colspan="7" class="empty">{{t("目前没有告警，系统状态良好。")}}</td></tr></tbody></table></div></Card>
</template>
