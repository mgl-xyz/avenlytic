<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Host } from '../types'
import Card from './Card.vue'
import ModalDialog from './ModalDialog.vue'
const { locale, t } = useI18n()

const { selectedHost, pct, fmtBytes, processState } = defineProps<{
  selectedHost: Host | null
  pct: (value: number | undefined) => string
  fmtBytes: (value: number | undefined) => string
  processState: (value: string) => string
}>()
const emit = defineEmits<{ (event: 'close'): void }>()
</script>

<template>
  <ModalDialog
    :open="!!selectedHost"
    :label="selectedHost?.snapshot.hostname || t('主机详情')"
    size="wide"
    @close="emit('close')"
  >
    <div v-if="selectedHost" class="host-modal">
      <div class="modal-head">
        <div>
          <h2>{{ selectedHost.snapshot.hostname }}</h2>
          <p>{{ selectedHost.snapshot.platform || selectedHost.snapshot.os }} · {{ selectedHost.snapshot.arch }}</p>
        </div>
        <button class="close" :aria-label="t('关闭')" @click="emit('close')">×</button>
      </div>

      <h3>{{ t('基本信息') }}</h3>
      <div v-if="selectedHost.snapshot.warnings?.length" class="security-warning">
        <b>{{ t('采集警告') }}</b>
        <ul><li v-for="warning in selectedHost.snapshot.warnings" :key="warning">{{ warning }}</li></ul>
      </div>
      <div class="host-basics">
        <Card class="host-basic"><span>{{ t('操作系统') }}</span><b>{{ selectedHost.snapshot.platform || selectedHost.snapshot.os }}</b></Card>
        <Card class="host-basic"><span>{{ t('系统架构') }}</span><b>{{ selectedHost.snapshot.arch }}</b></Card>
        <Card class="host-basic"><span>{{ t('处理器核心') }}</span><b>{{ selectedHost.snapshot.cpu?.cores ?? '—' }}</b></Card>
        <Card class="host-basic"><span>{{ t('运行时长') }}</span><b>{{ Math.floor((selectedHost.snapshot.uptime_seconds || 0) / 86400) }}{{ t('天') }} {{ Math.floor(((selectedHost.snapshot.uptime_seconds || 0) % 86400) / 3600) }}{{ t('小时') }}</b></Card>
        <Card class="host-basic"><span>{{ t('状态') }}</span><b>{{ selectedHost.status === 'online' ? t('在线') : t('离线') }}</b></Card>
        <Card class="host-basic"><span>{{ t('监控数据采集时间') }}</span><b>{{ selectedHost.snapshot.collected_at ? new Date(selectedHost.snapshot.collected_at).toLocaleString(locale) : '—' }}</b></Card>
      </div>

      <h3>{{ t('处理器与内存') }}</h3>
      <div class="detail-stats">
        <Card class="detail-stat"><small>CPU {{ selectedHost.snapshot.cpu?.cores ? `· ${selectedHost.snapshot.cpu.cores} ${t('核心')}` : '' }}</small><b>{{ pct(selectedHost.snapshot.cpu?.usage_percent) }}</b></Card>
        <Card v-if="selectedHost.snapshot.cpu?.model" class="detail-stat"><small>{{ t('CPU 型号') }}</small><b>{{ selectedHost.snapshot.cpu.model }}</b></Card>
        <Card v-if="selectedHost.snapshot.cpu?.vendor" class="detail-stat"><small>{{ t('CPU 厂商') }}</small><b>{{ selectedHost.snapshot.cpu.vendor }}</b></Card>
        <Card v-if="selectedHost.snapshot.cpu?.frequency_mhz" class="detail-stat"><small>{{ t('当前频率') }}</small><b>{{ selectedHost.snapshot.cpu.frequency_mhz }} MHz</b></Card>
        <Card v-if="selectedHost.snapshot.cpu?.max_frequency_mhz" class="detail-stat"><small>{{ t('频率范围') }}</small><b>{{ selectedHost.snapshot.cpu.min_frequency_mhz ?? '—' }}–{{ selectedHost.snapshot.cpu.max_frequency_mhz }} MHz</b></Card>
        <Card class="detail-stat"><small>{{ t('内存使用率') }}</small><b>{{ pct(selectedHost.snapshot.memory?.usage_percent) }}</b></Card>
        <Card class="detail-stat"><small>{{ t('内存总量') }}</small><b>{{ fmtBytes(selectedHost.snapshot.memory?.total_bytes) }}</b></Card>
        <Card class="detail-stat"><small>{{ t('已用内存') }}</small><b>{{ fmtBytes(selectedHost.snapshot.memory?.used_bytes) }}</b></Card>
        <Card class="detail-stat"><small>{{ t('可用内存') }}</small><b>{{ fmtBytes(selectedHost.snapshot.memory?.available_bytes) }}</b></Card>
        <Card class="detail-stat"><small>{{ t('进程数') }}</small><b>{{ selectedHost.snapshot.process_count ?? '—' }}</b></Card>
      </div>

      <template v-if="selectedHost.snapshot.hardware?.sensors?.length">
        <h3>{{ t('硬件传感器') }}</h3>
        <div class="table-wrap">
          <table>
            <thead><tr><th>{{ t('传感器') }}</th><th>{{ t('类型') }}</th><th>{{ t('读数') }}</th><th>{{ t('临界值') }}</th></tr></thead>
            <tbody>
              <tr v-for="sensor in selectedHost.snapshot.hardware.sensors" :key="`${sensor.name}-${sensor.kind}`">
                <td>{{ sensor.name }}</td><td>{{ sensor.kind === 'temperature' ? t('温度') : t('风扇') }}</td>
                <td>{{ sensor.value }} {{ sensor.unit }}</td><td>{{ sensor.critical === undefined ? '—' : `${sensor.critical} ${sensor.unit}` }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <template v-if="selectedHost.snapshot.cgroup">
        <h3>{{ t('Cgroup 资源') }} · {{ selectedHost.snapshot.cgroup.version }}</h3>
        <div class="detail-stats">
          <Card class="detail-stat"><small>{{ t('Cgroup CPU 时间') }}</small><b>{{ selectedHost.snapshot.cgroup.cpu_usage_usec === undefined ? '—' : `${(selectedHost.snapshot.cgroup.cpu_usage_usec / 1000000).toFixed(2)} s` }}</b></Card>
          <Card class="detail-stat"><small>{{ t('CPU 配额') }}</small><b>{{ selectedHost.snapshot.cgroup.cpu_quota_cores === undefined ? '—' : `${selectedHost.snapshot.cgroup.cpu_quota_cores} ${t('核心')}` }}</b></Card>
          <Card class="detail-stat"><small>{{ t('Cgroup 内存用量') }}</small><b>{{ fmtBytes(selectedHost.snapshot.cgroup.memory_usage_bytes) }}</b></Card>
          <Card class="detail-stat"><small>{{ t('Cgroup 内存限制') }}</small><b>{{ fmtBytes(selectedHost.snapshot.cgroup.memory_limit_bytes) }}</b></Card>
          <Card class="detail-stat"><small>{{ t('Cgroup 进程数') }}</small><b>{{ selectedHost.snapshot.cgroup.pids_current ?? '—' }}</b></Card>
          <Card class="detail-stat"><small>{{ t('Cgroup 进程限制') }}</small><b>{{ selectedHost.snapshot.cgroup.pids_limit ?? '—' }}</b></Card>
          <Card class="detail-stat"><small>{{ t('Cgroup 磁盘读取累计') }}</small><b>{{ fmtBytes(selectedHost.snapshot.cgroup.io_read_bytes) }}</b></Card>
          <Card class="detail-stat"><small>{{ t('Cgroup 磁盘写入累计') }}</small><b>{{ fmtBytes(selectedHost.snapshot.cgroup.io_write_bytes) }}</b></Card>
        </div>
        <p class="security-note">{{ t('Cgroup 根目录说明') }}</p>
      </template>

      <h3>{{ t('系统负载') }}</h3>
      <div class="detail-stats detail-stats--load">
        <Card class="detail-stat"><small>{{ t('1 分钟') }}</small><b>{{ selectedHost.snapshot.load?.one?.toFixed(2) ?? '—' }}</b></Card>
        <Card class="detail-stat"><small>{{ t('5 分钟') }}</small><b>{{ selectedHost.snapshot.load?.five?.toFixed(2) ?? '—' }}</b></Card>
        <Card class="detail-stat"><small>{{ t('15 分钟') }}</small><b>{{ selectedHost.snapshot.load?.fifteen?.toFixed(2) ?? '—' }}</b></Card>
      </div>

      <h3>{{ t('安全与维护') }}</h3>
      <div class="detail-stats">
        <Card class="detail-stat"><small>{{ t('15 分钟内登录失败次数') }}</small><b>{{ selectedHost.security?.login_monitoring_available ? selectedHost.security.failed_login_attempts : t('暂不可用') }}</b></Card>
        <Card class="detail-stat"><small>{{ t('可用系统更新') }}</small><b>{{ selectedHost.security?.update_check_available ? selectedHost.security.available_updates : t('暂不可用') }}</b></Card>
        <Card class="detail-stat"><small>{{ t('公开漏洞') }}</small><b>{{ selectedHost.security?.vulnerability_checked_at ? selectedHost.security.vulnerability_count : t(selectedHost.security?.vulnerability_error === 'checking' ? '正在检查' : selectedHost.security?.package_inventory_available === false && selectedHost.security.package_inventory_error ? '暂不可用' : '等待首次检查') }}</b></Card>
      </div>
      <p class="security-note">{{ t('漏洞匹配说明') }}</p>
      <div v-if="selectedHost.security?.update_check_error" class="security-warning">{{ t('系统更新检查暂不可用') }}：{{ selectedHost.security.update_check_error }}</div>
      <div v-if="selectedHost.security?.package_inventory_error" class="security-warning">{{ t('软件清单暂不可用') }}：{{ selectedHost.security.package_inventory_error }}</div>
      <div v-if="selectedHost.security?.vulnerability_error && selectedHost.security.vulnerability_error !== 'checking'" class="security-warning">{{ t('漏洞检查暂不可用') }}：{{ selectedHost.security.vulnerability_error }}</div>
      <div v-if="selectedHost.security?.vulnerabilities?.length" class="vulnerability-list">
        <div v-for="vulnerability in selectedHost.security.vulnerabilities.slice(0, 10)" :key="vulnerability.id">
          <b>{{ vulnerability.id }}</b><span v-if="vulnerability.summary">{{ vulnerability.summary }}</span>
        </div>
      </div>

      <h3>{{ t('文件系统') }}</h3>
      <div class="table-wrap">
        <table>
          <thead><tr><th>{{ t('设备') }}</th><th>{{ t('挂载点') }}</th><th>{{ t('总量') }}</th><th>{{ t('已用') }}</th><th>{{ t('可用') }}</th><th>{{ t('使用率') }}</th></tr></thead>
          <tbody>
            <tr v-for="disk in selectedHost.snapshot.disks || []" :key="disk.mountpoint">
              <td>{{ disk.device || '—' }}</td><td>{{ disk.mountpoint }}</td><td>{{ fmtBytes(disk.total_bytes) }}</td><td>{{ fmtBytes(disk.used_bytes) }}</td><td>{{ fmtBytes(disk.free_bytes) }}</td><td>{{ pct(disk.usage_percent) }}</td>
            </tr>
            <tr v-if="!selectedHost.snapshot.disks?.length"><td colspan="6" class="empty">{{ t('暂无文件系统数据') }}</td></tr>
          </tbody>
        </table>
      </div>

      <h3>{{ t('网络接口') }}</h3>
      <div class="table-wrap">
        <table>
          <thead><tr><th>{{ t('接口') }}</th><th>{{ t('接收累计') }}</th><th>{{ t('发送累计') }}</th></tr></thead>
          <tbody>
            <tr v-for="network in selectedHost.snapshot.networks || []" :key="network.name"><td>{{ network.name }}</td><td>{{ fmtBytes(network.rx_bytes) }}</td><td>{{ fmtBytes(network.tx_bytes) }}</td></tr>
            <tr v-if="!selectedHost.snapshot.networks?.length"><td colspan="3" class="empty">{{ t('暂无网络接口数据') }}</td></tr>
          </tbody>
        </table>
      </div>

      <h3>{{ t('运行中的进程') }}</h3>
      <div class="table-wrap">
        <table>
          <thead><tr><th>{{ t('进程编号') }}</th><th>{{ t('进程') }}</th><th>{{ t('状态') }}</th><th>CPU</th><th>{{ t('内存占用') }}</th></tr></thead>
          <tbody>
            <tr v-for="process in selectedHost.snapshot.processes || []" :key="process.pid"><td>{{ process.pid }}</td><td>{{ process.name }}</td><td>{{ processState(process.state) }}</td><td>{{ pct(process.cpu_percent) }}</td><td>{{ fmtBytes(process.rss_bytes) }}</td></tr>
            <tr v-if="!selectedHost.snapshot.processes?.length"><td colspan="5" class="empty">{{ t('无进程数据') }}</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </ModalDialog>
</template>
