import { useEffect, useState, useCallback, lazy, Suspense } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { FolderOpen, Activity, Zap } from 'lucide-react'
import {
  BarChart, Bar, XAxis as BarXAxis, YAxis as BarYAxis, CartesianGrid as BarGrid,
  Tooltip as BarTooltip, ResponsiveContainer as BarContainer, Legend,
} from 'recharts'
import { StatBar } from '@/components/StatCard'
import { Badge } from '@/components/ui/badge'
import type { ChartRow } from '@/components/TrendChart'
import * as api from '@/lib/api'
import { useTheme } from '@/hooks/use-theme'

const TrendChart = lazy(() => import('@/components/TrendChart'))

type Granularity = '1h' | '1d' | '1w'

interface Snapshot { available: number; recorded_at: string }
interface CategoryOverview {
  id: number; name: string; total: number; available: number; used: number; banned: number; last_validated_at: string | null
}
interface RecentRun {
  id: number; category_id: number; category_name: string; status: string
  total_count: number; used_count: number; banned_count: number; started_at: string; finished_at: string | null
}
interface FrequencyPoint { hour: string; count: number }

function formatSnapshotDate(recorded_at: string, granularity: Granularity): string {
  const d = new Date(recorded_at)
  if (granularity === '1h') {
    return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function buildChartRows(snapshots: Snapshot[], granularity: Granularity): ChartRow[] {
  return snapshots.map((s) => ({ date: formatSnapshotDate(s.recorded_at, granularity), Available: s.available }))
}

export default function DashboardPage() {
  const { t } = useTranslation()
  const { isDark } = useTheme()
  const navigate = useNavigate()
  const [stats, setStats] = useState<{ categories: number; accounts: { total: number; available: number; used: number; banned: number } }>({
    categories: 0, accounts: { total: 0, available: 0, used: 0, banned: 0 },
  })
  const [chartRows, setChartRows] = useState<ChartRow[]>([])
  const [granularity, setGranularity] = useState<Granularity>('1d')
  const [categories, setCategories] = useState<CategoryOverview[]>([])
  const [recentRuns, setRecentRuns] = useState<RecentRun[]>([])
  const [frequency, setFrequency] = useState<FrequencyPoint[]>([])

  const loadStats = useCallback(() => {
    api.getGlobalStats().then((res) => setStats(res.data)).catch(() => {})
  }, [])

  const loadSnapshots = useCallback(() => {
    api.getGlobalSnapshots(granularity)
      .then((res) => setChartRows(buildChartRows(res.data || [], granularity)))
      .catch(() => {})
  }, [granularity])

  const loadCategories = useCallback(() => {
    api.getCategoriesOverview().then((res) => setCategories(res.data || [])).catch(() => {})
  }, [])

  const loadRecentRuns = useCallback(() => {
    api.getRecentValidationRuns(10).then((res) => setRecentRuns(res.data || [])).catch(() => {})
  }, [])

  const loadFrequency = useCallback(() => {
    api.getAPICallFrequency(24).then((res) => setFrequency(res.data || [])).catch(() => {})
  }, [])

  useEffect(() => { loadStats(); loadCategories(); loadRecentRuns(); loadFrequency() }, [loadStats, loadCategories, loadRecentRuns, loadFrequency])
  useEffect(() => { loadSnapshots() }, [loadSnapshots])

  useEffect(() => {
    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        loadStats(); loadSnapshots(); loadCategories(); loadRecentRuns(); loadFrequency()
      }
    }
    document.addEventListener('visibilitychange', onVisibilityChange)
    return () => document.removeEventListener('visibilitychange', onVisibilityChange)
  }, [loadStats, loadSnapshots, loadCategories, loadRecentRuns, loadFrequency])

  const chartLabel = t('dashboard.available')
  const granularityOptions: { value: Granularity; label: string }[] = [
    { value: '1h', label: t('dashboard.hourly') },
    { value: '1d', label: t('dashboard.daily') },
    { value: '1w', label: t('dashboard.weekly') },
  ]

  const frequencyRows: ChartRow[] = frequency.map((f) => ({
    date: f.hour.slice(11, 16) || f.hour,
    Calls: f.count,
  }))

  const statusVariant = (s: string) => {
    if (s === 'success') return 'success' as const
    if (s === 'running') return 'default' as const
    if (s === 'stopping') return 'warning' as const
    if (s === 'stopped') return 'secondary' as const
    return 'danger' as const
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Header with inline stats */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h1 className="text-lg font-semibold">{t('dashboard.title')}</h1>
        <StatBar items={[
          { label: t('common.categories'), value: stats.categories },
          { label: t('dashboard.available'), value: stats.accounts?.available || 0, color: 'text-[var(--success)]' },
          { label: t('dashboard.used'), value: stats.accounts?.used || 0, color: 'text-[var(--warning)]' },
          { label: t('dashboard.banned'), value: stats.accounts?.banned || 0, color: 'text-[var(--danger)]' },
        ]} />
      </div>

      {/* Trend Chart */}
      <div className="bg-[var(--card)] border border-[var(--border)] rounded-xl p-3">
        <div className="flex items-center justify-end gap-1 mb-2">
          {granularityOptions.map((opt) => (
            <button key={opt.value} onClick={() => setGranularity(opt.value)}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors ${granularity === opt.value ? 'bg-[var(--primary)] text-[var(--primary-foreground)]' : 'bg-[var(--muted)] text-[var(--muted-foreground)] hover:bg-[var(--accent)]'}`}
            >{opt.label}</button>
          ))}
        </div>
        <div className="h-48" role="img" aria-label={t('dashboard.allAccounts')}>
          <Suspense fallback={<div className="h-full flex items-center justify-center text-xs text-[var(--muted-foreground)]">{t('common.loading')}</div>}>
            <TrendChart data={chartRows} isDark={isDark} label={chartLabel} />
          </Suspense>
        </div>
      </div>

      {/* Two-column grid: Categories Overview + Category Distribution */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Categories Overview Table */}
        <div className="bg-[var(--card)] border border-[var(--border)] rounded-xl p-3">
          <h2 className="text-sm font-semibold mb-2">{t('dashboard.categoriesOverview')}</h2>
          {categories.length === 0 ? (
            <div className="flex flex-col items-center gap-1.5 py-6 text-[var(--muted-foreground)]">
              <FolderOpen className="h-5 w-5 text-[var(--primary)]" />
              <p className="text-xs text-center">{t('dashboard.noCategories')}</p>
            </div>
          ) : (
            <div className="overflow-auto">
              <table className="w-full text-[13px]">
                <thead>
                  <tr className="border-b border-[var(--border)] bg-[var(--muted)]">
                    <th className="p-2 text-left font-medium text-[var(--muted-foreground)]">{t('dashboard.category')}</th>
                    <th className="p-2 text-right font-medium text-[var(--muted-foreground)]">{t('dashboard.total')}</th>
                    <th className="p-2 text-right font-medium text-[var(--muted-foreground)]">{t('dashboard.available')}</th>
                    <th className="p-2 text-right font-medium text-[var(--muted-foreground)]">{t('dashboard.used')}</th>
                    <th className="p-2 text-right font-medium text-[var(--muted-foreground)]">{t('dashboard.banned')}</th>
                  </tr>
                </thead>
                <tbody>
                  {categories.map((cat) => (
                    <tr key={cat.id} className="border-b border-[var(--border)] hover:bg-[var(--muted)] cursor-pointer transition-colors" onClick={() => navigate(`/dashboard/${cat.id}`)}>
                      <td className="p-2 font-medium text-[var(--primary)]">{cat.name}</td>
                      <td className="p-2 text-right tabular-nums">{cat.total}</td>
                      <td className="p-2 text-right tabular-nums">{cat.available}</td>
                      <td className={`p-2 text-right tabular-nums ${cat.used > 0 ? 'text-[var(--warning)]' : ''}`}>{cat.used}</td>
                      <td className={`p-2 text-right tabular-nums ${cat.banned > 0 ? 'text-[var(--danger)]' : ''}`}>{cat.banned}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Category Distribution Chart */}
        <div className="bg-[var(--card)] border border-[var(--border)] rounded-xl p-3">
          <h2 className="text-sm font-semibold mb-2">{t('dashboard.categoryDistribution')}</h2>
          {categories.length === 0 ? (
            <div className="flex flex-col items-center gap-1.5 py-6 text-[var(--muted-foreground)]">
              <FolderOpen className="h-5 w-5 text-[var(--primary)]" />
              <p className="text-xs text-center">{t('dashboard.noCategories')}</p>
            </div>
          ) : (
            <div className="h-48">
              <CategoryDistributionChart categories={categories} isDark={isDark} t={t} />
            </div>
          )}
        </div>
      </div>

      {/* Two-column grid: Recent Validations + API Frequency */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Recent Validations */}
        <div className="bg-[var(--card)] border border-[var(--border)] rounded-xl p-3">
          <h2 className="text-sm font-semibold mb-2">{t('dashboard.recentValidations')}</h2>
          {recentRuns.length === 0 ? (
            <div className="flex flex-col items-center gap-1.5 py-6 text-[var(--muted-foreground)]">
              <Activity className="h-5 w-5 text-[var(--success)]" />
              <p className="text-xs text-center">{t('dashboard.noValidations')}</p>
            </div>
          ) : (
            <div className="overflow-auto">
              <table className="w-full text-[13px]">
                <thead>
                  <tr className="border-b border-[var(--border)] bg-[var(--muted)]">
                    <th className="p-2 text-left font-medium text-[var(--muted-foreground)]">{t('dashboard.category')}</th>
                    <th className="p-2 text-left font-medium text-[var(--muted-foreground)]">{t('common.status')}</th>
                    <th className="p-2 text-right font-medium text-[var(--muted-foreground)]">{t('dashboard.total')}</th>
                    <th className="p-2 text-right font-medium text-[var(--muted-foreground)] hidden xl:table-cell">{t('dashboard.used')}</th>
                    <th className="p-2 text-right font-medium text-[var(--muted-foreground)] hidden xl:table-cell">{t('dashboard.banned')}</th>
                    <th className="p-2 text-right font-medium text-[var(--muted-foreground)] hidden xl:table-cell">{t('validation.started')}</th>
                  </tr>
                </thead>
                <tbody>
                  {recentRuns.map((run) => (
                    <tr key={run.id} className="border-b border-[var(--border)] hover:bg-[var(--muted)] cursor-pointer transition-colors" onClick={() => navigate(`/dashboard/${run.category_id}`)}>
                      <td className="p-2 font-medium text-[var(--primary)]">{run.category_name}</td>
                      <td className="p-2"><Badge variant={statusVariant(run.status)}>{run.status}</Badge></td>
                      <td className="p-2 text-right tabular-nums">{run.total_count}</td>
                      <td className={`p-2 text-right tabular-nums hidden xl:table-cell ${run.used_count > 0 ? 'text-[var(--warning)]' : ''}`}>{run.used_count}</td>
                      <td className={`p-2 text-right tabular-nums hidden xl:table-cell ${run.banned_count > 0 ? 'text-[var(--danger)]' : ''}`}>{run.banned_count}</td>
                      <td className="p-2 text-right text-xs text-[var(--muted-foreground)] hidden xl:table-cell">{new Date(run.started_at).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* API Call Frequency */}
        <div className="bg-[var(--card)] border border-[var(--border)] rounded-xl p-3">
          <h2 className="text-sm font-semibold mb-2">{t('dashboard.apiFrequency')}</h2>
          {frequency.length === 0 ? (
            <div className="flex flex-col items-center gap-1.5 py-6 text-[var(--muted-foreground)]">
              <Zap className="h-5 w-5 text-[var(--warning)]" />
              <p className="text-xs text-center">{t('dashboard.noApiCalls')}</p>
            </div>
          ) : (
            <div className="h-48">
              <Suspense fallback={<div className="h-full flex items-center justify-center text-xs text-[var(--muted-foreground)]">{t('common.loading')}</div>}>
                <TrendChart data={frequencyRows} isDark={isDark} label={t('dashboard.calls')} dataKey="Calls" color="#8b5cf6" />
              </Suspense>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function CategoryDistributionChart({ categories, isDark, t }: { categories: CategoryOverview[]; isDark: boolean; t: (key: string) => string }) {
  const gridColor = isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.05)'
  const textColor = isDark ? '#838b9e' : '#5c6578'
  const data = categories.map((c) => ({ name: c.name, [t('dashboard.available')]: c.available, [t('dashboard.used')]: c.used, [t('dashboard.banned')]: c.banned }))

  return (
    <BarContainer width="100%" height="100%">
      <BarChart data={data}>
        <BarGrid strokeDasharray="3 3" stroke={gridColor} />
        <BarXAxis dataKey="name" tick={{ fill: textColor, fontSize: 11 }} />
        <BarYAxis tick={{ fill: textColor, fontSize: 11 }} allowDecimals={false} />
        <BarTooltip contentStyle={{ background: isDark ? '#16181f' : '#fff', border: `1px solid ${isDark ? '#262a36' : '#e2e5eb'}`, borderRadius: 8, fontSize: 12 }} />
        <Legend wrapperStyle={{ fontSize: 11 }} />
        <Bar dataKey={t('dashboard.available')} stackId="a" fill="#16a34a" />
        <Bar dataKey={t('dashboard.used')} stackId="a" fill="#ca8a04" />
        <Bar dataKey={t('dashboard.banned')} stackId="a" fill="#dc2626" />
      </BarChart>
    </BarContainer>
  )
}
