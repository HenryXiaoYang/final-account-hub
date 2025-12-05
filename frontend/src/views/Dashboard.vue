<template>
  <div v-if="categoryId">
    <Card>
      <template #title>
        <div class="flex items-center justify-between flex-wrap gap-4">
          <div class="flex items-center gap-2">
            <i class="pi pi-users text-primary"></i>
            {{ categoryName }}
          </div>
          <div class="flex gap-2 flex-wrap">
            <Tag :value="`Total: ${accounts.length}`" severity="contrast" />
            <Tag :value="`Available: ${availableCount}`" severity="success" />
            <Tag :value="`Used: ${usedCount}`" severity="warn" />
            <Tag :value="`Banned: ${bannedCount}`" severity="danger" />
          </div>
        </div>
      </template>
      <template #content>
        <Tabs value="accounts">
          <TabList>
            <Tab value="accounts"><i class="pi pi-users mr-2"></i>Accounts</Tab>
            <Tab value="validation"><i class="pi pi-code mr-2"></i>Validation</Tab>
            <Tab value="api"><i class="pi pi-book mr-2"></i>API</Tab>
          </TabList>
          <TabPanels>
            <!-- Accounts Tab -->
            <TabPanel value="accounts">
              <Card class="mb-4">
                <template #title>Statistics</template>
                <template #content>
                  <Chart type="line" :data="chartData" :options="chartOptions" class="h-64" />
                </template>
              </Card>

              <Card class="mb-4">
                <template #title>Add Account</template>
                <template #content>
                  <Textarea v-model="newAccountData" placeholder="one line = one account" rows="3" class="w-full mb-3" />
                  <Button label="Add Account" icon="pi pi-plus" @click="addAccount" />
                </template>
              </Card>

              <Toolbar class="mb-4">
                <template #start>
                  <Button label="Set Used" icon="pi pi-check" @click="updateAccounts(selectedAccounts, { used: true })" :disabled="!selectedAccounts.length" class="mr-2" />
                  <Button label="Set Available" icon="pi pi-replay" @click="updateAccounts(selectedAccounts, { used: false, banned: false })" :disabled="!selectedAccounts.length" class="mr-2" />
                  <Button label="Set Banned" icon="pi pi-ban" @click="updateAccounts(selectedAccounts, { banned: true })" :disabled="!selectedAccounts.length" class="mr-2" />
                  <Button label="Unban" icon="pi pi-undo" @click="updateAccounts(selectedAccounts, { banned: false })" :disabled="!selectedAccounts.length" />
                </template>
                <template #end>
                  <Button label="Delete Selected" icon="pi pi-trash" severity="danger" @click="confirmDeleteSelected" :disabled="!selectedAccounts.length" class="mr-2" />
                  <Button label="Delete Used" icon="pi pi-trash" outlined @click="confirmDeleteUsed" class="mr-2" />
                  <Button label="Delete Banned" icon="pi pi-ban" outlined @click="confirmDeleteBanned" />
                </template>
              </Toolbar>

              <DataTable v-model:selection="selectedAccountsObj" :value="accounts" dataKey="id" stripedRows paginator :rows="10" :rowsPerPageOptions="[10, 25, 50, 100]">
                <Column selectionMode="multiple" style="width: 3rem" />
                <Column field="id" header="ID" style="width: 80px" />
                <Column field="data" header="Data" style="max-width: 400px">
                  <template #body="{ data }">
                    <div class="flex items-center gap-2">
                      <code v-tooltip.top="data.data.length > 50 ? { value: data.data } : null" class="text-xs bg-surface-100 dark:bg-surface-800 px-2 py-1 rounded block truncate flex-1">{{ data.data }}</code>
                      <Button icon="pi pi-copy" text size="small" @click="copyData(data.data)" />
                    </div>
                  </template>
                </Column>
                <Column field="used" header="Status" style="width: 120px">
                  <template #body="{ data }">
                    <Tag v-if="data.banned" value="Banned" severity="danger" />
                    <Tag v-else-if="data.used" value="Used" severity="warn" />
                    <Tag v-else value="Available" severity="success" />
                  </template>
                </Column>
                <Column header="Actions" style="width: 150px">
                  <template #body="{ data }">
                    <Button v-if="!data.used" icon="pi pi-check" text @click="updateAccounts(data.id, { used: true })" :disabled="data.banned" />
                    <Button v-else icon="pi pi-replay" text @click="updateAccounts(data.id, { used: false, banned: false })" />
                    <Button v-if="!data.banned" icon="pi pi-ban" text @click="updateAccounts(data.id, { banned: true })" />
                    <Button v-else icon="pi pi-undo" text @click="updateAccounts(data.id, { banned: false })" />
                  </template>
                </Column>
              </DataTable>
            </TabPanel>

            <!-- Validation Tab -->
            <TabPanel value="validation">
              <Card class="mb-4">
                <template #title>Validation Script (Python)</template>
                <template #content>
                  <p class="text-sm text-surface-500 mb-3">Define a <code>validate(account: str)</code> function that returns <code>(used: bool, banned: bool)</code>. Only available (non-used, non-banned) accounts are validated.</p>
                  <VueMonacoEditor
                    v-model:value="validationScript"
                    language="python"
                    :theme="isDarkTheme ? 'vs-dark' : 'vs'"
                    :options="{ minimap: { enabled: false }, fontSize: 14, scrollBeyondLastLine: false, quickSuggestions: true, suggestOnTriggerCharacters: true }"
                    class="w-full mb-3 border rounded-lg overflow-hidden"
                    style="height: 300px"
                  />
                  <div class="flex items-center justify-between flex-wrap gap-3">
                    <div class="flex items-center gap-4">
                      <div class="flex items-center gap-2">
                        <label class="text-sm">Cron:</label>
                        <InputText v-model="validationCron" placeholder="0 0 * * *" class="w-32" />
                      </div>
                      <div class="flex items-center gap-2">
                        <label class="text-sm">Concurrency:</label>
                        <InputNumber v-model="validationConcurrency" :min="1" :max="100" class="w-20" />
                      </div>
                    </div>
                    <div class="flex items-center gap-2">
                      <Button label="Run Now" icon="pi pi-play" outlined @click="runValidationNow" />
                      <Button label="Save" icon="pi pi-save" @click="saveValidationScript" />
                    </div>
                  </div>
                </template>
              </Card>
              <Card class="mb-4">
                <template #title>Test Script</template>
                <template #content>
                  <div class="flex gap-3 items-end">
                    <div class="flex-1">
                      <label class="text-sm block mb-1">Test Account Data:</label>
                      <InputText v-model="testAccount" placeholder="Enter account data to test" class="w-full" />
                    </div>
                    <Button label="Test" icon="pi pi-play" @click="testScript" :loading="testLoading" />
                  </div>
                  <div v-if="testResult" class="mt-3 p-3 rounded-lg border-2" :class="testResult.success ? 'border-green-500' : 'border-red-500'">
                    <div v-if="testResult.success" class="text-sm">
                      <span class="font-semibold">Result:</span> used={{ testResult.used }}, banned={{ testResult.banned }}
                    </div>
                    <div v-else class="text-sm">
                      <span class="font-semibold">Error:</span> <pre class="mt-1 text-xs whitespace-pre-wrap">{{ testResult.error }}</pre>
                    </div>
                  </div>
                </template>
              </Card>
              <Card class="mb-4">
                <template #title>Run History</template>
                <template #content>
                  <DataTable :value="validationRuns" stripedRows>
                    <Column field="started_at" header="Started">
                      <template #body="{ data }">{{ new Date(data.started_at).toLocaleString() }}</template>
                    </Column>
                    <Column field="status" header="Status">
                      <template #body="{ data }">
                        <Tag v-if="data.status === 'running'" :value="`${data.processed_count}/${data.total_count}`" severity="warn" />
                        <Tag v-else :value="data.status" :severity="data.status === 'success' ? 'success' : 'danger'" />
                      </template>
                    </Column>
                    <Column field="total_count" header="Total" />
                    <Column field="banned_count" header="Banned" />
                    <Column field="finished_at" header="Finished">
                      <template #body="{ data }">{{ data.finished_at ? new Date(data.finished_at).toLocaleString() : '-' }}</template>
                    </Column>
                    <Column header="Log">
                      <template #body="{ data }">
                        <Button icon="pi pi-file" text size="small" @click="showRunLog(data.id)" />
                      </template>
                    </Column>
                  </DataTable>
                </template>
              </Card>
              <Card>
                <template #title>
                  <div class="flex items-center justify-between">
                    <span>Python Packages (UV)</span>
                    <div class="flex gap-2">
                      <Button v-if="selectedPackages.length" label="Delete Selected" icon="pi pi-trash" severity="danger" size="small" @click="uninstallSelectedPackages" />
                      <Button icon="pi pi-refresh" text size="small" @click="loadPackages" />
                    </div>
                  </div>
                </template>
                <template #content>
                  <div class="flex items-center gap-2 mb-3 font-mono text-sm bg-surface-100 dark:bg-surface-800 p-2 rounded">
                    <span class="text-surface-500">$</span>
                    <span>uv pip install</span>
                    <InputText v-model="newPackage" placeholder="package" class="flex-1 font-mono" size="small" />
                    <Button icon="pi pi-play" @click="installPackage" :loading="packageLoading" size="small" />
                    <span class="text-surface-400">|</span>
                    <Button label="-r requirements.txt" icon="pi pi-upload" size="small" outlined @click="$refs.reqFile.click()" />
                    <input ref="reqFile" type="file" accept=".txt" class="hidden" @change="uploadRequirements" />
                  </div>
                  <DataTable v-model:selection="selectedPackages" :value="uvPackages" dataKey="name" stripedRows :rows="10" paginator>
                    <Column selectionMode="multiple" style="width: 3rem" />
                    <Column field="name" header="Package" />
                    <Column field="version" header="Version" />
                  </DataTable>
                </template>
              </Card>
            </TabPanel>

            <!-- API Tab -->
            <TabPanel value="api">
              <Card class="mb-4">
                <template #title>API Examples</template>
                <template #content>
                  <div class="flex flex-col gap-3">
                    <div>
                      <div class="font-semibold mb-1">Add Account</div>
                      <pre class="text-xs bg-surface-100 dark:bg-surface-800 p-3 rounded overflow-x-auto m-0"><code>curl -X POST {{ baseUrl }}/api/accounts \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": {{ categoryId }}, "data": "{\"username\": \"user\", \"password\": \"pass\"}"}'</code></pre>
                    </div>
                    <div>
                      <div class="font-semibold mb-1">Get Account (Fetch & Mark Used)</div>
                      <pre class="text-xs bg-surface-100 dark:bg-surface-800 p-3 rounded overflow-x-auto m-0"><code>curl -X POST {{ baseUrl }}/api/accounts/fetch \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": {{ categoryId }}, "count": 1}'</code></pre>
                    </div>
                    <div>
                      <div class="font-semibold mb-1">Mark Account as Banned</div>
                      <pre class="text-xs bg-surface-100 dark:bg-surface-800 p-3 rounded overflow-x-auto m-0"><code>curl -X PUT {{ baseUrl }}/api/accounts/update \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"ids": [ACCOUNT_ID], "banned": true}'</code></pre>
                    </div>
                  </div>
                </template>
              </Card>
              <Card>
                <template #title>
                  <div class="flex items-center justify-between">
                    <span>API Call History</span>
                    <div class="flex items-center gap-2">
                      <Button icon="pi pi-refresh" text @click="loadAPIHistory" />
                    </div>
                  </div>
                </template>
                <template #content>
                  <div class="flex items-center gap-4 mb-3">
                    <div class="flex items-center gap-2">
                      <label class="text-sm">History Limit:</label>
                      <InputNumber v-model="historyLimit" :min="1" :max="10000" :showButtons="false" inputClass="w-24" />
                    </div>
                    <Button label="Save" icon="pi pi-save" outlined size="small" @click="saveHistoryLimit" />
                  </div>
                  <DataTable :value="apiHistory" stripedRows paginator :rows="10" :rowsPerPageOptions="[10, 25, 50]">
                    <Column field="created_at" header="Time" style="width: 180px">
                      <template #body="{ data }">{{ new Date(data.created_at).toLocaleString() }}</template>
                    </Column>
                    <Column field="method" header="Method" style="width: 80px">
                      <template #body="{ data }"><Tag :value="data.method" /></template>
                    </Column>
                    <Column field="endpoint" header="Endpoint" />
                    <Column field="status_code" header="Status" style="width: 80px">
                      <template #body="{ data }">
                        <Tag :value="data.status_code" :severity="data.status_code === 200 ? 'success' : 'danger'" />
                      </template>
                    </Column>
                    <Column field="request" header="Request">
                      <template #body="{ data }"><code class="text-xs">{{ data.request }}</code></template>
                    </Column>
                    <Column field="request_ip" header="Request IP">
                      <template #body="{ data }"><code class="text-xs">{{ data.request_ip }}</code></template>
                    </Column>
                  </DataTable>
                </template>
              </Card>
            </TabPanel>
          </TabPanels>
        </Tabs>
      </template>
    </Card>
  </div>
  <div v-else>
    <Card>
      <template #title>
        <div class="flex items-center gap-2">
          <i class="pi pi-chart-bar text-primary"></i>
          Dashboard Overview
        </div>
      </template>
      <template #content>
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Card class="bg-surface-100 dark:bg-surface-800">
            <template #content>
              <div class="text-center">
                <div class="text-3xl font-bold text-primary">{{ globalStats.categories }}</div>
                <div class="text-sm text-surface-500">Categories</div>
              </div>
            </template>
          </Card>
          <Card class="bg-surface-100 dark:bg-surface-800">
            <template #content>
              <div class="text-center">
                <div class="text-3xl font-bold text-green-500">{{ globalStats.accounts?.available || 0 }}</div>
                <div class="text-sm text-surface-500">Available</div>
              </div>
            </template>
          </Card>
          <Card class="bg-surface-100 dark:bg-surface-800">
            <template #content>
              <div class="text-center">
                <div class="text-3xl font-bold text-yellow-500">{{ globalStats.accounts?.used || 0 }}</div>
                <div class="text-sm text-surface-500">Used</div>
              </div>
            </template>
          </Card>
          <Card class="bg-surface-100 dark:bg-surface-800">
            <template #content>
              <div class="text-center">
                <div class="text-3xl font-bold text-red-500">{{ globalStats.accounts?.banned || 0 }}</div>
                <div class="text-sm text-surface-500">Banned</div>
              </div>
            </template>
          </Card>
        </div>
        <Card class="mt-4">
          <template #title>All Accounts</template>
          <template #content>
            <Chart type="line" :data="globalChartData" :options="chartOptions" class="h-64" />
          </template>
        </Card>
        <Card class="mt-4">
          <template #title>API Reference</template>
          <template #content>
            <div class="flex flex-col gap-3">
              <div>
                <div class="font-semibold mb-1">Create Category (idempotent)</div>
                <pre class="text-xs bg-surface-100 dark:bg-surface-800 p-3 rounded overflow-x-auto m-0"><code>curl -X POST {{ baseUrl }}/api/categories/ensure \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-category"}'</code></pre>
              </div>
              <div>
                <div class="font-semibold mb-1">Add Account</div>
                <pre class="text-xs bg-surface-100 dark:bg-surface-800 p-3 rounded overflow-x-auto m-0"><code>curl -X POST {{ baseUrl }}/api/accounts \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": 1, "data": "{\"username\": \"user\"}"}'</code></pre>
              </div>
              <div>
                <div class="font-semibold mb-1">Add Accounts (Bulk)</div>
                <pre class="text-xs bg-surface-100 dark:bg-surface-800 p-3 rounded overflow-x-auto m-0"><code>curl -X POST {{ baseUrl }}/api/accounts/bulk \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": 1, "data": ["account1", "account2"]}'</code></pre>
              </div>
              <div>
                <div class="font-semibold mb-1">Fetch Account (marks as used)</div>
                <pre class="text-xs bg-surface-100 dark:bg-surface-800 p-3 rounded overflow-x-auto m-0"><code>curl -X POST {{ baseUrl }}/api/accounts/fetch \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"category_id": 1, "count": 1}'</code></pre>
              </div>
              <div>
                <div class="font-semibold mb-1">Mark Account as Banned</div>
                <pre class="text-xs bg-surface-100 dark:bg-surface-800 p-3 rounded overflow-x-auto m-0"><code>curl -X PUT {{ baseUrl }}/api/accounts/update \
  -H "X-Passkey: YOUR_PASSKEY" \
  -H "Content-Type: application/json" \
  -d '{"ids": [1], "banned": true}'</code></pre>
              </div>
            </div>
          </template>
        </Card>
      </template>
    </Card>
  </div>
  <Dialog v-model:visible="logDialogVisible" header="Run Log" :style="{ width: '80vw', maxWidth: '1000px' }" modal>
    <pre class="text-xs bg-surface-100 dark:bg-surface-800 p-3 rounded overflow-auto m-0" style="max-height: 70vh">{{ runLog || 'No log available' }}</pre>
  </Dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { useLayout } from '../layout/composables/layout'
