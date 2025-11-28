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
  deleteCategory: (id) => api.delete(`/categories/${id}`),

  addAccount: (category_id, data) => api.post('/accounts', { category_id: Number(category_id), data: String(data) }),
  getAccounts: (category_id) => api.get(`/accounts/${category_id}`),
  fetchAccounts: (category_id, count) => api.post('/accounts/fetch', { category_id, count }),
  updateAccounts: (ids, status) => api.put('/accounts/update', { ids: Array.isArray(ids) ? [...ids] : [ids], ...status }),
  deleteAccounts: (category_id, used, banned) => api.delete('/accounts', { data: { category_id, used, banned } }),
  getAccountStats: (category_id) => api.get(`/accounts/${category_id}/stats`),
  getGlobalStats: () => api.get('/stats')
}
