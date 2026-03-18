import { useState, useEffect, useCallback } from 'react'
import { useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Loader2 } from 'lucide-react'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { StatBar } from '@/components/StatCard'
import { AccountsTab } from '@/features/accounts/AccountsTab'
import { ValidationTab } from '@/features/validation/ValidationTab'
import { ApiTab } from '@/features/api/ApiTab'
import * as api from '@/lib/api'

interface CategoryData {
  id: number
  name: string
  validation_script: string
  validation_concurrency: number
  validation_cron: string
  validation_history_limit: number
  api_history_limit: number
  validation_enabled: boolean
  validation_scope: string
}

export default function CategoryPage() {
  const { t } = useTranslation()
  const { categoryId } = useParams<{ categoryId: string }>()
  const [category, setCategory] = useState<CategoryData | null>(null)
  const [loading, setLoading] = useState(true)
  const [counts, setCounts] = useState({ total: 0, available: 0, used: 0, banned: 0 })
  const [activeTab, setActiveTab] = useState('accounts')

  const loadCategory = useCallback(async () => {
    if (!categoryId) return
    setLoading(true)
    try {
      const [catRes, statsRes] = await Promise.all([
        api.getCategory(categoryId),
        api.getAccountStats(categoryId),
      ])
      setCategory(catRes.data)
      const c = statsRes.data?.counts
      if (c) setCounts({ total: c.total || 0, available: c.available || 0, used: c.used || 0, banned: c.banned || 0 })
    } catch {
      /* ignore */
    }
    setLoading(false)
  }, [categoryId])

  useEffect(() => {
    setActiveTab('accounts')
    loadCategory()
  }, [categoryId, loadCategory])

  const refreshCounts = useCallback(async () => {
    if (!categoryId) return
    try {
      const statsRes = await api.getAccountStats(categoryId)
      const c = statsRes.data?.counts
      if (c) setCounts({ total: c.total || 0, available: c.available || 0, used: c.used || 0, banned: c.banned || 0 })
    } catch {
      /* ignore */
    }
  }, [categoryId])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="h-5 w-5 animate-spin text-[var(--muted-foreground)]" />
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h1 className="text-lg font-semibold">{category?.name || ''}</h1>
        <StatBar items={[
          { label: t('dashboard.total'), value: counts.total },
          { label: t('dashboard.available'), value: counts.available, color: 'text-[var(--success)]' },
          { label: t('dashboard.used'), value: counts.used, color: 'text-[var(--warning)]' },
          { label: t('dashboard.banned'), value: counts.banned, color: 'text-[var(--danger)]' },
        ]} />
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="accounts">
            {t('accounts.title')}
          </TabsTrigger>
          <TabsTrigger value="validation">
            {t('validation.title')}
          </TabsTrigger>
          <TabsTrigger value="api">
            {t('api.title')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="accounts">
          <AccountsTab
            categoryId={Number(categoryId)}
            counts={counts}
            onCountsChange={refreshCounts}
          />
        </TabsContent>

        <TabsContent value="validation">
          {category && (
            <ValidationTab
              categoryId={Number(categoryId)}
              initialScript={category.validation_script}
              initialConcurrency={category.validation_concurrency}
              initialCron={category.validation_cron}
              initialHistoryLimit={category.validation_history_limit}
              initialValidationEnabled={category.validation_enabled ?? true}
              initialValidationScope={category.validation_scope || 'available,used'}
            />
          )}
        </TabsContent>

        <TabsContent value="api">
          <ApiTab
            categoryId={Number(categoryId)}
            historyLimit={category?.api_history_limit || 1000}
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}