import api from '../api'
import Button from 'primevue/button'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Textarea from 'primevue/textarea'
import Tag from 'primevue/tag'
import Toolbar from 'primevue/toolbar'
import Chart from 'primevue/chart'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Tabs from 'primevue/tabs'
import { VueMonacoEditor } from '@guolao/vue-monaco-editor'
import TabList from 'primevue/tablist'
import Tab from 'primevue/tab'
import TabPanels from 'primevue/tabpanels'
import TabPanel from 'primevue/tabpanel'
import Dialog from 'primevue/dialog'

const route = useRoute()
const confirm = useConfirm()
const toast = useToast()
const categoryId = computed(() => route.params.categoryId)
const baseUrl = computed(() => window.location.origin)
const categoryName = ref('')
const accounts = ref([])
const newAccountData = ref('')
const selectedAccountsObj = ref([])
const validationScript = ref('')
const validationConcurrency = ref(1)
const validationCron = ref('0 0 * * *')
const validationRuns = ref([])
const testAccount = ref('')
const testResult = ref(null)
const testLoading = ref(false)
let runsPollingInterval = null
const logDialogVisible = ref(false)
const runLog = ref('')
const uvPackages = ref([])
const newPackage = ref('')
const packageLoading = ref(false)
const selectedPackages = ref([])
const chartData = ref({})
const globalStats = ref({ categories: 0, accounts: { total: 0, available: 0, used: 0, banned: 0 }, chart: [] })
const apiHistory = ref([])
const historyLimit = ref(1000)
const globalChartData = computed(() => buildChartData(globalStats.value.chart || {}))
const { isDarkTheme } = useLayout()
const chartOptions = computed(() => {
  const textColor = isDarkTheme.value ? '#fff' : '#333'
  const gridColor = isDarkTheme.value ? 'rgba(255,255,255,0.2)' : 'rgba(0,0,0,0.1)'
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: { legend: { display: true, labels: { color: textColor } } },
    scales: {
      x: { grid: { display: true, color: gridColor }, ticks: { color: textColor } },
      y: { grid: { display: true, color: gridColor }, ticks: { color: textColor }, beginAtZero: true }
    }
  }
})

