import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('../views/HomeView.vue'),
    },
    {
      path: '/client/:id',
      name: 'client',
      component: () => import('../views/ClientView.vue'),
    },
  ],
})

export default router
