import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import PrimeVue from 'primevue/config'
import { loader } from '@guolao/vue-monaco-editor'
import * as monaco from 'monaco-editor'

loader.config({ monaco })
import Aura from '@primevue/themes/aura'
import { definePreset } from '@primevue/themes'

const GoldPreset = definePreset(Aura, {
    semantic: {
        primary: {
            50: '{amber.50}',
            100: '{amber.100}',
            200: '{amber.200}',
            300: '{amber.300}',
            400: '{amber.400}',
            500: '{amber.500}',
            600: '{amber.600}',
            700: '{amber.700}',
            800: '{amber.800}',
            900: '{amber.900}',
            950: '{amber.950}'
        }
    }
})
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import Tooltip from 'primevue/tooltip'
import './assets/styles.scss'
import './style.css'

const app = createApp(App)
app.use(i18n)
app.use(router)
app.use(PrimeVue, {
    theme: {
        preset: GoldPreset,
        options: {
            darkModeSelector: '.app-dark'
        }
    }
})
app.use(ToastService)
app.use(ConfirmationService)
app.directive('tooltip', Tooltip)
app.mount('#app')
