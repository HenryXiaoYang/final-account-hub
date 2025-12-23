import axios from 'axios'

const api = axios.create({
  baseURL: '/api'
})

api.interceptors.request.use(config => {
  const passkey = localStorage.getItem('passkey')
  if (passkey) {
    config.headers['X-Passkey'] = passkey
  }
  return config
})

export default {
  createCategory: (name) => api.post('/categories', { name }),
  getCategories: () => api.get('/categories'),
  getCategory: (id) => api.get(`/categories/${id}`),
  deleteCategory: (id) => api.delete(`/categories/${id}`),
  updateValidationScript: (id, validation_script, validation_concurrency, validation_cron) => api.put(`/categories/${id}/validation-script`, { validation_script, validation_concurrency, validation_cron }),
  testValidationScript: (categoryId, script, test_account) => api.post(`/categories/${categoryId}/test-validation`, { script, test_account }),
  getValidationRuns: (id) => api.get(`/categories/${id}/validation-runs`),
  runValidationNow: (id) => api.post(`/categories/${id}/run-validation`),
  stopValidation: (id) => api.post(`/categories/${id}/stop-validation`),
  getValidationRunLog: (runId, offset = 0, limit = 100) => api.get(`/validation-runs/${runId}/log?offset=${offset}&limit=${limit}`),
  getUVPackages: (categoryId) => api.get(`/categories/${categoryId}/packages`),
  installUVPackage: (categoryId, pkg) => api.post(`/categories/${categoryId}/packages/install`, { package: pkg }),
  uninstallUVPackage: (categoryId, pkg) => api.post(`/categories/${categoryId}/packages/uninstall`, { package: pkg }),
  installRequirements: (categoryId, file) => {
    const formData = new FormData()
    formData.append('file', file)
    return api.post(`/categories/${categoryId}/packages/requirements`, formData)
  },

  addAccount: (category_id, data) => api.post('/accounts', { category_id: Number(category_id), data: String(data) }),
  addAccountsBulk: (category_id, data) => api.post('/accounts/bulk', { category_id: Number(category_id), data }),
  getAccounts: (category_id, page = 1, limit = 100) => api.get(`/accounts/${category_id}?page=${page}&limit=${limit}`),
  fetchAccounts: (category_id, count) => api.post('/accounts/fetch', { category_id, count }),
  updateAccounts: (ids, status) => api.put('/accounts/update', { ids: Array.isArray(ids) ? [...ids] : [ids], ...status }),
  deleteAccountsStream: (category_id, used, banned, onProgress) => {
    return new Promise((resolve, reject) => {
      const passkey = localStorage.getItem('passkey')
      fetch('/api/accounts', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json', 'X-Passkey': passkey },
        body: JSON.stringify({ category_id, used, banned })
      }).then(res => {
        if (res.headers.get('content-type')?.includes('text/event-stream')) {
          const reader = res.body.getReader()
          const decoder = new TextDecoder()
          let buffer = ''
          const read = () => {
            reader.read().then(({ done, value }) => {
              if (done) return resolve({ deleted: 0, total: 0 })
              buffer += decoder.decode(value, { stream: true })
              const lines = buffer.split('\n')
              buffer = lines.pop()
              for (const line of lines) {
                if (line.startsWith('data:')) {
                  const data = JSON.parse(line.slice(5))
                  if (onProgress) onProgress(data)
                }
                if (line.startsWith('event: done')) {
                  const dataLine = lines[lines.indexOf(line) + 1]
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
      }).catch(reject)
    })
  },
  deleteAccountsByIds: (ids) => api.delete('/accounts/by-ids', { data: { ids } }),
  getAccountStats: (category_id) => api.get(`/accounts/${category_id}/stats`),
  getGlobalStats: () => api.get('/stats'),
  getAPICallHistory: (categoryId) => api.get(`/categories/${categoryId}/history`),
  updateHistoryLimit: (categoryId, history_limit) => api.put(`/categories/${categoryId}/history-limit`, { history_limit })
}
