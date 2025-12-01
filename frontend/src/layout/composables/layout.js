import { computed, reactive } from 'vue'

const savedTheme = localStorage.getItem('darkTheme') === 'true'
if (savedTheme) document.documentElement.classList.add('app-dark')

const layoutConfig = reactive({
    darkTheme: savedTheme,
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
        localStorage.setItem('darkTheme', layoutConfig.darkTheme)
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
