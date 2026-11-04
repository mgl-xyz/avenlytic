<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Host, HostSnapshot } from '../types'
import Card from './Card.vue'
const { t } = useI18n()
const { filteredHosts, hostQuery, pct, diskPct, ago } = defineProps<{ filteredHosts: Host[], hostQuery: string, pct: (value: number | undefined) => string, diskPct: (snapshot: HostSnapshot) => number, ago: (value: string | undefined) => string }>()
const emit = defineEmits<{ (event: 'navigate', view: string): void; (event: 'select-host', host: Host): void; (event: 'update:host-query', value: string): void }>()
</script>
<template>
<div class="section-head"><div><h2>{{t("服务器清单")}}</h2><p>{{t("监控程序会主动连接本服务，无需在服务器上开放入站端口。")}}</p></div><button class="primary" @click="emit('navigate', 'settings')">{{t("＋ 添加服务器")}}</button></div><Card class="panel"><div class="panel-head"><input :value="hostQuery" @input="emit('update:host-query', ($event.target as HTMLInputElement).value)" :placeholder="t('搜索服务器名称或操作系统')" class="search-input"><span>{{filteredHosts.length}} {{t("台设备")}}</span></div><div class="table-wrap"><table><thead><tr><th>{{t("主机名")}}</th><th>{{t("系统")}}</th><th>CPU</th><th>{{t("内存")}}</th><th>{{t("磁盘")}}</th><th>{{t("进程数")}}</th><th>{{t("运行时间")}}</th><th>{{t("最近心跳")}}</th><th>{{t("状态")}}</th></tr></thead><tbody><tr v-for="h in filteredHosts" :key="h.agent_id" class="click-row" @click="emit('select-host', h)"><td><b>{{h.snapshot.hostname}}</b></td><td>{{h.snapshot.os}} / {{h.snapshot.arch}}</td><td>{{pct(h.snapshot.cpu?.usage_percent)}}</td><td>{{pct(h.snapshot.memory?.usage_percent)}}</td><td>{{pct(diskPct(h.snapshot))}}</td><td>{{h.snapshot.process_count}}</td><td>{{Math.floor((h.snapshot.uptime_seconds||0)/3600)}} {{t("小时")}}</td><td>{{ago(h.snapshot.collected_at)}}</td><td><span class="badge" :class="h.status">{{h.status==='online'?t("在线"):t("离线")}}</span></td></tr><tr v-if="!filteredHosts.length"><td colspan="9" class="empty">{{t("暂无服务器")}}</td></tr></tbody></table></div></Card>
</template>
