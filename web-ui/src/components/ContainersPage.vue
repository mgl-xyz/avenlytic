<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DockerPort, Host } from '../types'
import Card from './Card.vue'

const { t, locale } = useI18n()
const { hosts, fmtBytes, pct } = defineProps<{
  hosts: Host[]
  fmtBytes: (value: number | undefined) => string
  pct: (value: number | undefined) => string
}>()
const emit = defineEmits<{ (event: 'navigate', view: string): void }>()

const dockerHosts = computed(() => hosts.filter(host => host.snapshot.docker))
const selectedAgentId = ref('')
const selectedHost = computed(() =>
  dockerHosts.value.find(host => host.agent_id === selectedAgentId.value) || dockerHosts.value[0],
)
const containers = computed(() => selectedHost.value?.snapshot.docker?.containers || [])
const runningCount = computed(() => containers.value.filter(container => container.state === 'running').length)
const stoppedCount = computed(() => containers.value.length - runningCount.value)
const stateLabels: Record<string, string> = {
  running: '运行中',
  exited: '已退出',
  paused: '已暂停',
  restarting: '重启中',
  created: '已创建',
  dead: '异常停止',
}

function portLabel(port: DockerPort) {
  const address = port.public_port ? `${port.ip ? `${port.ip}:` : ''}${port.public_port}->` : ''
  return `${address}${port.private_port}/${port.type}`
}

function createdAt(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString(locale.value)
}

function containerBytes(value: number | undefined) {
  if (value === undefined) return '—'
  return value === 0 ? '0 B' : fmtBytes(value)
}
</script>

