import { createRouter, createWebHistory } from '@ionic/vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/shipments' },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/pages/LoginPage.vue'),
      meta: { guest: true },
    },
    {
      path: '/shipments',
      name: 'shipments',
      component: () => import('@/pages/ShipmentsPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/shipments/:shipmentId',
      name: 'shipment-detail',
      component: () => import('@/pages/ShipmentDetailPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/shipments/:shipmentId/delay',
      name: 'report-delay',
      component: () => import('@/pages/ReportDelayPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/shipments/:shipmentId/problem',
      name: 'report-problem',
      component: () => import('@/pages/ReportProblemPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/result',
      name: 'submission-result',
      component: () => import('@/pages/SubmissionResultPage.vue'),
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.restored) {
    await auth.restoreSession()
  }

  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  if (to.meta.guest && auth.isAuthenticated) {
    return { name: 'shipments' }
  }

  return true
})

export default router
