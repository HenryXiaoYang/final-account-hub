import { useState, useEffect, useCallback, useRef, lazy, Suspense } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Play, Save, Square, FileText, RefreshCw, Trash2, Upload, Loader2,
  ChevronsLeft, ChevronLeft, ChevronRight, ChevronsRight,
} from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Switch } from '@/components/ui/switch'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { IconButton } from '@/components/IconButton'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { useTheme } from '@/hooks/use-theme'
import * as api from '@/lib/api'

const MonacoEditor = lazy(() => import('@monaco-editor/react'))

interface Props {
  categoryId: number
  initialScript: string
  initialConcurrency: number
  initialCron: string
  initialHistoryLimit: number
  initialValidationEnabled: boolean
  initialValidationScope: string
}

interface ValidationRun {
  id: number
  status: string
  started_at: string
  finished_at: string | null
  total_count: number
  processed_count: number
  used_count: number
  banned_count: number
}

const defaultScript = `def validate(account: str) -> tuple[bool, bool]:
    # Return (used, banned)
    return False, False`

const helperExample = `def validate(account: str) -> tuple[bool, bool]:
    refreshed = refresh_token(account)
    if refreshed != account:
        update_account(data=refreshed)
    return False, False`

const SCOPE_OPTIONS = ['available', 'used', 'banned'] as const

