import { useState, useEffect, useCallback } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  Home, Plus, Moon, Sun, LogOut, X, FolderOpen, Trash2,
} from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { IconButton } from '@/components/IconButton'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { useAuth } from '@/hooks/use-auth'
import { useTheme } from '@/hooks/use-theme'
import * as api from '@/lib/api'
import { cn } from '@/lib/utils'

interface Category {
  id: number
  name: string
}

interface SidebarProps {
  open: boolean
  onClose: () => void
}

export function Sidebar({ open, onClose }: SidebarProps) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { categoryId } = useParams()
  const { isDark, toggle: toggleTheme } = useTheme()
  const logout = useAuth((s) => s.logout)

  const [categories, setCategories] = useState<Category[]>([])
  const [newName, setNewName] = useState('')
  const [creating, setCreating] = useState(false)

  // Confirm dialog for category deletion
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [pendingDelete, setPendingDelete] = useState<Category | null>(null)

  const loadCategories = useCallback(async () => {
    try {
      const res = await api.getCategories()
      setCategories(res.data || [])
    } catch {
      /* ignore */
    }
  }, [])

  useEffect(() => {
    loadCategories()
  }, [loadCategories])

  const handleCreate = async () => {
    const name = newName.trim()
    if (!name || creating) return
    if (categories.some((c) => c.name.toLowerCase() === name.toLowerCase())) {
      toast.warning(t('sidebar.categoryExists'))
      return
    }
    setCreating(true)
    try {
      await api.createCategory(name)
      toast.success(t('sidebar.categoryCreated'))
      setNewName('')
      await loadCategories()
    } catch {
      toast.error(t('common.error'))
    }
    setCreating(false)
  }

  const requestDelete = (cat: Category) => {
    setPendingDelete(cat)
    setConfirmOpen(true)
  }

  const executeDelete = async () => {
    if (!pendingDelete) return
    setConfirmOpen(false)
    try {
      await api.deleteCategory(pendingDelete.id)
      toast.success(t('sidebar.categoryDeleted'))
      if (String(pendingDelete.id) === categoryId) navigate('/dashboard')
      await loadCategories()
    } catch {
      toast.error(t('common.error'))
    }
    setPendingDelete(null)
  }

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const toggleLocale = () => {
    const next = i18n.language === 'en' ? 'zh' : 'en'
    i18n.changeLanguage(next)
    localStorage.setItem('locale', next)
  }

  return (
    <>
      {/* Mobile overlay */}
      {open && (
        <div className="fixed inset-0 z-40 bg-black/40 lg:hidden" onClick={onClose} aria-hidden="true" />
      )}

      <aside
        className={cn(
          'fixed top-0 left-0 z-50 h-screen w-[240px] flex flex-col gap-3 p-4 border-r border-[var(--border)] overflow-y-auto transition-transform duration-150',
          'bg-[var(--background)]',
          'lg:sticky lg:translate-x-0',
          open ? 'translate-x-0' : '-translate-x-full',
        )}
        aria-label={t('sidebar.navigation')}
      >
        {/* Brand */}
        <div className="flex items-center justify-between px-1 pb-1">
          <span className="font-semibold text-sm truncate">{t('appName')}</span>
          <IconButton label={t('sidebar.closeMenu')} className="lg:hidden" onClick={onClose}>
            <X className="h-3.5 w-3.5" />
          </IconButton>
        </div>

        {/* Home */}
        <Link
          to="/dashboard"
          onClick={onClose}
          className={cn(
            'flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-[13px] font-medium transition-colors no-underline',
            !categoryId
              ? 'bg-[var(--primary)]/10 text-[var(--primary)]'
              : 'text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]',
          )}
        >
          <Home className="h-3.5 w-3.5" />
          {t('common.home')}
        </Link>

        {/* Categories section */}
        <div className="flex-1 min-h-0 flex flex-col gap-1.5">
          <span className="text-[11px] font-semibold uppercase tracking-wider text-[var(--muted-foreground)] px-1">
            {t('common.categories')}
          </span>

          {/* Create input */}
          <div className="flex items-center gap-1">
            <Input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              placeholder={t('sidebar.categoryName')}
              className="h-7 text-xs"
              aria-label={t('sidebar.categoryName')}
            />
            <Button size="sm" className="h-7 w-7 shrink-0 px-0" onClick={handleCreate} disabled={creating || !newName.trim()} aria-label={t('common.confirm')}>
              <Plus className="h-3 w-3" />
            </Button>
          </div>

          {/* Category list */}
          <nav className="flex-1 min-h-0 overflow-y-auto space-y-px" aria-label={t('common.categories')}>
            {categories.length === 0 && (
              <p className="text-xs text-[var(--muted-foreground)] px-2 py-3 text-center">
                {t('sidebar.noCategories')}
              </p>
            )}
            {categories.map((cat) => (
              <Link
                key={cat.id}
                to={`/dashboard/${cat.id}`}
                onClick={onClose}
                className={cn(
                  'group flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[13px] cursor-pointer transition-colors no-underline',
                  String(cat.id) === categoryId
                    ? 'bg-[var(--primary)]/10 text-[var(--primary)]'
                    : 'text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]',
                )}
              >
                <FolderOpen className="h-3.5 w-3.5 shrink-0" />
                <span className="truncate flex-1 font-medium min-w-0">{cat.name}</span>
                <IconButton
                  label={t('common.delete')}
                  tooltip={false}
                  className="opacity-0 group-hover:opacity-100 hover:text-[var(--danger)] transition-opacity"
                  onClick={(e) => {
                    e.preventDefault()
                    e.stopPropagation()
                    requestDelete(cat)
                  }}
                >
                  <Trash2 className="h-3 w-3" />
                </IconButton>
              </Link>
            ))}
          </nav>
        </div>

        {/* Footer */}
        <div className="flex items-center gap-1 pt-2 border-t border-[var(--border)]">
          <IconButton label={isDark ? t('sidebar.lightMode') : t('sidebar.darkMode')} onClick={toggleTheme}>
            {isDark ? <Sun className="h-3.5 w-3.5" /> : <Moon className="h-3.5 w-3.5" />}
          </IconButton>
          <Button variant="ghost" size="sm" onClick={toggleLocale} className="h-7 px-1.5 text-xs text-[var(--muted-foreground)]">
            {i18n.language === 'en' ? 'EN' : '中文'}
          </Button>
          <div className="flex-1" />
          <IconButton label={t('common.logout')} className="text-[var(--muted-foreground)] hover:text-[var(--danger)]" onClick={handleLogout}>
            <LogOut className="h-3.5 w-3.5" />
          </IconButton>
        </div>
      </aside>

      {/* Confirm dialog for category delete */}
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('sidebar.confirmDeleteHeader')}
        description={pendingDelete ? t('sidebar.confirmDelete', { name: pendingDelete.name }) : ''}
        onConfirm={executeDelete}
        confirmText={t('common.delete')}
      />
    </>
  )
}
