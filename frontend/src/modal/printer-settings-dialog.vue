<template>
  <teleport to="body">
    <transition enter-active-class="transition duration-200 ease-out" enter-from-class="opacity-0"
      enter-to-class="opacity-100" leave-active-class="transition duration-150 ease-in" leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div v-if="show" class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/75" @click="close" />

        <div class="relative bg-white rounded-2xl w-full max-w-sm shadow-xl overflow-hidden p-6">
          <div class="flex items-center justify-between mb-4">
            <div class="text-lg font-semibold text-gray-900 break-all pr-4">
              {{ printer?.name || 'Printer' }}
            </div>

            <CloseButton @click="close" />
          </div>

          <div class="mb-5">
            <div class="text-sm font-medium text-gray-700 mb-3">
              Receipt Width
            </div>
            <div class="space-y-2">
              <!-- 384 -->
              <label
                class="flex items-center gap-3 p-3 border border-gray-200 rounded-xl hover:bg-slate-50 cursor-pointer transition">
                <input type="radio" name="printer-width" :value="384" v-model="selectedWidth"
                  class="h-4 w-4 text-odoo focus:ring-odoo border-gray-300" />

                <div class="flex-1">
                  <div class="text-sm font-medium text-gray-900">
                    58mm (384px)
                  </div>

                  <div class="text-xs text-gray-500">
                    Standard format
                  </div>
                </div>
              </label>

              <!-- 576 -->
              <label
                class="flex items-center gap-3 p-3 border border-gray-200 rounded-xl hover:bg-slate-50 cursor-pointer transition">
                <input type="radio" name="printer-width" :value="576" v-model="selectedWidth"
                  class="h-4 w-4 text-odoo focus:ring-odoo border-gray-300" />

                <div class="flex-1">
                  <div class="text-sm font-medium text-gray-900">
                    80mm (576px)
                  </div>

                  <div class="text-xs text-gray-500">
                    Wide format (Default)
                  </div>
                </div>
              </label>

              <!-- Custom -->
              <label class="flex items-start gap-3 p-3 border border-gray-200 rounded-xl hover:bg-slate-50 transition">
                <input type="radio" name="printer-width" value="custom" v-model="selectedWidth"
                  class="h-4 w-4 mt-1 text-odoo focus:ring-odoo border-gray-300" />

                <div class="flex-1">
                  <div class="text-sm font-medium text-gray-900 mb-2">
                    Custom Width
                  </div>

                  <div v-if="selectedWidth === 'custom'">
                    <div class="flex items-center gap-2">
                      <input type="number" min="200" max="1200" step="8" v-model.number="customWidth"
                        placeholder="Enter width"
                        class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-odoo focus:border-odoo outline-none" />

                      <span class="text-sm text-gray-500">
                        px
                      </span>
                    </div>

                    <div v-if="error" class="text-danger text-xs mt-2">
                      {{ error }}
                    </div>
                    <div v-else class="text-xs text-gray-500 mt-2">
                      Must be divisible by 8
                    </div>
                  </div>
                </div>
              </label>

            </div>
          </div>

          <div class="flex gap-3">
            <button @click="close"
              class="flex-1 border border-gray-300 rounded-lg px-4 py-2 cursor-pointer text-sm font-medium text-gray-700 bg-white hover:bg-gray-50">
              Cancel
            </button>

            <button @click="save" :disabled="loading"
              class="flex-1 border rounded-lg px-4 py-2 cursor-pointer text-sm font-medium bg-odoo text-white hover:bg-odoo-dark disabled:opacity-50">
              {{ loading ? 'Saving...' : 'Save' }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import { ref, watch } from 'vue'
import CloseButton from './close-button.vue'
import { GetPrinterWidth, SetPrinterWidth } from '../../wailsjs/go/main/App'

const props = defineProps({
  show: { type: Boolean, default: false },
  printer: { type: Object, required: true },
})

const emit = defineEmits(['close', 'notify'])

const selectedWidth = ref(576)
const customWidth = ref(576)
const error = ref(null)
const loading = ref(false)

watch(() => props.show, async (newVal) => {
  if (newVal && props.printer?.id) {
    error.value = null
    loading.value = true
    try {
      const width = await GetPrinterWidth(props.printer.id)
      if (width && [384, 576].includes(width)) {
        selectedWidth.value = width
      } else if (width) {
        customWidth.value = width
        selectedWidth.value = 'custom'
      } else {
        selectedWidth.value = 576
      }
    } catch (err) {
      console.error('Failed to get printer width:', err)
      selectedWidth.value = 576
    } finally {
      loading.value = false
    }
  }
})

function close() {
  emit('close')
}

async function save() {
  loading.value = true
  error.value = null

  try {
    const width =
      selectedWidth.value === 'custom'
        ? Number(customWidth.value)
        : Number(selectedWidth.value)

    if (!width || width < 200 || width > 1200) {
      error.value = 'Width must be between 200 and 1200'
      return
    }

    if (width % 8 !== 0) {
      error.value = 'Width must be divisible by 8'
      return
    }

    await SetPrinterWidth(props.printer.id, width)
    emit('notify', 'Printer width updated successfully', 'success')
    close()
  } catch (err) {
    console.error('Failed to save printer settings:', err)
    error.value = err?.message || err || 'Failed to save settings'
  } finally {
    loading.value = false
  }
}
</script>
