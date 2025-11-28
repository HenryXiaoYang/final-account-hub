import { computed, reactive } from 'vue'

const layoutConfig = reactive({
    darkTheme: false,
    menuMode: 'static'
})

const layoutState = reactive({
    staticMenuDesktopInactive: false,
    overlayMenuActive: false,
    staticMenuMobileActive: false,
    menuHoverActive: false
})

export function useLayout() {
    const toggleDarkMode = () => {
        layoutConfig.darkTheme = !layoutConfig.darkTheme
        document.documentElement.classList.toggle('app-dark')
    }

    const toggleMenu = () => {
        if (layoutConfig.menuMode === 'overlay') {
            layoutState.overlayMenuActive = !layoutState.overlayMenuActive
        }
        if (window.innerWidth > 991) {
            layoutState.staticMenuDesktopInactive = !layoutState.staticMenuDesktopInactive
        } else {
            layoutState.staticMenuMobileActive = !layoutState.staticMenuMobileActive
        }
    }

    const isSidebarActive = computed(() => layoutState.overlayMenuActive || layoutState.staticMenuMobileActive)
    const isDarkTheme = computed(() => layoutConfig.darkTheme)

    return { layoutConfig, layoutState, toggleMenu, isSidebarActive, isDarkTheme, toggleDarkMode }
}
