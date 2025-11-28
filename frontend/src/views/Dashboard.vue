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
        <Card class="mb-4">
          <template #title>Accounts</template>
          <template #content>
            <Chart type="line" :data="chartData" :options="chartOptions" class="h-64" />
          </template>
        </Card>

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

        <Accordion value="0">
          <AccordionPanel value="0">
            <AccordionHeader>Add Account</AccordionHeader>
            <AccordionContent>
              <Textarea v-model="newAccountData" placeholder='one line = one account, you can put access token refresh token client secret api key' rows="3" class="w-full mb-3" />
              <Button label="Add Account" icon="pi pi-plus" @click="addAccount" />
            </AccordionContent>
          </AccordionPanel>
        </Accordion>

        <Toolbar class="my-4">
          <template #start>
            <Button label="Set Used" icon="pi pi-check" @click="updateAccounts(selectedAccounts, { used: true })" :disabled="!selectedAccounts.length" class="mr-2" />
            <Button label="Set Available" icon="pi pi-replay" @click="updateAccounts(selectedAccounts, { used: false, banned: false })" :disabled="!selectedAccounts.length" class="mr-2" />
            <Button label="Set Banned" icon="pi pi-ban" @click="updateAccounts(selectedAccounts, { banned: true })" :disabled="!selectedAccounts.length" class="mr-2" />
            <Button label="Unban" icon="pi pi-undo" @click="updateAccounts(selectedAccounts, { banned: false })" :disabled="!selectedAccounts.length" />
          </template>
          <template #end>
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
import Accordion from 'primevue/accordion'
import AccordionPanel from 'primevue/accordionpanel'
import AccordionHeader from 'primevue/accordionheader'
import AccordionContent from 'primevue/accordioncontent'
import Chart from 'primevue/chart'

const route = useRoute()
const confirm = useConfirm()
const toast = useToast()
const categoryId = computed(() => route.params.categoryId)
const baseUrl = computed(() => window.location.origin)
const categoryName = ref('')
const accounts = ref([])
const newAccountData = ref('')
const selectedAccountsObj = ref([])
const chartData = ref({})
const globalStats = ref({ categories: 0, accounts: { total: 0, available: 0, used: 0, banned: 0 }, chart: [] })
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
  const catRes = await api.getCategories()
  const cat = catRes.data.find(c => c.id == categoryId.value)
  categoryName.value = cat?.name || ''
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

const copyData = (text) => {
  navigator.clipboard.writeText(text)
  toast.add({ severity: 'success', summary: 'Copied', life: 2000 })
}

const addAccount = async () => {
  if (!newAccountData.value) return
  const lines = newAccountData.value.split('\n').filter(l => l.trim()).map(l => l.trim())
  if (lines.length === 1) {
    await api.addAccount(Number(categoryId.value), lines[0])
  } else {
    await api.addAccountsBulk(Number(categoryId.value), lines)
  }
  toast.add({ severity: 'success', summary: 'Success', detail: `${lines.length} account(s) added`, life: 3000 })
  newAccountData.value = ''
  await loadAccounts()
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

const deleteBanned = async () => {
  await api.deleteAccounts(categoryId.value, false, true)
  toast.add({ severity: 'success', summary: 'Success', detail: 'Banned accounts deleted', life: 3000 })
  await loadAccounts()
}

</script>
