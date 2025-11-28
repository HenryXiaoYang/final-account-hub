import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '../layout/AppLayout.vue'

const routes = [
  { path: '/', redirect: '/login' },
  {
    path: '/login',
    component: () => import('../views/Login.vue')
  },
  {
    path: '/dashboard',
    component: AppLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        component: () => import('../views/Dashboard.vue')
      },
      {
        path: ':categoryId',
        component: () => import('../views/Dashboard.vue')
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const passkey = localStorage.getItem('passkey')
  if (to.meta.requiresAuth && !passkey) {
    next('/login')
  } else {
    next()
  }
})

export default router
