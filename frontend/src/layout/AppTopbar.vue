<template>
    <div class="layout-topbar">
        <div class="layout-topbar-logo-container">
            <button class="layout-menu-button layout-topbar-action" @click="toggleMenu">
                <i class="pi pi-bars"></i>
            </button>
            <router-link to="/dashboard" class="layout-topbar-logo">
                <i class="pi pi-database text-3xl"></i>
                <span>Final Account Hub</span>
            </router-link>
        </div>

        <div class="layout-topbar-actions">
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
import { useLayout } from './composables/layout'
import { useRouter } from 'vue-router'

const { toggleMenu, toggleDarkMode, isDarkTheme } = useLayout()
const router = useRouter()

const logout = () => {
    localStorage.removeItem('passkey')
    router.push('/login')
}
</script>