const selectedAccounts = computed(() => selectedAccountsObj.value.map(a => a.id))
const availableCount = computed(() => accounts.value.filter(a => !a.used && !a.banned).length)
const usedCount = computed(() => accounts.value.filter(a => a.used).length)
const bannedCount = computed(() => accounts.value.filter(a => a.banned).length)

const loadAccounts = async () => {
  if (!categoryId.value) {
    const res = await api.getGlobalStats()
    globalStats.value = res.data
    return
  }
  const res = await api.getAccounts(categoryId.value)
  accounts.value = res.data
  const catRes = await api.getCategory(categoryId.value)
  const cat = catRes.data
  categoryName.value = cat?.name || ''
  validationScript.value = cat?.validation_script || `def validate(account: str) -> tuple[bool, bool]:
    # Return (used, banned)
    return False, False`
  validationConcurrency.value = cat?.validation_concurrency || 1
  validationCron.value = cat?.validation_cron || '0 0 * * *'
  historyLimit.value = cat?.history_limit || 1000
  loadAPIHistory()
  const runsRes = await api.getValidationRuns(categoryId.value)
  validationRuns.value = runsRes.data || []
  startRunsPolling()
  loadPackages()
  const statsRes = await api.getAccountStats(categoryId.value)
  const stats = statsRes.data || {}
  chartData.value = buildChartData(stats)
}

