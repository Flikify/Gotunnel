import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from '../api'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      name: 'home',
      component: () => import('../views/HomeView.vue'),
    },
    {
      path: '/clients',
      name: 'clients',
      component: () => import('../views/ClientsView.vue'),
    },
    {
      path: '/client/:id',
      name: 'client',
      component: () => import('../views/ClientView.vue'),
    },
    {
      path: '/client/:id/remote-control',
      name: 'remote-control',
      component: () => import('../views/RemoteControlView.vue'),
      meta: { fullscreen: true },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
    },
  ],
})

// 路由守卫
router.beforeEach((to, _from, next) => {
  const token = getToken()
  if (!to.meta.public && !token) {
    next('/login')
  } else if (to.path === '/login' && token) {
    next('/')
  } else {
    next()
  }
})

export default router
