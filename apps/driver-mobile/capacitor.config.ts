import type { CapacitorConfig } from '@capacitor/cli'

const config: CapacitorConfig = {
  appId: 'com.freightplatform.driver.pilot',
  appName: 'Freight Driver',
  webDir: 'dist',
  server: {
    androidScheme: 'https',
  },
}

export default config
