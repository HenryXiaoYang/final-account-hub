<template>
    <div class="layout-topbar">
        <div class="layout-topbar-logo-container">
            <button class="layout-menu-button layout-topbar-action" @click="toggleMenu">
                <i class="pi pi-bars"></i>
            </button>
            <router-link to="/dashboard" class="layout-topbar-logo">
                <i class="pi pi-database text-3xl"></i>
                <span>{{ t('appName') }}</span>
            </router-link>
        </div>

        <div class="layout-topbar-actions">
            <Select v-model="currentLocale" :options="localeOptions" optionLabel="label" optionValue="value" @change="changeLocale" class="w-24" />
            <button type="button" class="layout-topbar-action" @click="toggleDarkMode">
                <i :class="['pi', { 'pi-moon': isDarkTheme, 'pi-sun': !isDarkTheme }]"></i>
            </button>
            <button type="button" class="layout-topbar-action" @click="logout">
                <i class="pi pi-sign-out"></i>
            </button>
        </div>
    </div>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useLayout } from './composables/layout'
import { useRouter } from 'vue-router'
import Select from 'primevue/select'

const { t, locale } = useI18n()
const { toggleMenu, toggleDarkMode, isDarkTheme } = useLayout()
const router = useRouter()

const localeOptions = [
    { label: 'EN', value: 'en' },
    { label: '中文', value: 'zh' }
]
const currentLocale = ref(locale.value)

const changeLocale = () => {
    locale.value = currentLocale.value
    localStorage.setItem('locale', currentLocale.value)
}

const logout = () => {
    localStorage.removeItem('passkey')
    router.push('/login')
}
</script>
