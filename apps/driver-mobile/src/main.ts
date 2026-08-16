import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { IonicVue } from '@ionic/vue'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'

import '@ionic/vue/css/core.css'
import '@ionic/vue/css/normalize.css'
import '@ionic/vue/css/structure.css'
import '@ionic/vue/css/typography.css'
import '@ionic/vue/css/padding.css'
import '@ionic/vue/css/float-elements.css'
import '@ionic/vue/css/text-alignment.css'
import '@ionic/vue/css/text-transformation.css'
import '@ionic/vue/css/flex-utils.css'
import '@ionic/vue/css/display.css'
import './theme/variables.css'

async function bootstrapNetworkListeners() {
  const { useNetworkStore } = await import('./stores/network')
  const network = useNetworkStore()
  window.addEventListener('online', () => network.setOnline(true))
  window.addEventListener('offline', () => network.setOnline(false))

  try {
    const { Network } = await import('@capacitor/network')
    const status = await Network.getStatus()
    network.setOnline(status.connected)
    await Network.addListener('networkStatusChange', (event) => {
      network.setOnline(event.connected)
    })
  } catch {
    // Capacitor unavailable in pure web dev — browser events above are enough.
  }
}

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(IonicVue)
app.use(router)
app.use(i18n)

bootstrapNetworkListeners().finally(() => {
  router.isReady().then(() => app.mount('#app'))
})
