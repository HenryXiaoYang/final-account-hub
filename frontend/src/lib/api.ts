import axios from 'axios'

const api = axios.create({ baseURL: '/api' })

api.interceptors.request.use((config) => {
  const passkey = localStorage.getItem('passkey')
  if (passkey) {
    config.headers['X-Passkey'] = passkey
  }
  return config
})

// Category
export const createCategory = (name: string) => api.post('/categories', { name })
export const getCategories = () => api.get('/categories')
export const getCategory = (id: number | string) => api.get(`/categories/${id}`)
export const deleteCategory = (id: number | string) => api.delete(`/categories/${id}`)
export const updateValidationScript = (
  id: number | string,
  validation_script: string,
  validation_concurrency: number,
  validation_cron: string,
  validation_enabled?: boolean,
  validation_scope?: string,
) => api.put(`/categories/${id}/validation-script`, { validation_script, validation_concurrency, validation_cron, validation_enabled, validation_scope })
export const testValidationScript = (categoryId: number | string, script: string, test_account: string) =>
  api.post(`/categories/${categoryId}/test-validation`, { script, test_account })
export const getValidationRuns = (id: number | string, page = 1, limit = 20) =>
  api.get(`/categories/${id}/validation-runs?page=${page}&limit=${limit}`)
export const deleteValidationRuns = (categoryId: number | string, ids: number[]) =>
  api.delete(`/categories/${categoryId}/validation-runs`, { data: { ids } })
export const runValidationNow = (id: number | string) => api.post(`/categories/${id}/run-validation`)
export const stopValidation = (id: number | string) => api.post(`/categories/${id}/stop-validation`)
export const getValidationRunLog = (runId: number | string, offset = 0, limit = 100) =>
  api.get(`/validation-runs/${runId}/log?offset=${offset}&limit=${limit}`)
export const updateValidationHistoryLimit = (categoryId: number | string, validation_history_limit: number) =>
  api.put(`/categories/${categoryId}/validation-history-limit`, { validation_history_limit })
export const updateApiHistoryLimit = (categoryId: number | string, api_history_limit: number) =>
  api.put(`/categories/${categoryId}/api-history-limit`, { api_history_limit })

// Packages
export const getUVPackages = (categoryId: number | string) => api.get(`/categories/${categoryId}/packages`)
export const installUVPackage = (categoryId: number | string, pkg: string) =>
  api.post(`/categories/${categoryId}/packages/install`, { package: pkg })
export const uninstallUVPackage = (categoryId: number | string, pkg: string) =>
  api.post(`/categories/${categoryId}/packages/uninstall`, { package: pkg })
export const installRequirements = (categoryId: number | string, file: File) => {
  const formData = new FormData()
  formData.append('file', file)
  return api.post(`/categories/${categoryId}/packages/requirements`, formData)
}

// Accounts
export const addAccount = (category_id: number, data: string) =>
  api.post('/accounts', { category_id, data })
export const addAccountsBulk = (category_id: number, data: string[]) =>
  api.post('/accounts/bulk', { category_id, data })
export const getAccounts = (category_id: number | string, page = 1, limit = 100, signal?: AbortSignal) =>
  api.get(`/accounts/${category_id}?page=${page}&limit=${limit}`, { signal })
export interface FetchAccountsParams {
  category_id: number
  count: number
  order?: 'sequential' | 'random'
  account_type?: string | string[]
  mark_as_used?: boolean
  created_after?: string
  created_before?: string
  updated_after?: string
  updated_before?: string
}
export const fetchAccounts = (params: FetchAccountsParams) =>
  api.post('/accounts/fetch', params)
export const updateAccount = (id: number, fields: { data?: string; used?: boolean; banned?: boolean }) =>
  api.put(`/accounts/${id}`, fields)
export const batchUpdateAccounts = (ids: number[], status: Record<string, boolean>) =>
  api.put('/accounts/batch/update', { ids, ...status })
export const deleteAccountsByIds = (ids: number[]) =>
  api.delete('/accounts/by-ids', { data: { ids } })
export const getAccountStats = (category_id: number | string) =>
  api.get(`/accounts/${category_id}/stats`)
export const getSnapshots = (category_id: number | string, granularity = '1d') =>
  api.get(`/accounts/${category_id}/snapshots?granularity=${granularity}`)
export const getGlobalStats = () => api.get('/stats')
export const getGlobalSnapshots = (granularity = '1d') =>
  api.get(`/snapshots?granularity=${granularity}`)
export const getCategoriesOverview = () => api.get('/categories/overview')
export const getRecentValidationRuns = (limit = 10) =>
  api.get(`/validation-runs/recent?limit=${limit}`)
export const getAPICallFrequency = (hours = 24) =>
  api.get(`/history/frequency?hours=${hours}`)
export const getAPICallHistory = (categoryId: number | string, page = 1, limit = 50) =>
  api.get(`/categories/${categoryId}/history?page=${page}&limit=${limit}`)
export const deleteAPICallHistory = (categoryId: number | string, ids: number[]) =>
  api.delete(`/categories/${categoryId}/history`, { data: { ids } })
export const clearAPICallHistory = (categoryId: number | string) =>
  api.delete(`/categories/${categoryId}/history/all`)

// SSE streaming delete
export const deleteAccountsStream = (
  category_id: number,
  used: boolean,
  banned: boolean,
  onProgress?: (data: { deleted: number; total: number }) => void,
): Promise<{ deleted: number; total: number }> => {
  return new Promise((resolve, reject) => {
    const passkey = localStorage.getItem('passkey')
    fetch('/api/accounts', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json', 'X-Passkey': passkey || '' },
      body: JSON.stringify({ category_id, used, banned }),
    })
      .then((res) => {
        if (res.headers.get('content-type')?.includes('text/event-stream')) {
          const reader = res.body!.getReader()
          const decoder = new TextDecoder()
          let buffer = ''
          const read = (): void => {
            reader.read().then(({ done, value }) => {
              if (done) return resolve({ deleted: 0, total: 0 })
              buffer += decoder.decode(value, { stream: true })
              const lines = buffer.split('\n')
              buffer = lines.pop()!
              for (let i = 0; i < lines.length; i++) {
                const line = lines[i]
                if (line.startsWith('data:')) {
                  const data = JSON.parse(line.slice(5))
                  onProgress?.(data)
                }
                if (line.startsWith('event: done')) {
                  const dataLine = lines[i + 1]
                  if (dataLine?.startsWith('data:')) resolve(JSON.parse(dataLine.slice(5)))
                }
              }
              read()
            }).catch(reject)
          }
          read()
        } else {
          res.json().then(resolve).catch(reject)
        }
      })
      .catch(reject)
  })
}
