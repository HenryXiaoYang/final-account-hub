<template>
  <div class="flex items-center justify-center min-h-screen" style="background-color: var(--surface-ground)">
    <Card class="w-96">
      <template #title>
        <div class="flex items-center justify-center gap-2">
          <i class="pi pi-lock text-primary"></i>
          Final Account Hub Login
        </div>
      </template>
      <template #content>
        <Password v-model="passkey" placeholder="Enter passkey" class="w-full mb-4" inputClass="w-full" :feedback="false" toggleMask @keyup.enter="login" />
        <Button label="Login" icon="pi pi-sign-in" class="w-full" @click="login" />
        <Message v-if="error" severity="error" class="mt-4">{{ error }}</Message>
      </template>
    </Card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import Card from 'primevue/card'
import Password from 'primevue/password'
import Button from 'primevue/button'
import Message from 'primevue/message'

const router = useRouter()
const passkey = ref('')
const error = ref('')

const login = async () => {
  try {
    localStorage.setItem('passkey', passkey.value)
    await api.getCategories()
    router.push('/dashboard')
  } catch (e) {
    error.value = 'Invalid passkey'
    localStorage.removeItem('passkey')
  }
}
</script>
