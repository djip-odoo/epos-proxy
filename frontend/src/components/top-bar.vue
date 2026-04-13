<template>
  <div class="bg-white border-b border-gray-200">
    <div class="px-6 flex items-center justify-between">

      <!-- Tabs -->
      <div class="flex -mb-px">
        <button v-for="tab in tabs" :key="tab.value" @click="setFilter(tab.value)" :class="tabClass(tab.value)">
          {{ tab.label }}
        </button>
      </div>

      <!-- Refresh Button -->
      <button @click="emit('refresh')" class="p-3 rounded-2xl hover:bg-gray-100 active:bg-gray-200 transition-all"
        :title="loading ? 'Refreshing...' : 'Refresh printer list'" :disabled="loading">
        <img :src="refresh_svg" alt="Refresh" class="w-5 h-5 transition-transform"
          :class="{ 'animate-spin': loading }" />
      </button>

    </div>
  </div>

</template>

<script setup>
import { defineModel, computed } from 'vue'
import refresh_svg from "../assets/images/refresh.svg"

// Props & Model
const modelValue = defineModel({
  type: String,
  default: 'EPOS'
})

const props = defineProps({
  loading: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['refresh'])

// Tab Configuration
const tabs = [
  { label: 'Thermal', value: 'EPOS' },
  { label: 'Office', value: 'PDF' },
  { label: 'Unknown', value: 'UNKNOWN' }
]

// Set filter
function setFilter(value) {
  modelValue.value = value
  emit('refresh')           // Refresh when switching tabs (as per your original logic)
}

// Tab Class
const tabClass = (value) => {
  const isActive = modelValue.value === value

  return [
    'px-8 py-4 text-sm transition-all relative cursor-pointer',
    isActive
      ? 'text-odoo-dark font-bold border-b-2 border-odoo-dark'
      : 'text-gray-500 font-medium hover:text-gray-700 border-transparent hover:border-gray-300'
  ]
}
</script>
