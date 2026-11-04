<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { setLocale } from './i18n'
import { useLiveUpdates } from './composables/useLiveUpdates'
import { useToast } from './composables/useToast'
import AuthView from './components/AuthView.vue'
import AlertRuleModal from './components/AlertRuleModal.vue'
import NotificationChannelModal from './components/NotificationChannelModal.vue'
import HostDetailsModal from './components/HostDetailsModal.vue'
import PasswordModal from './components/PasswordModal.vue'
import ToastNotice from './components/ToastNotice.vue'
import DropdownSelect from './components/DropdownSelect.vue'
import { updateRouterAuthState } from './router'
import type { AlertEvent, Channel, ChannelForm, Delivery, Host, HostSnapshot, Rule, Settings, Summary } from './types'
const route = useRoute()
const routeView = useRouter()
const { locale, t } = useI18n()
const view = computed(() => String(route.name || 'overview'))
const initialized=ref(false), logged=ref(false), username=ref(''), busy=ref(false), error=ref('')
const authResolved=ref(false)
const { message: toast, level: toastLevel, notify, pause: pauseToast, resume: resumeToast, clear: clearToast } = useToast()
const theme=ref(localStorage.getItem('avenlytic.theme')==='dark'?'dark':'light')
function applyTheme(){document.documentElement.dataset.theme=theme.value;localStorage.setItem('avenlytic.theme',theme.value)}
function toggleTheme(){theme.value=theme.value==='light'?'dark':'light';applyTheme()}
function toggleLocale(){setLocale(locale.value==='zh-CN'?'en-US':'zh-CN');document.documentElement.lang=locale.value}
applyTheme()
const refreshInterval=ref(Number(localStorage.getItem('avenlytic.refreshInterval') ?? 10)), showAgentToken=ref(false)
const refreshOptions=computed(()=>[5,10,15,30,60,0].map(value=>({value,label:t(value===0?'关闭':value===60?'1 分钟':`${value} 秒`)})))
const setup=reactive({username:'admin',password:'',confirm:''}), login=reactive({username:'admin',password:''})
const hosts=ref<Host[]>([]), alerts=ref<AlertEvent[]>([]), rules=ref<Rule[]>([]), channels=ref<Channel[]>([]), deliveries=ref<Delivery[]>([]), summary=reactive<Summary>({hosts:0,online:0,offline:0,alerts:0,avg_cpu:0,avg_memory:0})
const settings=reactive<Settings>({offline_after_seconds:45,agent_token:'',agent_token_configured:false,ui_gate_enabled:false,ui_gate_path:''})
const ruleForm=reactive<Rule>({id:'',name:'',metric:'memory.usage_percent',message_template:'',operator:'>',threshold:85,duration_seconds:60,severity:'warning',enabled:true,notify_on_recovery:true,channel_ids:[]})
const channelForm=reactive<ChannelForm>({id:'',name:'',type:'dingtalk',enabled:true,settings:{}})
const showRule=ref(false),showChannel=ref(false),showPassword=ref(false),selectedHost=ref<Host|null>(null),hostQuery=ref(''),passwordForm=reactive({current:'',password:'',confirm:''})
let refreshing=false, refreshQueued=false, channelRequest=0
function navigateToView(next:string){if(next!==view.value)void routeView.push({name:next})}
const filteredHosts=computed(()=>hosts.value.filter(h=>(h.snapshot.hostname+' '+h.agent_id+' '+h.snapshot.os).toLowerCase().includes(hostQuery.value.toLowerCase())))
const agentCommand=computed(()=>`./avenlytic-agent-linux-amd64 --server https://YOUR_SERVER:9080 --token ${settings.agent_token||'YOUR_AGENT_TOKEN'} --interval 5s`)
const uiGateURL=computed(()=>settings.ui_gate_enabled&&settings.ui_gate_path?`${window.location.origin}/${settings.ui_gate_path}`:'')
const pageTitle=computed(()=>t(({overview:'基础设施总览',hosts:'服务器管理',containers:'Docker 容器',alerts:'告警中心',rules:'告警规则',channels:'通知通道',settings:'系统设置'} as Record<string,string>)[view.value]||'系统设置'))
const channelTypes=[['dingtalk','钉钉机器人'],['feishu','飞书机器人'],['wecom','企业微信机器人'],['email','SMTP 邮件'],['slack','Slack Webhook'],['telegram','Telegram Bot'],['webhook','通用 Webhook']]
const metricTypes=[['cpu.usage_percent','CPU 使用率'],['memory.usage_percent','内存使用率'],['disk.usage_percent','磁盘使用率'],['load.1m','系统负载 1 分钟'],['load.5m','系统负载 5 分钟'],['load.15m','系统负载 15 分钟'],['process.count','进程数量'],['agent.online','服务器在线状态'],['security.login_failures_15m','15 分钟内登录失败次数'],['system.updates_available','可用系统更新数量'],['security.vulnerabilities','公开漏洞数量']]
async function api(path:string,opts:RequestInit={}){const r=await fetch('/api/v1'+path,{credentials:'same-origin',...opts,headers:{'Content-Type':'application/json',...(opts.headers||{})}});if(!r.ok){const t=await r.text();throw new Error(t||r.statusText)}return r.status===204?null:r.json()}
function setRefreshInterval(v:number){refreshInterval.value=v;localStorage.setItem('avenlytic.refreshInterval',String(v))}
async function copyAgentToken(){if(!settings.agent_token){notify('请先生成或输入接入密钥','warning');return}try{await navigator.clipboard.writeText(settings.agent_token);notify('接入密钥已复制，请妥善保管','success')}catch{notify('复制失败，请手动复制密钥','error')}}
function loadCurrentView(){if(logged.value&&!['overview','hosts','alerts'].includes(view.value))void refresh()}
async function checkAuth(){try{const s=await api('/auth/status');initialized.value=s.initialized;if(!initialized.value){logged.value=false;return}try{const me=await api('/auth/me');username.value=me.username;logged.value=true;loadCurrentView()}catch{logged.value=false}}catch(e:any){error.value=e.message}}
async function submitSetup(){if(setup.password!==setup.confirm){error.value=t('两次输入的密码不一致');return}busy.value=true;error.value='';try{await api('/auth/setup',{method:'POST',body:JSON.stringify({username:setup.username,password:setup.password})});initialized.value=true;logged.value=true;username.value=setup.username;loadCurrentView()}catch(e:any){error.value=e.message}finally{busy.value=false}}
async function submitLogin(){busy.value=true;error.value='';try{await api('/auth/login',{method:'POST',body:JSON.stringify(login)});logged.value=true;username.value=login.username;loadCurrentView()}catch(e:any){error.value=e.message}finally{busy.value=false}}
async function logout(){await api('/auth/logout',{method:'POST'});logged.value=false;login.password=''}
async function refresh(queueIfBusy=true,monitoringOnly=false){if(!logged.value)return;if(monitoringOnly&&!['overview','hosts','containers','alerts'].includes(view.value))return;if(refreshing){if(queueIfBusy)refreshQueued=true;return}refreshing=true;try{switch(view.value){case'overview':{const[h,s,a]=await Promise.all([api('/hosts'),api('/summary'),api('/alerts')]);hosts.value=h.items||[];Object.assign(summary,s);alerts.value=a.items||[];break}case'hosts':case'containers':{const h=await api('/hosts');hosts.value=h.items||[];break}case'alerts':{const a=await api('/alerts');alerts.value=a.items||[];break}case'rules':{const[r,c]=await Promise.all([api('/rules'),api('/channels')]);rules.value=r.items||[];channels.value=c.items||[];break}case'channels':{const[c,d]=await Promise.all([api('/channels'),api('/deliveries')]);channels.value=c.items||[];deliveries.value=d.items||[];break}case'settings':{const st=await api('/settings');settings.offline_after_seconds=st.offline_after_seconds;settings.agent_token_configured=st.agent_token_configured;settings.ui_gate_enabled=st.ui_gate_enabled;break}}}catch(e:any){notify('数据加载失败：'+e.message,'error')}finally{refreshing=false;if(refreshQueued){refreshQueued=false;void refresh()}}}
function openRule(r?:Rule){Object.assign(ruleForm,r?JSON.parse(JSON.stringify(r)):{id:'',name:'',metric:'memory.usage_percent',message_template:'',operator:'>',threshold:85,duration_seconds:60,severity:'warning',enabled:true,notify_on_recovery:true,channel_ids:[]});showRule.value=true}
async function saveRule(){try{const saved=await api('/rules',{method:'POST',body:JSON.stringify(ruleForm)});const index=rules.value.findIndex(rule=>rule.id===saved.id);if(index<0)rules.value.push(saved);else rules.value[index]=saved;showRule.value=false;notify('告警规则已保存','success')}catch(e:any){notify(e.message,'error')}}
async function deleteRule(id:string){if(!confirm(t("确定删除这条告警规则？")))return;try{await api('/rules/'+id,{method:'DELETE'});rules.value=rules.value.filter(rule=>rule.id!==id);notify('规则已删除','success')}catch(e:any){notify(e.message,'error')}}
function channelFields(type:string){if(type==='email')return [['smtp_host','SMTP 主机'],['smtp_port','SMTP 端口'],['username','用户名'],['password','密码'],['from','发件人'],['to','收件人']];if(type==='telegram')return [['bot_token','Bot Token'],['chat_id','Chat ID']];if(type==='webhook')return [['url','Webhook URL'],['authorization','Authorization（可选）']];if(type==='dingtalk'||type==='feishu')return [['webhook_url','Webhook URL'],['secret','签名密钥（可选）']];return [['webhook_url','Webhook URL']]}
async function openChannel(c?:Channel){const request=++channelRequest;Object.assign(channelForm,{id:c?.id||'',name:c?.name||'',type:c?.type||'dingtalk',enabled:c?.enabled??true,settings:c?.settings?{...c.settings}:{}});showChannel.value=true;if(!c)return;try{const details=await api('/channels/'+encodeURIComponent(c.id));if(request!==channelRequest||!showChannel.value||channelForm.id!==c.id)return;Object.assign(channelForm,{id:details.id,name:details.name,type:details.type,enabled:details.enabled,settings:{...details.settings}})}catch(e:any){if(request===channelRequest&&showChannel.value)notify('通知通道加载失败：'+e.message,'error')}}
function closeChannel(){channelRequest++;showChannel.value=false}
async function saveChannel(){try{const saved=await api('/channels',{method:'POST',body:JSON.stringify(channelForm)});const index=channels.value.findIndex(channel=>channel.id===saved.id);if(index<0)channels.value.push(saved);else channels.value[index]={...channels.value[index],...saved};showChannel.value=false;notify('通知通道已保存','success')}catch(e:any){notify(e.message,'error')}}
async function testChannel(){try{await api('/channels/test',{method:'POST',body:JSON.stringify(channelForm)});notify('测试通知发送成功','success')}catch(e:any){notify('发送失败：'+e.message,'error')}}
async function ackAlert(id:string){try{await api('/alerts/'+id+'/ack',{method:'POST'});const alert=alerts.value.find(event=>event.id===id);if(alert)alert.status='acknowledged';notify('告警已确认','success')}catch(e:any){notify(e.message,'error')}}
async function copyAgentCommand(){try{await navigator.clipboard.writeText(agentCommand.value);notify('服务器接入命令已复制','success')}catch{notify('复制失败，请手动选择命令','error')}}
async function testSaved(id:string){try{await api('/channels/'+id+'/test',{method:'POST'});notify('测试通知发送成功','success')}catch(e:any){notify(e.message,'error')}}
async function deleteChannel(id:string){if(!confirm(t("确定删除通知通道？")))return;try{await api('/channels/'+id,{method:'DELETE'});channels.value=channels.value.filter(channel=>channel.id!==id);notify('通知通道已删除','success')}catch(e:any){notify(e.message,'error')}}
function randomToken(){const a=new Uint8Array(32);crypto.getRandomValues(a);settings.agent_token=Array.from(a).map(x=>x.toString(16).padStart(2,'0')).join('')}
async function saveSettings(){try{const result=await api('/settings',{method:'POST',body:JSON.stringify(settings)});settings.agent_token='';settings.ui_gate_enabled=result.ui_gate_enabled;notify('系统设置已保存','success')}catch(e:any){notify(e.message,'error')}}
function randomUIGatePath(){const bytes=new Uint8Array(32);crypto.getRandomValues(bytes);settings.ui_gate_path=Array.from(bytes,b=>b.toString(16).padStart(2,'0')).join('');settings.ui_gate_enabled=true}
async function copyUIGateLink(){if(!uiGateURL.value)return;try{await navigator.clipboard.writeText(uiGateURL.value);notify('安全链接已复制','success')}catch{notify('复制失败，请手动复制安全链接','error')}}
async function changePassword(){if(passwordForm.password!==passwordForm.confirm){notify('两次密码不一致','warning');return}try{await api('/auth/password',{method:'POST',body:JSON.stringify({current:passwordForm.current,password:passwordForm.password})});showPassword.value=false;Object.assign(passwordForm,{current:'',password:'',confirm:''});notify('密码已修改','success')}catch(e:any){notify(e.message,'error')}}
function fmtBytes(n:number|undefined){if(!n)return '—';const u=['B','KB','MB','GB','TB'];let i=0;while(n>=1024&&i<4){n/=1024;i++}return n.toFixed(i?1:0)+' '+u[i]}
function ago(t:string|undefined){if(!t)return '—';const s=Math.max(0,Math.floor((Date.now()-new Date(t).getTime())/1000));return locale.value==='en-US'?(s<60?`${s} sec ago`:s<3600?`${Math.floor(s/60)} min ago`:`${Math.floor(s/3600)} hr ago`):(s<60?`${s} 秒前`:s<3600?`${Math.floor(s/60)} 分钟前`:`${Math.floor(s/3600)} 小时前`)}
function pct(v:number|undefined){return `${Number(v||0).toFixed(1)}%`}
function diskPct(s:HostSnapshot){return Math.max(0,...(s.disks||[]).map(d=>Number(d.usage_percent||0)))}
function severity(s:string){return t(s==='critical'?'严重':s==='warning'?'警告':'提示')}
function metricLabel(metric:string){return t(metricTypes.find(([id])=>id===metric)?.[1]||metric)}
function processState(state:string){return t(({R:'运行中',S:'休眠',D:'等待 I/O',Z:'已结束',T:'已暂停',t:'跟踪中',X:'已结束',I:'空闲'} as Record<string,string>)[state]||state)}
function eventStatus(status:string){return t(({firing:'处理中',acknowledged:'已确认',resolved:'已恢复'} as Record<string,string>)[status]||status)}
function deliveryStatus(status:string){return t(({sent:'发送成功',failed:'发送失败'} as Record<string,string>)[status]||status)}
function channelType(type:string){return t(channelTypes.find(([id])=>id===type)?.[1]||type)}
function alertMessage(message:string){
  const threshold=message.match(/^([\w.]+) is ([\d.]+) \(threshold (>=|>|<=|<|==|!=) ([\d.]+)\)$/)
  if(threshold)return t('当前监控指标已达到告警条件').replace('{metric}',metricLabel(threshold[1])).replace('{value}',threshold[2]).replace('{operator}',threshold[3]).replace('{threshold}',threshold[4])
  const offline=message.match(/^Agent heartbeat timeout; last seen (.+)$/)
  if(offline)return t('服务器长时间未连接，最后在线时间：')+new Date(offline[1]).toLocaleString(locale.value)
  if(message==='Condition returned to normal')return t('监控指标已恢复正常')
  if(message==='Agent heartbeat restored')return t('服务器已恢复连接')
  return message
}
function applyMonitoringUpdate(event:Event){try{const data=JSON.parse((event as MessageEvent<string>).data);hosts.value=data.hosts.items||[];Object.assign(summary,data.summary);alerts.value=data.alerts.items||[]}catch(e:any){notify('实时监控数据加载失败：'+e.message,'error')}}
const authProps = computed(() => ({
  initialized: initialized.value,
  busy: busy.value,
  error: error.value,
  setup,
  login,
  submitSetup,
  submitLogin,
  theme: theme.value,
  toggleTheme,
  locale: locale.value,
  toggleLocale,
}))
const pageProps = computed(() => {
  switch (view.value) {
    case 'overview':
      return { hosts: hosts.value, summary, alerts: alerts.value, pct, diskPct, ago, alertMessage, eventStatus }
    case 'hosts':
      return { filteredHosts: filteredHosts.value, hostQuery: hostQuery.value, pct, diskPct, ago }
    case 'containers':
      return { hosts: hosts.value, fmtBytes, pct }
    case 'alerts':
      return { alerts: alerts.value, severity, eventStatus, alertMessage, ackAlert, refresh }
    case 'rules':
      return { rules: rules.value, metricLabel, severity, openRule, deleteRule }
    case 'channels':
      return { channels: channels.value, deliveries: deliveries.value, channelType, deliveryStatus, openChannel, testSaved, deleteChannel }
    case 'settings':
      return { settings, showAgentToken: showAgentToken.value, agentCommand: agentCommand.value, username: username.value, randomToken, copyAgentToken, saveSettings, copyAgentCommand, logout, uiGateUrl: uiGateURL.value, randomUiGatePath: randomUIGatePath, copyUiGateLink: copyUIGateLink }
    default:
      return {}
  }
})
const pageListeners = computed(() => {
  switch (view.value) {
    case 'overview':
      return { navigate: navigateToView }
    case 'hosts':
      return {
        navigate: navigateToView,
        'update:host-query': (value: string) => { hostQuery.value = value },
        'select-host': (host: Host) => { selectedHost.value = host },
      }
    case 'containers':
      return { navigate: navigateToView }
    case 'settings':
      return {
        'toggle-agent-token': () => { showAgentToken.value = !showAgentToken.value },
        'show-password': () => { showPassword.value = true },
      }
    default:
      return {}
  }
})
watch(logged, active => {
  updateRouterAuthState(authResolved.value, active)
  if (authResolved.value) void routeView.replace({ name: active ? 'overview' : 'login' })
}, { flush: 'sync' })
const { connected: eventStreamConnected } = useLiveUpdates({
  active: logged,
  view,
  refreshInterval,
  refresh,
  onMonitoring: applyMonitoringUpdate,
  onAuthExpired: () => {
    error.value = t('登录已过期，请重新登录')
    logged.value = false
  },
})
watch(view, loadCurrentView)
onMounted(async()=>{
  document.documentElement.lang=locale.value
  await checkAuth()
  updateRouterAuthState(true, logged.value)
  authResolved.value=true
  const destination = logged.value && route.name !== 'login' ? String(route.name) : logged.value ? 'overview' : 'login'
  await routeView.replace({ name: destination })
})
</script>
<template>
  <RouterView v-slot="{ Component }">
  <AuthView v-if="!authResolved" :initialized="initialized" :busy="busy" :error="error" :setup="setup" :login="login" :submit-setup="submitSetup" :submit-login="submitLogin" :theme="theme" :toggle-theme="toggleTheme" :locale="locale" :toggle-locale="toggleLocale" />
  <component :is="Component" v-else-if="view==='login'" v-bind="authProps" />
  <div v-else-if="logged" class="layout"><aside class="sidebar"><div class="brand"><div class="brand-mark">◉</div><div><b>AVENLYTIC</b><small>{{t("基础设施监控")}}</small></div></div><div class="nav-group"><div class="nav-caption">{{t("监控")}}</div><button v-for="item in [{id:'overview',icon:'◫',name:'总览'},{id:'hosts',icon:'▣',name:'服务器'},{id:'containers',icon:'◧',name:'Docker 容器'},{id:'alerts',icon:'⚑',name:'告警中心'}]" :key="item.id" class="nav" :class="{active:view===item.id}" @click="navigateToView(item.id)"><span>{{item.icon}}</span>{{t(item.name)}}<i v-if="item.id==='alerts'&&summary.alerts">{{summary.alerts}}</i></button></div><div class="nav-group"><div class="nav-caption">{{t("配置管理")}}</div><button v-for="item in [{id:'rules',icon:'⌁',name:'告警规则'},{id:'channels',icon:'↗',name:'通知通道'},{id:'settings',icon:'⚙',name:'系统设置'}]" :key="item.id" class="nav" :class="{active:view===item.id}" @click="navigateToView(item.id)"><span>{{item.icon}}</span>{{t(item.name)}}</button></div><div class="side-bottom"><span class="dot"></span>{{t("主机运行状态一目了然")}}<small></small></div></aside>
  <main class="main"><header class="topbar"><div><div class="crumb">AVENLYTIC <span>/</span> {{pageTitle}}</div><h1>{{pageTitle}}</h1><p>{{t("实时掌握基础设施运行状态与告警情况。")}}</p></div><div class="top-actions"><button class="appearance-toggle" @click="toggleTheme" :aria-label="t(theme==='light'?'切换深色主题':'切换浅色主题')" :aria-pressed="theme==='dark'">{{theme==='light'?t('深色'):t('浅色')}}</button><button class="appearance-toggle" @click="toggleLocale">{{locale==='zh-CN'?'EN':'中文'}}</button><label class="refresh-control">{{t("断线轮询兜底")}} <DropdownSelect :model-value="refreshInterval" :options="refreshOptions" @update:model-value="setRefreshInterval(Number($event))" /></label><span v-if="eventStreamConnected" class="live"><i></i> {{t("实时推送已连接")}}</span><span v-else-if="refreshInterval>0" class="live manual"><i></i> {{t("SSE 断开，轮询兜底中")}}</span><span v-else class="live manual"><i></i> {{t("实时连接断开")}}</span><button class="icon-button" @click="refresh" :title="t('刷新')">↻</button><button class="user-chip" @click="showPassword=true"><span class="avatar">{{username.slice(0,1).toUpperCase()}}</span><span>{{username}}</span></button><button class="logout" @click="logout">{{t("退出")}}</button></div></header>
    <Transition name="page" mode="out-in">
      <div :key="view" class="page-transition-shell">
        <component
          :is="Component"
          v-bind="pageProps"
          v-on="pageListeners"
        />
      </div>
    </Transition>
  </main>
  <AlertRuleModal :show-rule="showRule" :rule-form="ruleForm" :channels="channels" :metric-types="metricTypes" :save-rule="saveRule" @close="showRule=false" />
  <NotificationChannelModal :show-channel="showChannel" :channel-form="channelForm" :channel-types="channelTypes" :channel-fields="channelFields" :save-channel="saveChannel" :test-channel="testChannel" @close="closeChannel" />
  <HostDetailsModal :selected-host="selectedHost" :pct="pct" :fmt-bytes="fmtBytes" :process-state="processState" @close="selectedHost=null" />
  <PasswordModal :show-password="showPassword" :password-form="passwordForm" :change-password="changePassword" @close="showPassword=false" />
  <ToastNotice :toast="toast" :toast-level="toastLevel" :pause-toast="pauseToast" :resume-toast="resumeToast" @close="clearToast" /></div>
  </RouterView>
</template>
