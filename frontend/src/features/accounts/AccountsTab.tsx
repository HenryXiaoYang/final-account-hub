import { useState, useEffect, useCallback, useRef, lazy, Suspense } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Plus, Check, RotateCcw, Ban, Undo2, Trash2, Copy,
  ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight,
} from 'lucide-react'
import axios from 'axios'
import { toast } from 'sonner'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { Progress } from '@/components/ui/progress'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { IconButton } from '@/components/IconButton'
import { useTheme } from '@/hooks/use-theme'
import * as api from '@/lib/api'

const TrendChart = lazy(() => import('@/components/TrendChart'))
import type { ChartRow } from '@/components/TrendChart'

interface Account {
  id: number
  data: string
  used: boolean
  banned: boolean
}

interface Snapshot {
  available: number
  recorded_at: string
}

interface Props {
  categoryId: number
  counts: { total: number; available: number; used: number; banned: number }
  onCountsChange: () => void
}

type Granularity = '1h' | '1d' | '1w'

function formatSnapshotDate(recorded_at: string, granularity: Granularity): string {
  const d = new Date(recorded_at)
  if (granularity === '1h') {
    return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function buildChartRows(snapshots: Snapshot[], granularity: Granularity): ChartRow[] {
  return snapshots.map((s) => ({
    date: formatSnapshotDate(s.recorded_at, granularity),
    Available: s.available,
  }))
}

export function AccountsTab({ categoryId, counts, onCountsChange }: Props) {
  const { t } = useTranslation()
  const { isDark } = useTheme()

  const [accounts, setAccounts] = useState<Account[]>([])
  const [loading, setLoading] = useState(false)
  const [tableLoading, setTableLoading] = useState(false) // non-destructive overlay for pagination
  const [newAccountData, setNewAccountData] = useState('')
  const [adding, setAdding] = useState(false)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [page, setPage] = useState(1)
  const [rowsPerPage, setRowsPerPage] = useState(10)
  const [totalRecords, setTotalRecords] = useState(0)
  const [chartRows, setChartRows] = useState<ChartRow[]>([])
  const [granularity, setGranularity] = useState<Granularity>('1d')
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [deleteProgress, setDeleteProgress] = useState({ deleted: 0, total: 0 })

  // AbortController ref for cancelling in-flight requests
  const abortRef = useRef<AbortController | null>(null)

  // Confirm dialog state
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmTitle, setConfirmTitle] = useState('')
  const [confirmDesc, setConfirmDesc] = useState('')
  const [confirmAction, setConfirmAction] = useState<() => void>(() => {})

  const showConfirm = (title: string, desc: string, action: () => void) => {
    setConfirmTitle(title)
    setConfirmDesc(desc)
    setConfirmAction(() => action)
    setConfirmOpen(true)
  }

  const loadAccounts = useCallback(
    async (p: number, limit: number, isInitial = false) => {
      // Cancel any in-flight request
      abortRef.current?.abort()
      const controller = new AbortController()
      abortRef.current = controller

      // Initial load replaces table; pagination shows overlay without collapsing rows
      if (isInitial) setLoading(true)
      else setTableLoading(true)

      try {
        const res = await api.getAccounts(categoryId, p, limit, controller.signal)
        setAccounts(res.data.data || [])
        setTotalRecords(res.data.total || 0)
        // Sync page from server response (handles clamping)
        const serverPage = res.data.page
        if (serverPage != null && serverPage !== p) {
          setPage(serverPage)
        }
      } catch (e) {
        // Ignore cancelled requests
        if (axios.isCancel(e)) return
      }
      setLoading(false)
      setTableLoading(false)
    },
    [categoryId],
  )

  const loadStats = useCallback(async () => {
    try {
      const res = await api.getSnapshots(categoryId, granularity)
      setChartRows(buildChartRows(res.data || [], granularity))
    } catch { /* ignore */ }
  }, [categoryId, granularity])

  /** Reload both accounts and stats, then notify parent */
  const reloadAll = useCallback(
    async (p: number, limit: number) => {
      await Promise.all([loadAccounts(p, limit), loadStats()])
      onCountsChange()
    },
    [loadAccounts, loadStats, onCountsChange],
  )

  // Category change: reload accounts + chart, reset page & selection
  useEffect(() => {
    setPage(1)
    setSelected(new Set())
    loadAccounts(1, rowsPerPage, true)
    loadStats()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [categoryId])

  // Granularity change: only reload chart data
  useEffect(() => {
    loadStats()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [granularity])

  // Cleanup abort on unmount
  useEffect(() => {
    return () => { abortRef.current?.abort() }
  }, [])

  const handleAdd = async () => {
    if (!newAccountData.trim() || adding) return
    const lines = newAccountData.split('\n').filter((l) => l.trim()).map((l) => l.trim())
    setAdding(true)
    try {
      if (lines.length === 1) {
        await api.addAccount(categoryId, lines[0])
        toast.success(t('accounts.accountAdded'))
      } else {
        const res = await api.addAccountsBulk(categoryId, lines)
        const { count, skipped } = res.data
        if (skipped > 0) toast.warning(t('accounts.duplicatesSkipped', { count, skipped }))
        else toast.success(t('accounts.accountsAdded', { count }))
      }
      setNewAccountData('')
      await reloadAll(page, rowsPerPage)
    } catch (e: any) {
      if (e.response?.status === 409) toast.error(t('accounts.accountExists'))
      else toast.error(e.response?.data?.error || e.message)
    }
    setAdding(false)
  }

  const handleUpdateAccounts = async (ids: number | number[], status: Record<string, boolean>) => {
    try {
      const idArray = Array.isArray(ids) ? ids : [ids]
      await api.batchUpdateAccounts(idArray, status)
      setSelected(new Set())
      await reloadAll(page, rowsPerPage)
    } catch { /* ignore */ }
  }

  const handleDeleteSelected = () => {
    if (!selected.size) return
    showConfirm(
      t('common.confirm'),
      t('accounts.confirmDeleteSelected', { count: selected.size }),
      async () => {
        try {
          await api.deleteAccountsByIds([...selected])
          toast.success(t('accounts.selectedDeleted'))
          setSelected(new Set())
          await reloadAll(page, rowsPerPage)
        } catch { /* ignore */ }
      },
    )
  }

  const handleStreamDelete = (used: boolean, banned: boolean) => {
    const count = used ? counts.used : counts.banned
    if (count === 0) return
    const msg = used ? t('accounts.confirmDeleteUsed', { count }) : t('accounts.confirmDeleteBanned', { count })
    showConfirm(t('common.confirm'), msg, async () => {
      setDeleteProgress({ deleted: 0, total: count })
      setDeleteDialogOpen(true)
      try {
        const result = await api.deleteAccountsStream(categoryId, used, banned, (data) => setDeleteProgress(data))
        const detail = used
          ? t('accounts.usedDeletedCount', { count: result.deleted || 0 })
          : t('accounts.bannedDeletedCount', { count: result.deleted || 0 })
        toast.success(detail)
        setPage(1)
        await reloadAll(1, rowsPerPage)
      } finally { setDeleteDialogOpen(false) }
    })
  }

  const copyData = (text: string) => {
    navigator.clipboard.writeText(text).then(
      () => toast.success(t('common.copied')),
      () => toast.error(t('common.copyFailed')),
    )
  }
  const toggleSelect = (id: number) => setSelected((prev) => { const n = new Set(prev); if (n.has(id)) n.delete(id); else n.add(id); return n })
  const toggleSelectAll = () => { if (selected.size === accounts.length) setSelected(new Set()); else setSelected(new Set(accounts.map((a) => a.id))) }

  const totalPages = Math.max(1, Math.ceil(totalRecords / rowsPerPage))
  const selectedIds = [...selected]
  const chartLabel = t('dashboard.available')

  const granularityOptions: { value: Granularity; label: string }[] = [
    { value: '1h', label: t('dashboard.hourly') },
    { value: '1d', label: t('dashboard.daily') },
    { value: '1w', label: t('dashboard.weekly') },
  ]

  return (
    <div className="flex flex-col gap-4">
      {/* Chart */}
      <div className="bg-[var(--card)] border border-[var(--border)] rounded-xl p-3">
        <div className="flex items-center justify-end gap-1 mb-2">
          {granularityOptions.map((opt) => (
            <button
              key={opt.value}
              onClick={() => setGranularity(opt.value)}
              className={`px-2 py-0.5 text-xs rounded-md transition-colors ${
                granularity === opt.value
                  ? 'bg-[var(--primary)] text-[var(--primary-foreground)]'
                  : 'bg-[var(--muted)] text-[var(--muted-foreground)] hover:bg-[var(--accent)]'
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
        <div className="h-48" role="img" aria-label={t('accounts.statistics')}>
          <Suspense fallback={<div className="h-full flex items-center justify-center text-xs text-[var(--muted-foreground)]">{t('common.loading')}</div>}>
            <TrendChart data={chartRows} isDark={isDark} label={chartLabel} />
          </Suspense>
        </div>
      </div>

      {/* Add accounts */}
      <div className="flex gap-2 items-end">
        <Textarea
          value={newAccountData}
          onChange={(e) => setNewAccountData(e.target.value)}
          placeholder={t('accounts.addPlaceholder')}
          rows={2}
          className="flex-1"
          aria-label={t('accounts.addAccount')}
        />
        <Button onClick={handleAdd} className="shrink-0" loading={adding} disabled={!newAccountData.trim()}>
          <Plus className="h-3.5 w-3.5" />
          {t('accounts.addAccount')}
        </Button>
      </div>

      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-1.5">
        <Button variant="outline" size="sm" disabled={!selected.size} onClick={() => handleUpdateAccounts(selectedIds, { used: true })}>
          <Check className="h-3 w-3" /> {t('accounts.setUsed')}
        </Button>
        <Button variant="outline" size="sm" disabled={!selected.size} onClick={() => handleUpdateAccounts(selectedIds, { used: false, banned: false })}>
          <RotateCcw className="h-3 w-3" /> {t('accounts.setAvailable')}
        </Button>
        <Button variant="outline" size="sm" disabled={!selected.size} onClick={() => handleUpdateAccounts(selectedIds, { banned: true })}>
          <Ban className="h-3 w-3" /> {t('accounts.setBanned')}
        </Button>
        <Button variant="outline" size="sm" disabled={!selected.size} onClick={() => handleUpdateAccounts(selectedIds, { banned: false })}>
          <Undo2 className="h-3 w-3" /> {t('accounts.unban')}
        </Button>
        <div className="flex-1" />
        <Button variant="destructive" size="sm" disabled={!selected.size} onClick={handleDeleteSelected}>
          <Trash2 className="h-3 w-3" /> {t('accounts.deleteSelected')}
        </Button>
        <Button variant="outline" size="sm" disabled={counts.used === 0} onClick={() => handleStreamDelete(true, false)}>
          {t('accounts.deleteUsed')}
        </Button>
        <Button variant="outline" size="sm" disabled={counts.banned === 0} onClick={() => handleStreamDelete(false, true)}>
          {t('accounts.deleteBanned')}
        </Button>
      </div>

      {/* Table */}
      <Card>
        <CardContent className="p-0">
          <div className="overflow-auto relative">
            {/* Pagination overlay — keeps table layout intact, no scroll jump */}
            {tableLoading && (
              <div className="absolute inset-0 z-10 bg-[var(--background)]/60 flex items-center justify-center">
                <span className="text-xs text-[var(--muted-foreground)]">{t('common.loading')}</span>
              </div>
            )}
            <table className="w-full text-[13px]" aria-label={t('accounts.title')}>
              <thead>
                <tr className="border-b border-[var(--border)] bg-[var(--muted)]">
                  <th className="w-10 p-2.5 pl-3">
                    <Checkbox
                      checked={accounts.length > 0 && selected.size === accounts.length}
                      onCheckedChange={toggleSelectAll}
                      aria-label={t('accounts.selectAll')}
                    />
                  </th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)] w-16">{t('accounts.id')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('accounts.data')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)] w-24">{t('common.status')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)] w-28">{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody>
                {loading && accounts.length === 0 ? (
                  <tr><td colSpan={5} className="p-6 text-center text-[var(--muted-foreground)] text-sm">{t('common.loading')}</td></tr>
                ) : accounts.length === 0 ? (
                  <tr><td colSpan={5} className="p-6 text-center text-[var(--muted-foreground)] text-sm">{t('accounts.noAccounts')}</td></tr>
                ) : accounts.map((acc) => (
                  <tr key={acc.id} className={`border-b border-[var(--border)] transition-colors hover:bg-[var(--muted)] ${selected.has(acc.id) ? 'bg-[var(--accent)]' : ''}`}>
                    <td className="p-2.5 pl-3">
                      <Checkbox
                        checked={selected.has(acc.id)}
                        onCheckedChange={() => toggleSelect(acc.id)}
                        aria-label={t('accounts.selectAccount', { id: acc.id })}
                      />
                    </td>
                    <td className="p-2.5 font-mono text-xs text-[var(--muted-foreground)]">{acc.id}</td>
                    <td className="p-2.5 max-w-[400px]">
                      <div className="flex items-center gap-1.5 min-w-0">
                        <code className="text-xs bg-[var(--muted)] px-1.5 py-0.5 rounded truncate flex-1 block min-w-0">{acc.data}</code>
                        <IconButton label={t('common.copy')} className="shrink-0 text-[var(--muted-foreground)]" onClick={() => copyData(acc.data)}>
                          <Copy className="h-3 w-3" />
                        </IconButton>
                      </div>
                    </td>
                    <td className="p-2.5">
                      {acc.banned ? <Badge variant="danger">{t('dashboard.banned')}</Badge>
                        : acc.used ? <Badge variant="warning">{t('dashboard.used')}</Badge>
                        : <Badge variant="success">{t('dashboard.available')}</Badge>}
                    </td>
                    <td className="p-2.5">
                      <div className="flex items-center gap-0.5">
                        {!acc.used ? (
                          <IconButton label={t('accounts.setUsed')} onClick={() => handleUpdateAccounts(acc.id, { used: true })} disabled={acc.banned}>
                            <Check className="h-3.5 w-3.5" />
                          </IconButton>
                        ) : (
                          <IconButton label={t('accounts.setAvailable')} onClick={() => handleUpdateAccounts(acc.id, { used: false, banned: false })}>
                            <RotateCcw className="h-3.5 w-3.5" />
                          </IconButton>
                        )}
                        {!acc.banned ? (
                          <IconButton label={t('accounts.setBanned')} onClick={() => handleUpdateAccounts(acc.id, { banned: true })}>
                            <Ban className="h-3.5 w-3.5" />
                          </IconButton>
                        ) : (
                          <IconButton label={t('accounts.unban')} onClick={() => handleUpdateAccounts(acc.id, { banned: false })}>
                            <Undo2 className="h-3.5 w-3.5" />
                          </IconButton>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          <nav className="flex items-center justify-between px-3 py-2 border-t border-[var(--border)] text-xs" aria-label={t('accounts.pagination')}>
            <div className="flex items-center gap-2 text-[var(--muted-foreground)]">
              <span aria-live="polite">{t('accounts.totalCount', { count: totalRecords })}</span>
              <select
                value={rowsPerPage}
                onChange={(e) => { setRowsPerPage(Number(e.target.value)); setPage(1); loadAccounts(1, Number(e.target.value)) }}
                className="h-6 rounded border border-[var(--border)] bg-[var(--card)] px-1.5 text-xs text-[var(--foreground)]"
                aria-label={t('accounts.rowsPerPage')}
              >
                {[10, 25, 50, 100].map((n) => <option key={n} value={n}>{n}/page</option>)}
              </select>
            </div>
            <div className="flex items-center gap-0.5">
              <IconButton label={t('accounts.firstPage')} disabled={page <= 1} onClick={() => { setPage(1); loadAccounts(1, rowsPerPage) }}>
                <ChevronsLeft className="h-3.5 w-3.5" />
              </IconButton>
              <IconButton label={t('accounts.prevPage')} disabled={page <= 1} onClick={() => { const p = page - 1; setPage(p); loadAccounts(p, rowsPerPage) }}>
                <ChevronLeft className="h-3.5 w-3.5" />
              </IconButton>
              <span className="px-2 tabular-nums" aria-current="page">{page} / {totalPages}</span>
              <IconButton label={t('accounts.nextPage')} disabled={page >= totalPages} onClick={() => { const p = page + 1; setPage(p); loadAccounts(p, rowsPerPage) }}>
                <ChevronRight className="h-3.5 w-3.5" />
              </IconButton>
              <IconButton label={t('accounts.lastPage')} disabled={page >= totalPages} onClick={() => { setPage(totalPages); loadAccounts(totalPages, rowsPerPage) }}>
                <ChevronsRight className="h-3.5 w-3.5" />
              </IconButton>
            </div>
          </nav>
        </CardContent>
      </Card>

      {/* Delete progress dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={(open) => { if (!open) setDeleteDialogOpen(false) }}>
        <DialogContent className="max-w-xs" onPointerDownOutside={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle>{t('accounts.deleting')}</DialogTitle>
            <DialogDescription className="sr-only">
              {t('accounts.deleting')}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2 py-1">
            <Progress value={deleteProgress.total ? Math.round((deleteProgress.deleted / deleteProgress.total) * 100) : 0} />
            <p className="text-center text-xs text-[var(--muted-foreground)] tabular-nums" aria-live="polite">
              {deleteProgress.deleted} / {deleteProgress.total}
            </p>
          </div>
        </DialogContent>
      </Dialog>

      {/* Confirm dialog (replaces window.confirm) */}
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={confirmTitle}
        description={confirmDesc}
        onConfirm={() => { setConfirmOpen(false); confirmAction() }}
      />
    </div>
  )
}
