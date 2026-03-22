import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import {
  RefreshCw, Save, Trash2,
  ChevronsLeft, ChevronLeft, ChevronRight, ChevronsRight,
} from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { IconButton } from '@/components/IconButton'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import * as api from '@/lib/api'

interface HistoryItem {
  id: number
  created_at: string
  method: string
  endpoint: string
  status_code: number
  request: string
  request_ip: string
}

interface Props {
  categoryId: number
  historyLimit: number
}

export function ApiTab({ categoryId, historyLimit: initialLimit }: Props) {
  const { t } = useTranslation()
  const baseUrl = window.location.origin
  const [history, setHistory] = useState<HistoryItem[]>([])
  const [limit, setLimit] = useState(initialLimit)
  const [saving, setSaving] = useState(false)
  const [page, setPage] = useState(1)
  const [rowsPerPage, setRowsPerPage] = useState(25)
  const [totalRecords, setTotalRecords] = useState(0)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmTitle, setConfirmTitle] = useState('')
  const [confirmDesc, setConfirmDesc] = useState('')
  const [confirmAction, setConfirmAction] = useState<() => void>(() => {})

  const loadHistory = useCallback(async (p = 1, perPage = rowsPerPage) => {
    try {
      const res = await api.getAPICallHistory(categoryId, p, perPage)
      setHistory(res.data?.data || [])
      setTotalRecords(res.data?.total || 0)
    } catch { setHistory([]); setTotalRecords(0) }
  }, [categoryId, rowsPerPage])

  useEffect(() => { loadHistory() }, [loadHistory])

  const totalPages = Math.max(1, Math.ceil(totalRecords / rowsPerPage))
  const toggleSelect = (id: number) => setSelected((prev) => { const n = new Set(prev); if (n.has(id)) n.delete(id); else n.add(id); return n })

  const handleSaveLimit = async () => {
    setSaving(true)
    try { await api.updateApiHistoryLimit(categoryId, limit); toast.success(t('api.historySaved')) }
    catch (e: any) { toast.error(e.message) }
    setSaving(false)
  }

  const handleDeleteSelected = () => {
    if (!selected.size) return
    setConfirmTitle(t('common.deleteSelected'))
    setConfirmDesc(t('common.confirmDeleteSelected', { count: selected.size }))
    setConfirmAction(() => async () => {
      setConfirmOpen(false)
      try {
        await api.deleteAPICallHistory(categoryId, Array.from(selected))
        toast.success(t('common.deleted', { count: selected.size }))
        setSelected(new Set())
        const newPage = Math.min(page, Math.max(1, Math.ceil((totalRecords - selected.size) / rowsPerPage)))
        setPage(newPage)
        await loadHistory(newPage, rowsPerPage)
      } catch (e: any) { toast.error(e.response?.data?.error || e.message) }
    })
    setConfirmOpen(true)
  }

  const handleClearAll = () => {
    setConfirmTitle(t('common.clearAll'))
    setConfirmDesc(t('common.confirmClear'))
    setConfirmAction(() => async () => {
      setConfirmOpen(false)
      try {
        await api.clearAPICallHistory(categoryId)
        toast.success(t('common.cleared'))
        setSelected(new Set())
        setPage(1)
        await loadHistory(1, rowsPerPage)
      } catch (e: any) { toast.error(e.response?.data?.error || e.message) }
    })
    setConfirmOpen(true)
  }

  return (
    <div className="flex flex-col gap-4">
      {/* API Examples */}
      <div>
        <h3 className="text-sm font-semibold mb-2">{t('api.examples')}</h3>
        <div className="flex flex-col gap-3">
          <ApiExample title={t('api.addAccount')} code={`curl -X POST ${baseUrl}/api/accounts \\
  -H "X-Passkey: YOUR_PASSKEY" \\
  -H "Content-Type: application/json" \\
  -d '{"category_id": ${categoryId}, "data": "{\\"username\\": \\"user\\", \\"password\\": \\"pass\\"}"}'`}
            response={`{"id":1,"category_id":${categoryId},"used":false,"banned":false,"data":"{\\"username\\":\\"user\\",\\"password\\":\\"pass\\"}","created_at":"...","updated_at":"..."}`} />
          <ApiExample title={t('api.getAccount')} code={`curl -X POST ${baseUrl}/api/accounts/fetch \\
  -H "X-Passkey: YOUR_PASSKEY" \\
  -H "Content-Type: application/json" \\
  -d '{"category_id": ${categoryId}, "count": 1}'`}
            response={`[{"id":1,"category_id":${categoryId},"used":true,"banned":false,"data":"...","created_at":"...","updated_at":"..."}]`} />
          <ApiExample title={t('api.fetchAccountRandom')} code={`curl -X POST ${baseUrl}/api/accounts/fetch \\
  -H "X-Passkey: YOUR_PASSKEY" \\
  -H "Content-Type: application/json" \\
  -d '{"category_id": ${categoryId}, "count": 5, "order": "random"}'`}
            response={`[{"id":42,"category_id":${categoryId},"used":true,"banned":false,"data":"...","created_at":"...","updated_at":"..."},...]`} />
          <ApiExample title={t('api.fetchAccountType')} code={`curl -X POST ${baseUrl}/api/accounts/fetch \\
  -H "X-Passkey: YOUR_PASSKEY" \\
  -H "Content-Type: application/json" \\
  -d '{"category_id": ${categoryId}, "count": 10, "account_type": ["available", "used"], "mark_as_used": false}'`}
            response={`[{"id":1,"category_id":${categoryId},"used":false,"banned":false,"data":"...","created_at":"...","updated_at":"..."},...]`} />
          <ApiExample title={t('api.fetchAccountTime')} code={`curl -X POST ${baseUrl}/api/accounts/fetch \\
  -H "X-Passkey: YOUR_PASSKEY" \\
  -H "Content-Type: application/json" \\
  -d '{"category_id": ${categoryId}, "count": 20, "account_type": "available", "order": "random", "created_after": "2025-01-01T00:00:00Z", "updated_before": "2025-06-01T00:00:00Z"}'`}
            response={`[{"id":7,"category_id":${categoryId},"used":true,"banned":false,"data":"...","created_at":"2025-03-15T10:00:00Z","updated_at":"2025-03-15T10:00:00Z"},...]`} />
          <ApiExample title={t('api.updateAccountData')} code={`curl -X PUT ${baseUrl}/api/accounts/ACCOUNT_ID \\
  -H "X-Passkey: YOUR_PASSKEY" \\
  -H "Content-Type: application/json" \\
  -d '{"data": "new_data", "banned": true}'`}
            response={`{"id":1,"category_id":${categoryId},"used":false,"banned":true,"data":"new_data","created_at":"...","updated_at":"..."}`} />
          <ApiExample title={t('api.markBanned')} code={`curl -X PUT ${baseUrl}/api/accounts/batch/update \\
  -H "X-Passkey: YOUR_PASSKEY" \\
  -H "Content-Type: application/json" \\
  -d '{"ids": [1, 2, 3], "banned": true}'`}
            response={`{"message":"updated"}`} />
        </div>
      </div>

      {/* Call history */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>{t('api.callHistory')}</CardTitle>
            <div className="flex items-center gap-1.5">
              {selected.size > 0 && (
                <Button variant="destructive" size="sm" onClick={handleDeleteSelected}>
                  <Trash2 className="h-3 w-3" /> {t('common.deleteSelected')}
                </Button>
              )}
              {totalRecords > 0 && (
                <Button variant="ghost" size="sm" className="text-[var(--danger)]" onClick={handleClearAll}>
                  <Trash2 className="h-3 w-3" /> {t('common.clearAll')}
                </Button>
              )}
              <label className="text-xs text-[var(--muted-foreground)]" htmlFor="api-history-limit">{t('api.historyLimit')}:</label>
              <Input id="api-history-limit" type="number" min={1} max={10000} value={limit} onChange={(e) => setLimit(Number(e.target.value))} className="w-16 text-xs" />
              <Button variant="outline" size="sm" onClick={handleSaveLimit} loading={saving}>
                <Save className="h-3 w-3" /> {t('common.save')}
              </Button>
              <IconButton label={t('common.refresh')} onClick={() => loadHistory(page, rowsPerPage)}>
                <RefreshCw className="h-3.5 w-3.5" />
              </IconButton>
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-auto">
            <table className="w-full text-[13px]" aria-label={t('api.callHistory')}>
              <thead>
                <tr className="border-b border-[var(--border)] bg-[var(--muted)]">
                  <th className="w-10 p-2.5">
                    <Checkbox
                      checked={history.length > 0 && history.every((h) => selected.has(h.id))}
                      onCheckedChange={() => {
                        if (history.every((h) => selected.has(h.id))) setSelected(new Set())
                        else setSelected(new Set(history.map((h) => h.id)))
                      }}
                      aria-label={t('common.selectAll')}
                    />
                  </th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)] w-40">{t('api.time')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)] w-16">{t('api.method')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('api.endpoint')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)] w-16">{t('common.status')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('api.request')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('api.requestIp')}</th>
                </tr>
              </thead>
              <tbody>
                {history.length === 0 ? (
                  <tr><td colSpan={7} className="p-6 text-center text-sm text-[var(--muted-foreground)]">{t('api.noHistory')}</td></tr>
                ) : history.map((item) => (
                  <tr key={item.id} className="border-b border-[var(--border)] hover:bg-[var(--muted)] transition-colors">
                    <td className="p-2.5">
                      <Checkbox checked={selected.has(item.id)} onCheckedChange={() => toggleSelect(item.id)} />
                    </td>
                    <td className="p-2.5 text-xs">{new Date(item.created_at).toLocaleString()}</td>
                    <td className="p-2.5"><Badge variant="secondary">{item.method}</Badge></td>
                    <td className="p-2.5 font-mono text-xs truncate max-w-[200px]">{item.endpoint}</td>
                    <td className="p-2.5"><Badge variant={item.status_code < 400 ? 'success' : 'danger'}>{item.status_code}</Badge></td>
                    <td className="p-2.5 max-w-[200px]"><code className="text-xs break-all">{item.request}</code></td>
                    <td className="p-2.5"><code className="text-xs">{item.request_ip}</code></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {totalRecords > 0 && (
            <nav className="flex items-center justify-between px-3 py-2 border-t border-[var(--border)] text-xs" aria-label={t('common.pagination')}>
              <div className="flex items-center gap-2 text-[var(--muted-foreground)]">
                <span aria-live="polite">{t('common.totalCount', { count: totalRecords })}</span>
                <select
                  value={rowsPerPage}
                  onChange={(e) => { const n = Number(e.target.value); setRowsPerPage(n); setPage(1); setSelected(new Set()); loadHistory(1, n) }}
                  className="h-6 rounded border border-[var(--border)] bg-[var(--card)] px-1.5 text-xs text-[var(--foreground)]"
                  aria-label={t('common.rowsPerPage')}
                >
                  {[10, 25, 50, 100].map((n) => <option key={n} value={n}>{n}/page</option>)}
                </select>
              </div>
              <div className="flex items-center gap-0.5">
                <IconButton label={t('common.firstPage')} disabled={page <= 1} onClick={() => { setPage(1); setSelected(new Set()); loadHistory(1, rowsPerPage) }}>
                  <ChevronsLeft className="h-3.5 w-3.5" />
                </IconButton>
                <IconButton label={t('common.prevPage')} disabled={page <= 1} onClick={() => { const p = page - 1; setPage(p); setSelected(new Set()); loadHistory(p, rowsPerPage) }}>
                  <ChevronLeft className="h-3.5 w-3.5" />
                </IconButton>
                <span className="px-2 tabular-nums" aria-current="page">{page} / {totalPages}</span>
                <IconButton label={t('common.nextPage')} disabled={page >= totalPages} onClick={() => { const p = page + 1; setPage(p); setSelected(new Set()); loadHistory(p, rowsPerPage) }}>
                  <ChevronRight className="h-3.5 w-3.5" />
                </IconButton>
                <IconButton label={t('common.lastPage')} disabled={page >= totalPages} onClick={() => { setPage(totalPages); setSelected(new Set()); loadHistory(totalPages, rowsPerPage) }}>
                  <ChevronsRight className="h-3.5 w-3.5" />
                </IconButton>
              </div>
            </nav>
          )}
        </CardContent>
      </Card>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={confirmTitle}
        description={confirmDesc}
        onConfirm={() => { confirmAction() }}
      />
    </div>
  )
}

function ApiExample({ title, code, response }: { title: string; code: string; response?: string }) {
  return (
    <div>
      <div className="text-xs font-medium text-[var(--muted-foreground)] mb-1">{title}</div>
      <pre className="text-xs bg-[var(--muted)] p-3 rounded-lg overflow-x-auto font-mono leading-relaxed text-[var(--foreground)]">
        <code>{code}</code>
      </pre>
      {response && (
        <pre className="text-xs bg-[var(--muted)] p-2.5 mt-1 rounded-lg overflow-x-auto font-mono leading-relaxed text-[var(--muted-foreground)]">
          <code><span className="text-[var(--success)]">→</span> {response}</code>
        </pre>
      )}
    </div>
  )
}