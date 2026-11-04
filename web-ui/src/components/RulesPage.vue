<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Rule } from '../types'
import Card from './Card.vue'
const { t } = useI18n()
const { rules, metricLabel, severity, openRule, deleteRule } = defineProps<{ rules: Rule[], metricLabel: (value: string) => string, severity: (value: string) => string, openRule: (rule?: Rule) => void, deleteRule: (id: string) => void }>()
const emit = defineEmits<{ (event: 'navigate', view: string): void; (event: 'select-host', host: Host): void; (event: 'update:host-query', value: string): void }>()
</script>
<template>
<div class="section-head"><div><h2>{{t("告警规则")}}</h2><p>{{t("定义阈值、持续时间、通知通道和恢复行为。")}}</p></div><button class="primary" @click="openRule()">{{t("＋ 新建规则")}}</button></div><Card class="panel"><div class="table-wrap"><table><thead><tr><th>{{t("规则名称")}}</th><th>{{t("指标")}}</th><th>{{t("条件")}}</th><th>{{t("持续")}}</th><th>{{t("级别")}}</th><th>{{t("状态")}}</th><th>{{t("操作")}}</th></tr></thead><tbody><tr v-for="r in rules" :key="r.id"><td><b>{{r.name}}</b></td><td>{{metricLabel(r.metric)}}</td><td>{{r.operator}} {{r.threshold}}</td><td>{{r.duration_seconds}} {{t("秒")}}</td><td>{{severity(r.severity)}}</td><td><span class="badge" :class="r.enabled?'online':'offline'">{{r.enabled?t("启用"):t("停用")}}</span></td><td><div class="row-actions"><button class="action-edit" @click="openRule(r)">{{t("编辑")}}</button><button class="action-delete" @click="deleteRule(r.id)">{{t("删除")}}</button></div></td></tr><tr v-if="!rules.length"><td colspan="7" class="empty">{{t("尚未创建规则")}}</td></tr></tbody></table></div></Card>
</template>
