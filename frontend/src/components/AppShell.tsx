import { useState } from 'react'
import { Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Menu } from 'lucide-react'
import { Sidebar } from '@/components/Sidebar'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { Button } from '@/components/ui/button'

export function AppShell() {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const { t } = useTranslation()

  return (
    <div className="min-h-screen grid grid-cols-1 lg:grid-cols-[240px_minmax(0,1fr)]">
      <Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />
      <main className="min-h-screen">
        {/* Mobile topbar */}
        <div className="sticky top-0 z-30 flex items-center gap-3 px-4 py-2.5 bg-[var(--background)] border-b border-[var(--border)] lg:hidden">
          <Button variant="ghost" size="icon" onClick={() => setSidebarOpen(true)} className="h-7 w-7" aria-label={t('sidebar.openMenu')}>
            <Menu className="h-4 w-4" />
          </Button>
          <span className="font-semibold text-sm">{t('appName')}</span>
        </div>
        <div className="p-4 lg:p-5 max-w-[1200px] mx-auto">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </div>
      </main>
    </div>
  )
}
