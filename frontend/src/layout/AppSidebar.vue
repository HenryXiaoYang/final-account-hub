<template>
    <div class="layout-sidebar">
        <div class="layout-menu-container p-4">
            <div @click="goHome" :class="['flex items-center gap-2 py-3 px-4 cursor-pointer transition-colors', !selectedCategoryId ? 'bg-primary text-primary-contrast' : 'hover:bg-emphasis']">
                <i class="pi pi-home"></i>
                <span class="font-bold">Home</span>
            </div>
            <Accordion v-model:value="accordionValue" :pt="{ root: 'w-full' }">
                <AccordionPanel value="categories" :pt="{ root: 'border-0' }">
                    <AccordionHeader :pt="{ root: 'flex-row-reverse justify-between py-3 px-4 border-0 bg-transparent' }">
                        <span class="flex items-center gap-2">
                            <i class="pi pi-folder"></i>
                            <span class="font-bold">Categories</span>
                        </span>
                    </AccordionHeader>
                    <AccordionContent>
                        <div class="flex gap-2 mb-3">
                            <InputText v-model="newCategory" placeholder="Category name" class="flex-1" size="small" @keyup.enter="createCategory" />
                            <Button icon="pi pi-plus" size="small" @click="createCategory" />
                        </div>
                        <div v-if="categories.length === 0" class="text-sm text-gray-500 text-center py-2">
                            No categories
                        </div>
                        <ul v-else class="list-none p-0 m-0">
                            <li v-for="cat in categories" :key="cat.id" class="border-b" style="border-color: var(--p-amber-500)">
                                <div @click="selectCategory(cat)" :class="['flex items-center justify-between p-3 cursor-pointer transition-colors', selectedCategoryId === cat.id ? 'bg-primary text-primary-contrast' : 'hover:bg-emphasis']">
                                    <div>
                                        <div>{{ cat.name }}</div>
                                        <div class="text-xs text-gray-400">ID: {{ cat.id }}</div>
                                    </div>
                                    <Button icon="pi pi-trash" text size="small" severity="danger" @click.stop="confirmDelete(cat)" />
                                </div>
                            </li>
                        </ul>
                    </AccordionContent>
                </AccordionPanel>
            </Accordion>
        </div>
    </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import api from '../api'
import InputText from 'primevue/inputtext'
import Button from 'primevue/button'
import Accordion from 'primevue/accordion'
import AccordionPanel from 'primevue/accordionpanel'
import AccordionHeader from 'primevue/accordionheader'
import AccordionContent from 'primevue/accordioncontent'

const router = useRouter()
const confirm = useConfirm()
const toast = useToast()
const categories = ref([])
const newCategory = ref('')
const selectedCategoryId = ref(null)
const accordionValue = ref('categories')

const loadCategories = async () => {
    const res = await api.getCategories()
    categories.value = res.data
}

const createCategory = async () => {
    if (!newCategory.value) return
    if (categories.value.some(c => c.name.toLowerCase() === newCategory.value.toLowerCase())) {
        toast.add({ severity: 'warn', summary: 'Warning', detail: 'Category already exists', life: 3000 })
        return
    }
    await api.createCategory(newCategory.value)
    toast.add({ severity: 'success', summary: 'Success', detail: 'Category created', life: 3000 })
    newCategory.value = ''
    await loadCategories()
}

const goHome = () => {
    selectedCategoryId.value = null
    router.push('/dashboard')
}

const selectCategory = (cat) => {
    selectedCategoryId.value = cat.id
    router.push(`/dashboard/${cat.id}`)
}

const confirmDelete = (cat) => {
    confirm.require({
        message: `Delete "${cat.name}"? All accounts in this category will be permanently deleted and cannot be recovered.`,
        header: 'Confirm Delete',
        icon: 'pi pi-exclamation-triangle',
        accept: async () => {
            await api.deleteCategory(cat.id)
            toast.add({ severity: 'success', summary: 'Success', detail: 'Category deleted', life: 3000 })
            if (selectedCategoryId.value === cat.id) {
                router.push('/dashboard')
            }
            await loadCategories()
        }
    })
}

loadCategories()
</script>
