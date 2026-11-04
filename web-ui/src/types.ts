export type DockerPort = {
  ip?: string
  private_port: number
  public_port?: number
  type: string
}

export type DockerContainer = {
  id: string
  name: string
  image: string
  state: string
  status: string
  created_at: string
  ports?: DockerPort[]
  cpu_percent?: number
  memory_usage_bytes?: number
  memory_limit_bytes?: number
  memory_percent?: number
  stats_error?: string
}

export type DockerInventory = {
  available: boolean
  error?: string
  containers: DockerContainer[]
}

export type HostSnapshot = {
  hostname: string
  platform?: string
  os: string
  arch: string
  cpu?: {
    cores?: number
    usage_percent?: number
    model?: string
    vendor?: string
    frequency_mhz?: number
    min_frequency_mhz?: number
    max_frequency_mhz?: number
  }
  hardware?: {
    sensors?: { name: string; kind: string; value: number; unit: string; critical?: number }[]
  }
  cgroup?: {
    version: string
    cpu_usage_usec?: number
    cpu_quota_cores?: number
    memory_usage_bytes?: number
    memory_limit_bytes?: number
    pids_current?: number
    pids_limit?: number
    io_read_bytes?: number
    io_write_bytes?: number
  }
  memory?: {
    usage_percent?: number
    total_bytes?: number
    used_bytes?: number
    available_bytes?: number
  }
  disks?: {
    device?: string
    mountpoint: string
    total_bytes?: number
    used_bytes?: number
    free_bytes?: number
    usage_percent?: number
  }[]
  networks?: { name: string; rx_bytes?: number; tx_bytes?: number }[]
  processes?: {
    pid: number
    name: string
    state: string
    cpu_percent?: number
    rss_bytes?: number
  }[]
  process_count?: number
  uptime_seconds?: number
  collected_at?: string
  warnings?: string[]
  load?: { one?: number; five?: number; fifteen?: number }
  docker?: DockerInventory
}

export type Host = {
  agent_id: string
  status: string
  snapshot: HostSnapshot
  security?: {
    failed_login_attempts?: number
    login_window_seconds?: number
    login_monitoring_available?: boolean
    available_updates?: number
    update_check_available?: boolean
    update_check_error?: string
    package_inventory_available?: boolean
    package_inventory_error?: string
    vulnerabilities?: { id: string; summary?: string }[]
    vulnerability_count?: number
    vulnerability_checked_at?: string
    vulnerability_error?: string
  }
}

export type AlertEvent = {
  id: string
  rule_name: string
  host: string
  message: string
  severity: string
  status: string
  created_at: string
}

export type Delivery = {
  id: string
  channel_name: string
  channel_type: string
  status: string
  attempts: number
  created_at: string
  error?: string
}

export type Summary = {
  hosts: number
  online: number
  offline: number
  alerts: number
  avg_cpu: number
  avg_memory: number
}

export type Settings = {
  offline_after_seconds: number
  agent_token: string
  agent_token_configured: boolean
  ui_gate_enabled: boolean
  ui_gate_path: string
}

export type ChannelForm = {
  id: string
  name: string
  type: string
  enabled: boolean
  settings: Record<string, string>
}

export type Rule = {
  id: string
  name: string
  metric: string
  message_template: string
  operator: string
  threshold: number
  duration_seconds: number
  severity: string
  enabled: boolean
  notify_on_recovery: boolean
  channel_ids: string[]
}

export type Channel = {
  id: string
  name: string
  type: string
  enabled: boolean
  settings?: Record<string, string>
}
