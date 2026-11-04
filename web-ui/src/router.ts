import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import AlertRulePage from './components/RulesPage.vue'
import AlertsPage from './components/AlertsPage.vue'
import AuthView from './components/AuthView.vue'
import ChannelsPage from './components/ChannelsPage.vue'
import ContainersPage from './components/ContainersPage.vue'
import HostsPage from './components/HostsPage.vue'
import OverviewPage from './components/OverviewPage.vue'
import SettingsPage from './components/SettingsPage.vue'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: AuthView },
  { path: '/', name: 'overview', component: OverviewPage, meta: { requiresAuth: true } },
  { path: '/hosts', name: 'hosts', component: HostsPage, meta: { requiresAuth: true } },
  { path: '/containers', name: 'containers', component: ContainersPage, meta: { requiresAuth: true } },
  { path: '/alerts', name: 'alerts', component: AlertsPage, meta: { requiresAuth: true } },
  { path: '/rules', name: 'rules', component: AlertRulePage, meta: { requiresAuth: true } },
  { path: '/channels', name: 'channels', component: ChannelsPage, meta: { requiresAuth: true } },
  { path: '/settings', name: 'settings', component: SettingsPage, meta: { requiresAuth: true } },
  { path: '/:pathMatch(.*)*', redirect: { name: 'overview' } },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

let authResolved = false
let authenticated = false

export function updateRouterAuthState(resolved: boolean, loggedIn: boolean) {
  authResolved = resolved
  authenticated = loggedIn
}

router.beforeEach(to => {
  if (!authResolved) return true
  if (to.meta.requiresAuth && !authenticated) return { name: 'login', replace: true }
  if (to.name === 'login' && authenticated) return { name: 'overview', replace: true }
  return true
})
