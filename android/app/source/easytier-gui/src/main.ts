import Aura from '@primeuix/themes/aura';
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'

import { createRouter, createWebHistory } from 'vue-router/auto'
import { routes } from 'vue-router/auto-routes'
import App from '~/App.vue'
import EasyTierFrontendLib, { I18nUtils } from 'easytier-frontend-lib'

import { getAutoLaunchStatusAsync, loadAutoLaunchStatusAsync } from './modules/auto_launch'
import '~/styles.css'
import 'easytier-frontend-lib/style.css'

if (import.meta.env.PROD) {
  document.addEventListener('keydown', (event) => {
    if (
      event.key === 'F5'
      || (event.ctrlKey && event.key === 'r')
      || (event.metaKey && event.key === 'r')
    ) {
      event.preventDefault()
    }
  })

  document.addEventListener('contextmenu', (event) => {
    event.preventDefault()
  })
}

function showFatal(msg: string) {
  let el = document.getElementById('fatal-error')
  if (!el) {
    el = document.createElement('pre')
    el.id = 'fatal-error'
    el.style.cssText =
      'position:fixed;left:0;top:0;right:0;z-index:99999;background:#400;color:#fff;padding:12px;white-space:pre-wrap;font-size:12px;max-height:60%;overflow:auto'
    document.body.appendChild(el)
  }
  el.textContent = (el.textContent ? el.textContent + '\n' : '') + msg
}

window.addEventListener('error', (e: any) => showFatal('error: ' + (e?.message || e)))
window.addEventListener('unhandledrejection', (e: any) => showFatal('promise: ' + (e?.reason?.message || e?.reason)))

async function main() {
  try {
    await I18nUtils.loadLanguageAsync(localStorage.getItem('lang') || 'en')
  try {
    await loadAutoLaunchStatusAsync(getAutoLaunchStatusAsync())
  } catch (e: any) {
    // auto-launch is desktop-only on mobile; ignore "Window API not available" etc.
    console.warn('auto-launch init skipped:', e?.message || e)
  }

  const app = createApp(App)

  const router = createRouter({
    history: createWebHistory(),
    routes,
  })

  app.use(router)
  app.use(createPinia())
  app.use(EasyTierFrontendLib)
  // app.use(i18n, { useScope: 'global' })
  app.use(PrimeVue, {
    theme: {
      preset: Aura,
      options: {
        prefix: 'p',
        darkModeSelector: 'system',
        cssLayer: {
          name: 'primevue',
          order: 'tailwind-base, primevue, tailwind-utilities',
        },
      },
    },
  })
  app.use(ToastService as any)
  app.config.errorHandler = (err: any) => showFatal('vue: ' + (err?.message || err))
  app.mount('#app')
  } catch (e: any) {
    showFatal('init: ' + (e?.message || e))
  }
}

main()