const buildChartData = (stats) => {
  const dates = new Set([
    ...(stats.added || []).map(s => s.date),
    ...(stats.used || []).map(s => s.date),
    ...(stats.banned || []).map(s => s.date)
  ])
  const labels = [...dates].sort()
  const addedMap = Object.fromEntries((stats.added || []).map(s => [s.date, s.count]))
  const usedMap = Object.fromEntries((stats.used || []).map(s => [s.date, s.count]))
  const bannedMap = Object.fromEntries((stats.banned || []).map(s => [s.date, s.count]))
  return {
    labels,
    datasets: [
      { label: 'Added', data: labels.map(d => addedMap[d] || 0), borderColor: '#22c55e', tension: 0.4 },
      { label: 'Used', data: labels.map(d => usedMap[d] || 0), borderColor: '#f59e0b', tension: 0.4 },
      { label: 'Banned', data: labels.map(d => bannedMap[d] || 0), borderColor: '#ef4444', tension: 0.4 }
    ]
  }
}

watch(categoryId, loadAccounts, { immediate: true })

const pollValidationRuns = async () => {
  if (!categoryId.value) return
  try {
    const runsRes = await api.getValidationRuns(categoryId.value)
    validationRuns.value = runsRes.data || []
    console.log('Polling runs:', validationRuns.value)
    if (validationRuns.value.some(r => r.status === 'running')) {
      setTimeout(pollValidationRuns, 3000)
    } else if (runsPollingInterval) {
      runsPollingInterval = false
      await loadAccounts()
    }
  } catch (e) {
    console.error('Poll error:', e)
  }
}

