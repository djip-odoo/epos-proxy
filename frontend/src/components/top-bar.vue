<script setup>
import { ref, watch } from 'vue'
import refresh_svg from "../assets/images/refresh.svg"
const props = defineProps({
  modelValue: {
    type: String,
    default: 'EPOS'
  },
  loading: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:modelValue', 'refresh'])

const activeFilter = ref(props.modelValue)

// sync with parent
watch(() => props.modelValue, (val) => {
  activeFilter.value = val
})

// emit changes
function setFilter(type) {
  activeFilter.value = type
  emit('update:modelValue', type)
  handleRefresh()
}

// styling
function filterBtnClass(type) {
  return [
    "px-3 py-1 rounded-full text-sm border cursor-pointer transition",
    activeFilter.value === type
      ? "bg-odoo text-white"
      : "bg-gray-100 text-gray-700 hover:bg-gray-200"
  ]
}

async function handleRefresh() {
  emit('refresh')
}

</script>

<template>
  <div class="flex justify-between gap-2 border-b border-gray-200">
    <div class="flex gap-2 flex-wrap items-center py-1">
      <button @click="setFilter('EPOS')" :class="filterBtnClass('EPOS')">
        Thermal
      </button>

      <button @click="setFilter('PDF')" :class="filterBtnClass('PDF')">
        Normal
      </button>
      <button @click="setFilter('UNKNOWN')" :class="filterBtnClass('UNKNOWN')">
        Unknown
      </button>
    </div>
    <button>
      <img :src="refresh_svg" alt="Refresh" class="w-5 h-5 cursor-pointer" :class="{ 'animate-spin': props.loading }"
        @click="handleRefresh" title="Refresh printer list" />
    </button>
  </div>
</template>