export function ValidationTab({
  categoryId, initialScript, initialConcurrency, initialCron, initialHistoryLimit,
  initialValidationEnabled, initialValidationScope,
}: Props) {
  const { t } = useTranslation()
  const { isDark } = useTheme()

  const [script, setScript] = useState(initialScript || defaultScript)
  const [concurrency, setConcurrency] = useState(initialConcurrency || 1)
  const [cron, setCron] = useState(initialCron || '0 0 * * *')
  const [historyLimit, setHistoryLimit] = useState(initialHistoryLimit || 50)
  const [validationEnabled, setValidationEnabled] = useState(initialValidationEnabled)
  const [scope, setScope] = useState<Set<string>>(() => new Set(initialValidationScope.split(',').filter(Boolean)))
  const [runs, setRuns] = useState<ValidationRun[]>([])
  const [runsPage, setRunsPage] = useState(1)
  const [runsPerPage, setRunsPerPage] = useState(10)
  const [totalRuns, setTotalRuns] = useState(0)
  const [selectedRuns, setSelectedRuns] = useState<Set<number>>(new Set())
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmTitle, setConfirmTitle] = useState('')
  const [confirmDesc, setConfirmDesc] = useState('')
  const [confirmAction, setConfirmAction] = useState<() => void>(() => {})
  const [testAccount, setTestAccount] = useState('')
  const [testResult, setTestResult] = useState<{ success: boolean; used?: boolean; banned?: boolean; updated_data?: string; error?: string } | null>(null)
  const [testLoading, setTestLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [savingHistoryLimit, setSavingHistoryLimit] = useState(false)

  const [logOpen, setLogOpen] = useState(false)
  const [runLog, setRunLog] = useState<string[]>([])
  const [logRunId, setLogRunId] = useState<number | null>(null)
  const [logOffset, setLogOffset] = useState(0)
  const [logHasMore, setLogHasMore] = useState(false)
  const [logLoading, setLogLoading] = useState(false)
  const logRef = useRef<HTMLDivElement>(null)

  const [packages, setPackages] = useState<{ name: string; version: string }[]>([])
  const [newPkg, setNewPkg] = useState('')
  const [pkgLoading, setPkgLoading] = useState(false)
  const [selectedPkgs, setSelectedPkgs] = useState<Set<string>>(new Set())
  const fileInputRef = useRef<HTMLInputElement>(null)
  const pollingRef = useRef(false)

  const loadRuns = useCallback(async (p = 1, limit = runsPerPage) => {
    try {
      const res = await api.getValidationRuns(categoryId, p, limit)
      const data = res.data?.data || []
      setRuns(data)
      setTotalRuns(res.data?.total || 0)
      return data
    } catch { return [] }
  }, [categoryId, runsPerPage])

  const loadPackages = useCallback(async () => {
    try { const res = await api.getUVPackages(categoryId); setPackages(Array.isArray(res.data) ? res.data : []) }
    catch { setPackages([]) }
  }, [categoryId])

  useEffect(() => {
    loadRuns(1, runsPerPage).then((r: ValidationRun[]) => { if (r.some((run) => run.status === 'running' || run.status === 'stopping')) startPolling() })
    loadPackages()
    return () => { pollingRef.current = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [categoryId])

  const startPolling = () => {
    if (pollingRef.current) return
    pollingRef.current = true
    const poll = async () => {
      if (!pollingRef.current) return
      try {
        const res = await api.getValidationRuns(categoryId, runsPage, runsPerPage)
        const data = res.data?.data || []
        setRuns(data)
        setTotalRuns(res.data?.total || 0)
        if (data.some((r: ValidationRun) => r.status === 'running' || r.status === 'stopping')) setTimeout(poll, 3000)
        else pollingRef.current = false
      } catch { pollingRef.current = false }
    }
    setTimeout(poll, 1000)
  }

  const toggleScope = (value: string) => {
    setScope((prev) => {
      const next = new Set(prev)
      if (next.has(value)) { if (next.size > 1) next.delete(value) }
      else next.add(value)
      return next
    })
  }

  const handleSave = async () => {
    if (scope.size === 0) { toast.error(t('validation.scopeRequired')); return }
    setSaving(true)
    try {
      await api.updateValidationScript(
        categoryId, script, concurrency, cron,
        validationEnabled, Array.from(scope).join(','),
      )
      toast.success(t('validation.scriptSaved'))
    } catch (e: any) {
      toast.error(e.response?.data?.error || e.message)
    }
    setSaving(false)
  }

  const handleSaveHistoryLimit = async () => {
    setSavingHistoryLimit(true)
    try {
      await api.updateValidationHistoryLimit(categoryId, historyLimit)
      toast.success(t('validation.historyLimitSaved'))
    } catch (e: any) {
      toast.error(e.response?.data?.error || e.message)
    }
    setSavingHistoryLimit(false)
  }

  const handleRunNow = async () => {
    if (!validationEnabled) { toast.error(t('validation.validationDisabled')); return }
    try {
      await api.updateValidationScript(
        categoryId, script, concurrency, cron,
        validationEnabled, Array.from(scope).join(','),
      )
      await api.runValidationNow(categoryId)
      toast.info(t('validation.validationStarted'))
      startPolling()
      setTimeout(() => loadRuns(runsPage, runsPerPage), 500)
    } catch (e: any) { toast.error(e.response?.data?.error || e.message) }
  }
  const handleStop = async () => {
    try { await api.stopValidation(categoryId); toast.info(t('validation.validationStopped')); setTimeout(() => loadRuns(runsPage, runsPerPage), 500) }
    catch (e: any) { toast.error(e.message) }
  }
  const handleTest = async () => {
    if (!script || !testAccount || !categoryId) return
    setTestLoading(true); setTestResult(null)
    try { const res = await api.testValidationScript(categoryId, script, testAccount); setTestResult(res.data) }
    catch (e: any) { setTestResult({ success: false, error: e.message }) }
    setTestLoading(false)
  }

  const showRunLog = async (runId: number) => { setLogRunId(runId); setLogOffset(0); setRunLog([]); setLogHasMore(true); setLogOpen(true); await loadMoreLog(runId, 0) }
  const loadMoreLog = async (runId: number, offset: number) => {
    if (logLoading) return; setLogLoading(true)
    const prevHeight = logRef.current?.scrollHeight || 0
    try {
      const res = await api.getValidationRunLog(runId, offset)
      setRunLog((prev) => [...res.data.lines, ...prev]); setLogOffset(offset + res.data.lines.length); setLogHasMore(res.data.has_more)
      setTimeout(() => { if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight - prevHeight }, 0)
    } catch { /* ignore */ }
    setLogLoading(false)
  }
  const onLogScroll = () => { if (logRef.current && logRef.current.scrollTop < 50 && logHasMore && logRunId) loadMoreLog(logRunId, logOffset) }

  const handleInstallPkg = async () => {
    if (!newPkg.trim() || pkgLoading) return; setPkgLoading(true)
    try { const res = await api.installUVPackage(categoryId, newPkg.trim()); if (res.data.success) { toast.success(`${t('packages.installed')}: ${newPkg}`); setNewPkg(''); await loadPackages() } else toast.error(res.data.output || t('packages.failed')) }
    catch (e: any) { toast.error(e.message) }
    setPkgLoading(false)
  }
  const handleUploadReq = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]; if (!file) return; setPkgLoading(true)
    try { const res = await api.installRequirements(categoryId, file); if (res.data.success) { toast.success(t('packages.requirementsInstalled')); await loadPackages() } else toast.error(res.data.output || t('packages.failed')) }
    catch (e: any) { toast.error(e.message) }
    setPkgLoading(false); if (fileInputRef.current) fileInputRef.current.value = ''
  }
  const handleUninstallSelected = async () => {
    if (!selectedPkgs.size) return
    for (const name of selectedPkgs) { try { await api.uninstallUVPackage(categoryId, name) } catch { /* ignore */ } }
    toast.success(t('packages.uninstalled', { count: selectedPkgs.size })); setSelectedPkgs(new Set()); await loadPackages()
  }
  const togglePkg = (name: string) => setSelectedPkgs((prev) => { const n = new Set(prev); if (n.has(name)) n.delete(name); else n.add(name); return n })

  const runsTotalPages = Math.max(1, Math.ceil(totalRuns / runsPerPage))
  const selectableRuns = runs.filter((r) => r.status !== 'running' && r.status !== 'stopping')
  const toggleRunSelect = (id: number) => setSelectedRuns((prev) => { const n = new Set(prev); if (n.has(id)) n.delete(id); else n.add(id); return n })
  const handleDeleteRuns = async () => {
    if (!selectedRuns.size) return
    setConfirmTitle(t('common.deleteSelected'))
    setConfirmDesc(t('common.confirmDeleteSelected', { count: selectedRuns.size }))
    setConfirmAction(() => async () => {
      setConfirmOpen(false)
      try {
        await api.deleteValidationRuns(categoryId, Array.from(selectedRuns))
        toast.success(t('common.deleted', { count: selectedRuns.size }))
        setSelectedRuns(new Set())
        const newPage = Math.min(runsPage, Math.max(1, Math.ceil((totalRuns - selectedRuns.size) / runsPerPage)))
        setRunsPage(newPage)
        await loadRuns(newPage, runsPerPage)
      } catch (e: any) { toast.error(e.response?.data?.error || e.message) }
    })
    setConfirmOpen(true)
  }

  return (
    <div className="flex flex-col gap-4">
      {/* ── Script & Settings Card ── */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2.5">
            <Switch
              id="validation-enabled"
              checked={validationEnabled}
              onCheckedChange={async (checked) => {
                setValidationEnabled(checked)
                try {
                  await api.updateValidationScript(
                    categoryId, script, concurrency, cron,
                    checked, Array.from(scope).join(','),
                  )
                } catch (e: any) {
                  setValidationEnabled(!checked)
                  toast.error(e.response?.data?.error || e.message)
                }
              }}
            />
            <label htmlFor="validation-enabled" className="text-sm font-semibold cursor-pointer select-none leading-none">
              {t('validation.enableValidation')}
            </label>
          </div>
        </CardHeader>
        <CardContent>
          <div className={`flex flex-col gap-3 transition-opacity ${!validationEnabled ? 'opacity-40 pointer-events-none select-none' : ''}`}>
            {/* Script description */}
            <p className="text-xs text-[var(--muted-foreground)] leading-relaxed">
              {t('validation.scriptDesc')}
              <code className="text-xs bg-[var(--muted)] px-1.5 py-0.5 rounded ml-1">{t('validation.scriptPrompt')}</code>
            </p>

            <div className="rounded-xl border border-[var(--border)] bg-[var(--muted)]/50 p-3">
              <div className="space-y-1.5 text-xs">
                <p className="font-medium text-[var(--foreground)]">{t('validation.helperTitle')}</p>
                <p className="text-[var(--muted-foreground)]">{t('validation.helperDesc')}</p>
                <p className="text-[var(--muted-foreground)]">{t('validation.helperRuleCurrent')}</p>
                <p className="text-[var(--muted-foreground)]">{t('validation.helperRuleDuplicate')}</p>
                <p className="text-[var(--muted-foreground)]">{t('validation.helperRuleTest')}</p>
              </div>
              <div className="mt-3 space-y-1.5">
                <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-[var(--muted-foreground)]">{t('validation.helperExample')}</p>
                <pre className="overflow-x-auto rounded-lg border border-[var(--border)] bg-[var(--card)] px-3 py-2 text-[11px] leading-relaxed text-[var(--foreground)]">
                  <code>{helperExample}</code>
                </pre>
              </div>
            </div>

            {/* Monaco editor */}
            <div className="border border-[var(--border)] rounded-lg overflow-hidden" style={{ height: 280 }}>
              <Suspense fallback={<div className="flex items-center justify-center h-full"><Loader2 className="h-5 w-5 animate-spin text-[var(--muted-foreground)]" /></div>}>
                <MonacoEditor
                  height="100%"
                  language="python"
                  theme={isDark ? 'vs-dark' : 'light'}
                  value={script}
                  onChange={(v) => setScript(v || '')}
                  options={{ minimap: { enabled: false }, fontSize: 13, scrollBeyondLastLine: false, quickSuggestions: true, suggestOnTriggerCharacters: true, padding: { top: 10 } }}
                />
              </Suspense>
            </div>

            {/* Settings row */}
            <div className="flex items-center gap-3 flex-wrap">
              <div className="flex items-center gap-1.5">
                <label className="text-xs text-[var(--muted-foreground)]" htmlFor="cron-input">{t('validation.cron')}:</label>
                <Input id="cron-input" value={cron} onChange={(e) => setCron(e.target.value)} placeholder="0 0 * * *" className="w-28 text-xs" />
              </div>
              <div className="flex items-center gap-1.5">
                <label className="text-xs text-[var(--muted-foreground)]" htmlFor="concurrency-input">{t('validation.concurrency')}:</label>
                <Input id="concurrency-input" type="number" min={1} max={100} value={concurrency} onChange={(e) => setConcurrency(Number(e.target.value))} className="w-16 text-xs" />
              </div>
              <div className="h-4 w-px bg-[var(--border)]" />
              <div className="flex items-center gap-2">
                <span className="text-xs text-[var(--muted-foreground)]">{t('validation.scope')}:</span>
                {SCOPE_OPTIONS.map((opt) => (
                  <label key={opt} className="flex items-center gap-1 cursor-pointer">
                    <Checkbox
                      checked={scope.has(opt)}
                      onCheckedChange={() => toggleScope(opt)}
                      aria-label={t(`validation.scope${opt.charAt(0).toUpperCase() + opt.slice(1)}` as any)}
                    />
                    <span className="text-xs">{t(`validation.scope${opt.charAt(0).toUpperCase() + opt.slice(1)}` as any)}</span>
                  </label>
                ))}
              </div>
              <div className="flex-1" />
              <div className="flex items-center gap-1.5">
                <Button variant="outline" size="sm" onClick={handleRunNow} disabled={!validationEnabled}>
                  <Play className="h-3 w-3" /> {t('validation.runNow')}
                </Button>
                <Button size="sm" onClick={handleSave} loading={saving}>
                  <Save className="h-3 w-3" /> {t('common.save')}
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── Test Script Card ── */}
      <Card>
        <CardHeader>
          <CardTitle>{t('validation.testScript')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex gap-2 items-end">
            <div className="flex-1">
              <label className="text-xs text-[var(--muted-foreground)] block mb-1" htmlFor="test-account-input">{t('validation.testAccount')}:</label>
              <Input id="test-account-input" value={testAccount} onChange={(e) => setTestAccount(e.target.value)} placeholder={t('validation.testPlaceholder')} />
            </div>
            <Button size="sm" onClick={handleTest} loading={testLoading} disabled={!testAccount.trim()}>
              <Play className="h-3 w-3" /> {t('validation.test')}
            </Button>
          </div>
          {testResult && (
            <div className={`mt-3 p-2.5 rounded-lg border text-xs ${testResult.success ? 'border-[var(--success)] bg-[var(--success)]/15' : 'border-[var(--danger)] bg-[var(--danger)]/15'}`} role="status">
              {testResult.success ? (
                <div className="space-y-1">
                  <div><span className="font-medium">{t('validation.result')}:</span> used={String(testResult.used)}, banned={String(testResult.banned)}</div>
                  {testResult.updated_data !== undefined && (
                    <div><span className="font-medium">{t('accounts.data')}:</span> <code>{testResult.updated_data}</code></div>
                  )}
                </div>
              ) : (
                <div><span className="font-medium">{t('common.error')}:</span><pre className="mt-1 whitespace-pre-wrap break-words">{testResult.error}</pre></div>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* ── Run History Card ── */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>{t('validation.runHistory')}</CardTitle>
            <div className="flex items-center gap-1.5">
              {selectedRuns.size > 0 && (
                <Button variant="destructive" size="sm" onClick={handleDeleteRuns}>
                  <Trash2 className="h-3 w-3" /> {t('common.deleteSelected')}
                </Button>
              )}
              <label className="text-xs font-normal text-[var(--muted-foreground)]" htmlFor="history-limit-input">{t('validation.historyLimit')}:</label>
              <Input
                id="history-limit-input"
                type="number"
                min={1}
                max={10000}
                value={historyLimit}
                onChange={(e) => setHistoryLimit(Number(e.target.value))}
                className="w-16 text-xs"
              />
              <Button variant="outline" size="sm" onClick={handleSaveHistoryLimit} loading={savingHistoryLimit}>
                <Save className="h-3 w-3" /> {t('common.save')}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-auto">
            <table className="w-full text-[13px]" aria-label={t('validation.runHistory')}>
              <thead>
                <tr className="border-b border-[var(--border)] bg-[var(--muted)]">
                  <th className="w-10 p-2.5">
                    <Checkbox
                      checked={selectableRuns.length > 0 && selectableRuns.every((r) => selectedRuns.has(r.id))}
                      onCheckedChange={() => {
                        if (selectableRuns.every((r) => selectedRuns.has(r.id))) setSelectedRuns(new Set())
                        else setSelectedRuns(new Set(selectableRuns.map((r) => r.id)))
                      }}
                      aria-label={t('common.selectAll')}
                    />
                  </th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('validation.started')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('common.status')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('dashboard.total')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('dashboard.used')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('dashboard.banned')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('validation.finished')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody>
                {runs.length === 0 ? (
                  <tr><td colSpan={8} className="p-6 text-center text-sm text-[var(--muted-foreground)]">{t('validation.noRuns')}</td></tr>
                ) : runs.map((run) => (
                  <tr key={run.id} className="border-b border-[var(--border)] hover:bg-[var(--muted)] transition-colors">
                    <td className="p-2.5">
                      <Checkbox
                        checked={selectedRuns.has(run.id)}
                        onCheckedChange={() => toggleRunSelect(run.id)}
                        disabled={run.status === 'running' || run.status === 'stopping'}
                      />
                    </td>
                    <td className="p-2.5">{new Date(run.started_at).toLocaleString()}</td>
                    <td className="p-2.5">
                      {run.status === 'running'
                        ? <Badge variant="warning">{run.processed_count}/{run.total_count}</Badge>
                        : run.status === 'stopping'
                          ? <Badge variant="warning">{t('validation.stopping')}</Badge>
                          : <Badge variant={run.status === 'success' ? 'success' : 'danger'}>{run.status}</Badge>}
                    </td>
                    <td className="p-2.5 tabular-nums">{run.total_count}</td>
                    <td className="p-2.5 tabular-nums">{run.used_count}</td>
                    <td className="p-2.5 tabular-nums">{run.banned_count}</td>
                    <td className="p-2.5">{run.finished_at ? new Date(run.finished_at).toLocaleString() : '-'}</td>
                    <td className="p-2.5">
                      <div className="flex items-center gap-0.5">
                        <IconButton label={t('validation.log')} onClick={() => showRunLog(run.id)}>
                          <FileText className="h-3.5 w-3.5" />
                        </IconButton>
                        {run.status === 'running' && (
                          <IconButton label={t('common.stop')} className="text-[var(--danger)]" onClick={handleStop}>
                            <Square className="h-3.5 w-3.5" />
                          </IconButton>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {totalRuns > 0 && (
            <nav className="flex items-center justify-between px-3 py-2 border-t border-[var(--border)] text-xs" aria-label={t('common.pagination')}>
              <div className="flex items-center gap-2 text-[var(--muted-foreground)]">
                <span aria-live="polite">{t('common.totalCount', { count: totalRuns })}</span>
                <select
                  value={runsPerPage}
                  onChange={(e) => { const n = Number(e.target.value); setRunsPerPage(n); setRunsPage(1); setSelectedRuns(new Set()); loadRuns(1, n) }}
                  className="h-6 rounded border border-[var(--border)] bg-[var(--card)] px-1.5 text-xs text-[var(--foreground)]"
                  aria-label={t('common.rowsPerPage')}
                >
                  {[10, 25, 50, 100].map((n) => <option key={n} value={n}>{n}/page</option>)}
                </select>
              </div>
              <div className="flex items-center gap-0.5">
                <IconButton label={t('common.firstPage')} disabled={runsPage <= 1} onClick={() => { setRunsPage(1); setSelectedRuns(new Set()); loadRuns(1, runsPerPage) }}>
                  <ChevronsLeft className="h-3.5 w-3.5" />
                </IconButton>
                <IconButton label={t('common.prevPage')} disabled={runsPage <= 1} onClick={() => { const p = runsPage - 1; setRunsPage(p); setSelectedRuns(new Set()); loadRuns(p, runsPerPage) }}>
                  <ChevronLeft className="h-3.5 w-3.5" />
                </IconButton>
                <span className="px-2 tabular-nums" aria-current="page">{runsPage} / {runsTotalPages}</span>
                <IconButton label={t('common.nextPage')} disabled={runsPage >= runsTotalPages} onClick={() => { const p = runsPage + 1; setRunsPage(p); setSelectedRuns(new Set()); loadRuns(p, runsPerPage) }}>
                  <ChevronRight className="h-3.5 w-3.5" />
                </IconButton>
                <IconButton label={t('common.lastPage')} disabled={runsPage >= runsTotalPages} onClick={() => { setRunsPage(runsTotalPages); setSelectedRuns(new Set()); loadRuns(runsTotalPages, runsPerPage) }}>
                  <ChevronsRight className="h-3.5 w-3.5" />
                </IconButton>
              </div>
            </nav>
          )}
        </CardContent>
      </Card>

      {/* ── Python Packages Card ── */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>{t('packages.title')}</CardTitle>
            <div className="flex gap-1.5">
              {selectedPkgs.size > 0 && (
                <Button variant="destructive" size="sm" onClick={handleUninstallSelected}>
                  <Trash2 className="h-3 w-3" /> {t('packages.deleteSelected')}
                </Button>
              )}
              <IconButton label={t('common.refresh')} onClick={loadPackages}>
                <RefreshCw className="h-3.5 w-3.5" />
              </IconButton>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-1.5 mb-3 font-mono text-xs bg-[var(--muted)] p-2 rounded-lg">
            <span className="text-[var(--muted-foreground)]">$</span>
            <span>uv pip install</span>
            <Input
              value={newPkg}
              onChange={(e) => setNewPkg(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleInstallPkg()}
              placeholder="package"
              className="flex-1 font-mono h-6 text-xs"
              aria-label={t('packages.packageName')}
            />
            <Button size="sm" className="h-6 px-2" onClick={handleInstallPkg} loading={pkgLoading} disabled={!newPkg.trim()} aria-label={t('packages.install')}>
              <Play className="h-3 w-3" />
            </Button>
            <span className="text-[var(--muted-foreground)]">|</span>
            <Button variant="outline" size="sm" className="h-6 px-2" onClick={() => fileInputRef.current?.click()}>
              <Upload className="h-3 w-3" /> -r requirements.txt
            </Button>
            <input ref={fileInputRef} type="file" accept=".txt" className="hidden" onChange={handleUploadReq} aria-label={t('packages.uploadRequirements')} />
          </div>
          <div className="overflow-auto rounded-lg border border-[var(--border)]">
            <table className="w-full text-[13px]" aria-label={t('packages.title')}>
              <thead>
                <tr className="border-b border-[var(--border)] bg-[var(--muted)]">
                  <th className="w-10 p-2.5">
                    <Checkbox
                      checked={packages.length > 0 && selectedPkgs.size === packages.length}
                      onCheckedChange={() => {
                        if (selectedPkgs.size === packages.length) setSelectedPkgs(new Set())
                        else setSelectedPkgs(new Set(packages.map((p) => p.name)))
                      }}
                      aria-label={t('packages.selectAll')}
                    />
                  </th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('packages.package')}</th>
                  <th className="p-2.5 text-left font-medium text-[var(--muted-foreground)]">{t('packages.version')}</th>
                </tr>
              </thead>
              <tbody>
                {packages.length === 0 ? (
                  <tr><td colSpan={3} className="p-6 text-center text-sm text-[var(--muted-foreground)]">{t('packages.noPackages')}</td></tr>
                ) : packages.map((pkg) => (
                  <tr key={pkg.name} className="border-b border-[var(--border)] hover:bg-[var(--muted)] transition-colors">
                    <td className="p-2.5">
                      <Checkbox
                        checked={selectedPkgs.has(pkg.name)}
                        onCheckedChange={() => togglePkg(pkg.name)}
                        aria-label={t('packages.selectPackage', { name: pkg.name })}
                      />
                    </td>
                    <td className="p-2.5 font-mono">{pkg.name}</td>
                    <td className="p-2.5 font-mono text-[var(--muted-foreground)]">{pkg.version}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {/* Log dialog */}
      <Dialog open={logOpen} onOpenChange={setLogOpen}>
        <DialogContent className="max-w-4xl max-h-[85vh]">
          <DialogHeader>
            <DialogTitle>{t('validation.runLog')}</DialogTitle>
            <DialogDescription className="sr-only">{t('validation.log')}</DialogDescription>
          </DialogHeader>
          <div ref={logRef} className="text-xs bg-[var(--muted)] p-3 rounded-lg overflow-auto font-mono" style={{ maxHeight: '65vh' }} onScroll={onLogScroll}>
            {logLoading && <div className="text-center py-2 text-[var(--muted-foreground)]">{t('common.loading')}</div>}
            <pre className="whitespace-pre-wrap break-words m-0">{runLog.length ? runLog.join('\n') : t('validation.noLog')}</pre>
          </div>
        </DialogContent>
      </Dialog>

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