const startRunsPolling = () => {
  if (!runsPollingInterval && validationRuns.value.some(r => r.status === 'running')) {
    runsPollingInterval = true
    setTimeout(pollValidationRuns, 1000)
  }
}

const copyData = (text) => {
  navigator.clipboard.writeText(text)
  toast.add({ severity: 'success', summary: 'Copied', life: 2000 })
}

const addAccount = async () => {
  if (!newAccountData.value) return
  const lines = newAccountData.value.split('\n').filter(l => l.trim()).map(l => l.trim())
  try {
    if (lines.length === 1) {
      await api.addAccount(Number(categoryId.value), lines[0])
      toast.add({ severity: 'success', summary: 'Success', detail: '1 account added', life: 3000 })
    } else {
      const res = await api.addAccountsBulk(Number(categoryId.value), lines)
      const { count, skipped } = res.data
      if (skipped > 0) {
        toast.add({ severity: 'warn', summary: 'Added', detail: `${count} added, ${skipped} duplicates skipped`, life: 4000 })
      } else {
        toast.add({ severity: 'success', summary: 'Success', detail: `${count} account(s) added`, life: 3000 })
      }
    }
    newAccountData.value = ''
    await loadAccounts()
  } catch (e) {
    if (e.response?.status === 409) {
      toast.add({ severity: 'error', summary: 'Duplicate', detail: 'Account already exists', life: 3000 })
    } else {
      toast.add({ severity: 'error', summary: 'Error', detail: e.response?.data?.error || e.message, life: 3000 })
    }
  }
}