<template>
  <div class="containers-page">
    <div class="docker-heading">
      <p>{{ t('查看已启用 Docker 采集的主机、容器状态、镜像和资源用量。') }}</p>
      <span class="readonly-badge"><i></i>{{ t('只读监控') }}</span>
    </div>

    <Card v-if="!dockerHosts.length" class="panel empty-panel">
      <div class="docker-empty">
        <div class="empty-icon">◧</div>
        <div class="empty-copy">
          <h2>{{ hosts.length ? t('尚未启用 Docker 采集') : t('暂无已接入主机') }}</h2>
          <p v-if="hosts.length">{{ t('在 Linux Agent 启动参数中配置 --docker-socket，并使用仅允许读取的 Docker API 代理。') }}</p>
          <p v-else>{{ t('先接入 Linux 主机，再为 Agent 开启 Docker 容器采集。') }}</p>
          <code>--docker-socket /run/docker-ro.sock</code>
          <button class="ghost" @click="emit('navigate', 'settings')">{{ t('查看 Agent 接入设置') }} <span aria-hidden="true">→</span></button>
        </div>
      </div>
    </Card>

    <template v-else>
      <section class="host-section">
        <div class="section-head">
          <div>
            <h2>{{ t('Docker 主机') }}</h2>
            <p>{{ t('选择一台主机，只查看该主机上的容器。') }}</p>
          </div>
          <span class="host-count">{{ dockerHosts.length }} {{ t('台主机') }}</span>
        </div>
        <div class="docker-host-list">
          <button
            v-for="host in dockerHosts"
            :key="host.agent_id"
            class="docker-host-option"
            :class="{ selected: selectedHost?.agent_id === host.agent_id }"
            :aria-pressed="selectedHost?.agent_id === host.agent_id"
            @click="selectedAgentId = host.agent_id"
          >
            <span class="docker-host-icon">◧</span>
            <span class="docker-host-info">
              <b>{{ host.snapshot.hostname }}</b>
              <small>{{ host.snapshot.os }} · {{ host.snapshot.arch }}</small>
            </span>
            <span class="docker-host-meta">
              <span class="badge" :class="host.status">{{ t(host.status === 'online' ? '在线' : '离线') }}</span>
              <small v-if="host.snapshot.docker?.available">{{ host.snapshot.docker.containers.length }} {{ t('个容器') }}</small>
              <small v-else class="docker-unavailable">{{ t('Docker 数据暂不可用') }}</small>
            </span>
          </button>
        </div>
      </section>

      <section v-if="selectedHost" class="selected-host">
        <div class="selected-host-heading">
          <div>
            <span>{{ t('当前主机') }}</span>
            <h2>{{ selectedHost.snapshot.hostname }}</h2>
          </div>
          <span class="badge" :class="selectedHost.status">{{ t(selectedHost.status === 'online' ? '在线' : '离线') }}</span>
        </div>

        <section class="stats">
          <Card as="article" class="stat-card"><span>{{ t('容器总数') }}</span><b>{{ containers.length }}</b><small>{{ t('此主机已发现容器') }}</small><i>◧</i></Card>
          <Card as="article" class="stat-card"><span>{{ t('运行中') }}</span><b class="green">{{ runningCount }}</b><small>{{ t('当前运行的容器') }}</small><i>●</i></Card>
          <Card as="article" class="stat-card"><span>{{ t('非运行') }}</span><b>{{ stoppedCount }}</b><small>{{ t('已停止或已退出') }}</small><i>Ⅱ</i></Card>
          <Card as="article" class="stat-card"><span>{{ t('Docker 状态') }}</span><b class="host-state" :class="{ 'is-unavailable': !selectedHost.snapshot.docker?.available }">{{ t(selectedHost.snapshot.docker?.available ? '正常' : '不可用') }}</b><small>{{ selectedHost.snapshot.docker?.available ? t('正在接收容器数据') : t('检查 Agent 与 Docker 连接') }}</small><i>◉</i></Card>
        </section>

        <div v-if="!selectedHost.snapshot.docker?.available" class="docker-error">
          <b>{{ t('Docker 数据暂不可用') }}</b>
          <span v-if="selectedHost.snapshot.docker?.error">{{ selectedHost.snapshot.docker.error }}</span>
        </div>

        <Card class="panel">
          <div class="panel-head"><b>{{ selectedHost.snapshot.hostname }} · {{ t('容器清单') }}</b><span>{{ containers.length }} {{ t('个容器') }}</span></div>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{{ t('容器') }}</th>
                  <th>{{ t('镜像') }}</th>
                  <th>{{ t('状态') }}</th>
                  <th>CPU</th>
                  <th>{{ t('内存占用') }}</th>
                  <th>{{ t('端口') }}</th>
                  <th>{{ t('创建时间') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="container in containers" :key="container.id">
                  <td>
                    <b>{{ container.name }}</b>
                    <small class="container-id">{{ container.id.slice(0, 12) }}</small>
                  </td>
                  <td class="container-image">{{ container.image }}</td>
                  <td>
                    <span class="badge" :class="container.state === 'running' ? 'online' : 'offline'">{{ t(stateLabels[container.state] || container.state) }}</span>
                    <small class="container-status">{{ container.status }}</small>
                  </td>
                  <td>{{ container.cpu_percent === undefined ? '—' : pct(container.cpu_percent) }}</td>
                  <td>
                    <template v-if="container.memory_usage_bytes !== undefined">
                      {{ containerBytes(container.memory_usage_bytes) }} / {{ containerBytes(container.memory_limit_bytes) }}
                      <small v-if="container.memory_percent !== undefined" class="container-status">{{ pct(container.memory_percent) }}</small>
                    </template>
                    <span v-else :title="container.stats_error">{{ container.stats_error || '—' }}</span>
                  </td>
                  <td>{{ container.ports?.length ? container.ports.map(portLabel).join(', ') : '—' }}</td>
                  <td>{{ createdAt(container.created_at) }}</td>
                </tr>
                <tr v-if="!containers.length"><td colspan="7" class="empty">{{ t('该主机暂无容器') }}</td></tr>
              </tbody>
            </table>
          </div>
        </Card>
      </section>
    </template>

    <template v-if="dockerHosts.length">
      <p class="docker-note">{{ t('此页面为只读监控，不会启动、停止或修改容器。') }}</p>
    </template>
  </div>
</template>

<style scoped>
.containers-page { display: grid; gap: 18px; }
.docker-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.docker-heading p { margin: 0; color: var(--muted); font-size: 12px; }
.readonly-badge { display: inline-flex; align-items: center; gap: 7px; padding: 6px 10px; border: 1px solid var(--line); border-radius: 999px; background: var(--surface); color: var(--muted); font-size: 11px; white-space: nowrap; }
.readonly-badge i { width: 7px; height: 7px; border-radius: 50%; background: #35a779; }
.containers-page .stats { margin: 0; }
.containers-page .stats article { min-width: 0; }
.host-section { display: grid; gap: 12px; }
.host-section .section-head { margin: 0; }
.host-count { color: var(--muted); font-size: 11px; }
.docker-host-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); gap: 10px; }
.docker-host-option { min-width: 0; display: flex; align-items: center; gap: 11px; padding: 13px 14px; border: 1px solid var(--line); border-radius: 12px; background: var(--surface); color: var(--text); text-align: left; transition: border-color .18s, box-shadow .18s, background .18s; }
.docker-host-option:hover { border-color: var(--accent); }
.docker-host-option.selected { border-color: var(--accent); box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 15%, transparent); }
.docker-host-icon { flex: 0 0 36px; width: 36px; height: 36px; display: grid; place-items: center; border-radius: 10px; background: color-mix(in srgb, var(--accent) 12%, var(--surface)); color: var(--accent); font-size: 18px; }
.docker-host-info { min-width: 0; flex: 1; }
.docker-host-info b,.docker-host-info small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.docker-host-info b { font-size: 12px; }
.docker-host-info small,.docker-host-meta small { margin-top: 5px; color: var(--muted); font-size: 10px; }
.docker-host-meta { display: flex; flex-direction: column; align-items: flex-end; gap: 1px; white-space: nowrap; }
.docker-host-meta .badge { margin: 0; }
.docker-host-meta .docker-unavailable { color: #b7791f; }
.selected-host { display: grid; gap: 14px; }
.selected-host-heading { display: flex; align-items: center; justify-content: space-between; }
.selected-host-heading>div>span { color: var(--muted); font-size: 10px; }
.selected-host-heading h2 { margin: 4px 0 0; color: var(--text); font-size: 19px; }
.host-state { font-size: 22px !important; }
.host-state.is-unavailable { color: #c78322; }
.docker-error { display: flex; gap: 12px; align-items: center; padding: 12px 16px; border: 1px solid #e8c273; border-radius: 12px; background: var(--surface); color: var(--text); }
.empty-panel { min-height: 220px; display: flex; align-items: center; }
.docker-empty { display: flex; align-items: flex-start; gap: 18px; width: min(620px, 100%); margin: 0 auto; padding: 36px 24px; }
.empty-icon { flex: 0 0 48px; height: 48px; display: grid; place-items: center; border-radius: 14px; background: color-mix(in srgb, var(--accent) 12%, var(--surface)); color: var(--accent); font-size: 24px; }
.empty-copy h2 { margin: 1px 0 7px; color: var(--text); font-size: 16px; }
.empty-copy p { margin: 0 0 12px; color: var(--muted); font-size: 12px; line-height: 1.7; }
.empty-copy code { display: inline-block; padding: 8px 10px; border: 1px solid var(--line); border-radius: 7px; background: var(--canvas); color: var(--text); font-size: 11px; }
.empty-copy .ghost { margin-left: 9px; }
.docker-error { display: flex; gap: 12px; align-items: center; padding: 12px 16px; border: 1px solid #e8c273; border-radius: 12px; background: var(--surface); color: var(--text); }
.container-id,.container-status { display: block; margin-top: 4px; color: var(--muted); font-size: 11px; }
.container-image { max-width: 260px; overflow-wrap: anywhere; }
.docker-note { margin: 0; color: var(--muted); font-size: 11px; text-align: right; }
@media (max-width: 980px) { .containers-page .stats { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 560px) { .containers-page .stats { grid-template-columns: 1fr; } .docker-host-list { grid-template-columns: 1fr; } .docker-empty { padding: 25px 18px; } .empty-copy .ghost { margin: 12px 0 0; display: block; } }
</style>
