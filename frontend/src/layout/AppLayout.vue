<template>
    <div class="layout-wrapper" :class="containerClass">
        <AppTopbar />
        <AppSidebar />
        <div class="layout-main-container">
            <div class="layout-main">
                <router-view />
            </div>
        </div>
        <div class="layout-mask animate-fadein"></div>
    </div>
    <Toast />
    <ConfirmDialog />
</template>

<script setup>
import { useLayout } from './composables/layout'
import { computed, ref, watch } from 'vue'
import AppTopbar from './AppTopbar.vue'
import AppSidebar from './AppSidebar.vue'
import Toast from 'primevue/toast'
import ConfirmDialog from 'primevue/confirmdialog'

const { layoutConfig, layoutState, isSidebarActive } = useLayout()

const outsideClickListener = ref(null)

watch(isSidebarActive, (newVal) => {
    if (newVal) {
        bindOutsideClickListener()
    } else {
        unbindOutsideClickListener()
    }
})

const containerClass = computed(() => ({
    'layout-overlay': layoutConfig.menuMode === 'overlay',
    'layout-static': layoutConfig.menuMode === 'static',
    'layout-static-inactive': layoutState.staticMenuDesktopInactive && layoutConfig.menuMode === 'static',
    'layout-overlay-active': layoutState.overlayMenuActive,
    'layout-mobile-active': layoutState.staticMenuMobileActive
}))

function bindOutsideClickListener() {
    if (!outsideClickListener.value) {
        outsideClickListener.value = (event) => {
            if (isOutsideClicked(event)) {
                layoutState.overlayMenuActive = false
                layoutState.staticMenuMobileActive = false
                layoutState.menuHoverActive = false
            }
        }
        document.addEventListener('click', outsideClickListener.value)
    }
}

function unbindOutsideClickListener() {
    if (outsideClickListener.value) {
        document.removeEventListener('click', outsideClickListener)
        outsideClickListener.value = null
    }
}

function isOutsideClicked(event) {
    const sidebarEl = document.querySelector('.layout-sidebar')
    const topbarEl = document.querySelector('.layout-menu-button')
    return !(sidebarEl?.isSameNode(event.target) || sidebarEl?.contains(event.target) || topbarEl?.isSameNode(event.target) || topbarEl?.contains(event.target))
}
</script>