const updateAccounts = async (ids, status) => {
  try {
    await api.updateAccounts(ids, status)
    selectedAccountsObj.value = []
    await loadAccounts()
  } catch (e) {
    console.error('updateAccounts error:', e)
  }
}

const confirmDeleteUsed = () => {
  confirm.require({
    message: 'Delete all used accounts?',
    header: 'Confirm',
    icon: 'pi pi-exclamation-triangle',
    accept: deleteUsed
  })
}

const deleteUsed = async () => {
  await api.deleteAccounts(categoryId.value, true, false)
  toast.add({ severity: 'success', summary: 'Success', detail: 'Used accounts deleted', life: 3000 })
  await loadAccounts()
}

const confirmDeleteBanned = () => {
  confirm.require({
    message: 'Delete all banned accounts?',
    header: 'Confirm',
    icon: 'pi pi-exclamation-triangle',
    accept: deleteBanned
  })
}

const confirmDeleteSelected = () => {
  confirm.require({
    message: `Delete ${selectedAccounts.value.length} selected account(s)?`,
    header: 'Confirm',
    icon: 'pi pi-exclamation-triangle',
    accept: deleteSelected
  })
}

const deleteSelected = async () => {
  await api.deleteAccountsByIds(selectedAccounts.value)
  toast.add({ severity: 'success', summary: 'Success', detail: 'Selected accounts deleted', life: 3000 })
  selectedAccountsObj.value = []
  await loadAccounts()
}

const deleteBanned = async () => {
  await api.deleteAccounts(categoryId.value, false, true)
  toast.add({ severity: 'success', summary: 'Success', detail: 'Banned accounts deleted', life: 3000 })
  await loadAccounts()
}

const saveValidationScript = async () => {
  await api.updateValidationScript(categoryId.value, validationScript.value, validationConcurrency.value, validationCron.value)
  toast.add({ severity: 'success', summary: 'Success', detail: 'Validation script saved', life: 3000 })
}

const runValidationNow = async () => {
  try {
    await api.updateValidationScript(categoryId.value, validationScript.value, validationConcurrency.value, validationCron.value)
    await api.runValidationNow(categoryId.value)
    toast.add({ severity: 'info', summary: 'Started', detail: 'Validation started', life: 3000 })
    runsPollingInterval = true
    setTimeout(pollValidationRuns, 500)
  } catch (e) {
    toast.add({ severity: 'error', summary: 'Error', detail: e.response?.data?.error || e.message, life: 3000 })
  }
}

const testScript = async () => {
  if (!validationScript.value || !testAccount.value || !categoryId.value) return
  testLoading.value = true
  testResult.value = null
  try {
    const res = await api.testValidationScript(categoryId.value, validationScript.value, testAccount.value)
    testResult.value = res.data
  } catch (e) {
    testResult.value = { success: false, error: e.message }
  }
  testLoading.value = false
}

const showRunLog = async (runId) => {
  try {
    const res = await api.getValidationRunLog(runId)
    runLog.value = res.data.log || ''
    logDialogVisible.value = true
  } catch (e) {
    toast.add({ severity: 'error', summary: 'Error', detail: 'Failed to load log', life: 3000 })
  }
}

const loadPackages = async () => {
  if (!categoryId.value) return
  try {
    const res = await api.getUVPackages(categoryId.value)
    uvPackages.value = Array.isArray(res.data) ? res.data : []
  } catch (e) {
    uvPackages.value = []
  }
}

const installPackage = async () => {
  if (!newPackage.value || !categoryId.value) return
  packageLoading.value = true
  try {
    const res = await api.installUVPackage(categoryId.value, newPackage.value)
    if (res.data.success) {
      toast.add({ severity: 'success', summary: 'Installed', detail: newPackage.value, life: 3000 })
      newPackage.value = ''
      await loadPackages()
    } else {
      toast.add({ severity: 'error', summary: 'Failed', detail: res.data.output, life: 5000 })
    }
  } catch (e) {
    toast.add({ severity: 'error', summary: 'Error', detail: e.message, life: 3000 })
  }
  packageLoading.value = false
}

const uninstallSelectedPackages = async () => {
  if (!categoryId.value || !selectedPackages.value.length) return
  for (const pkg of selectedPackages.value) {
    try {
      await api.uninstallUVPackage(categoryId.value, pkg.name)
    } catch (e) { /* ignore */ }
  }
  toast.add({ severity: 'success', summary: 'Uninstalled', detail: `${selectedPackages.value.length} package(s)`, life: 3000 })
  selectedPackages.value = []
  await loadPackages()
}

const uploadRequirements = async (e) => {
  const file = e.target.files[0]
  if (!file || !categoryId.value) return
  packageLoading.value = true
  try {
    const res = await api.installRequirements(categoryId.value, file)
    if (res.data.success) {
      toast.add({ severity: 'success', summary: 'Installed', detail: 'Requirements installed', life: 3000 })
      await loadPackages()
    } else {
      toast.add({ severity: 'error', summary: 'Failed', detail: res.data.output, life: 5000 })
    }
  } catch (err) {
    toast.add({ severity: 'error', summary: 'Error', detail: err.message, life: 3000 })
  }
  packageLoading.value = false
  e.target.value = ''
}

const loadAPIHistory = async () => {
  if (!categoryId.value) return
  try {
    const res = await api.getAPICallHistory(categoryId.value)
    apiHistory.value = res.data || []
  } catch (e) {
    apiHistory.value = []
  }
}

const saveHistoryLimit = async () => {
  if (!categoryId.value) return
  try {
    await api.updateHistoryLimit(categoryId.value, historyLimit.value)
    toast.add({ severity: 'success', summary: 'Saved', detail: 'History limit updated', life: 3000 })
  } catch (e) {
    toast.add({ severity: 'error', summary: 'Error', detail: e.message, life: 3000 })
  }
}
</script>
