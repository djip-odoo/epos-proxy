<template>
  <button @click="openDialog" id="settings-btn" title="Settings"
    class="absolute top-3 left-3 w-12 h-12 flex items-center justify-center rounded-full bg-odoo text-white shadow-lg shadow-odoo/30 hover:shadow-odoo/50 hover:scale-105 active:scale-95 transition-all duration-300 ease-out">
    <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round"
        d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  </button>

  <teleport to="body">
    <transition enter-active-class="transition duration-200 ease-out" enter-from-class="opacity-0"
      enter-to-class="opacity-100" leave-active-class="transition duration-150 ease-in" leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div v-if="showDialog" class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/75" @click="close" />

        <div class="relative bg-white rounded-2xl w-full max-w-sm shadow-xl overflow-hidden p-6">
          <div class="flex items-center justify-between mb-4">
            <div class="text-lg font-semibold text-gray-900">
              Settings
            </div>

            <CloseButton @click="close" />
          </div>

          <div class="mb-5">
            <div class="text-sm font-medium text-gray-700 mb-3">
              Receipt Width
            </div>

            <div class="space-y-2">

              <!-- 320 -->
              <label
                class="flex items-center gap-3 p-3 border border-gray-200 rounded-xl hover:bg-slate-50 cursor-pointer transition">
                <input type="radio" name="receipt-width" :value="320" v-model="selectedWidth"
                  class="h-4 w-4 text-odoo focus:ring-odoo border-gray-300" />

                <div class="flex-1">
                  <div class="text-sm font-medium text-gray-900">
                    58mm (320px)
                  </div>

                  <div class="text-xs text-gray-500">
                    Narrow format
                  </div>
                </div>
              </label>

              <!-- 384 -->
              <label
                class="flex items-center gap-3 p-3 border border-gray-200 rounded-xl hover:bg-slate-50 cursor-pointer transition">
                <input type="radio" name="receipt-width" :value="384" v-model="selectedWidth"
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
                <input type="radio" name="receipt-width" :value="576" v-model="selectedWidth"
                  class="h-4 w-4 text-odoo focus:ring-odoo border-gray-300" />

                <div class="flex-1">
                  <div class="text-sm font-medium text-gray-900">
                    80mm (576px)
                  </div>

                  <div class="text-xs text-gray-500">
                    Wide format
                  </div>
                </div>
              </label>

              <!-- Custom -->
              <label class="flex items-start gap-3 p-3 border border-gray-200 rounded-xl hover:bg-slate-50 transition">
                <input type="radio" name="receipt-width" value="custom" v-model="selectedWidth"
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
                    <div v-else="" class="text-xs text-gray-500 mt-2">
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
import { ref } from 'vue'
import CloseButton from './close-button.vue'

import {
  GetReceiptWidth,
  SetReceiptWidth
} from '../../wailsjs/go/main/App'

const emit = defineEmits(['notify'])

const selectedWidth = ref(384)
const customWidth = ref(320)

const error = ref(null)
const loading = ref(false)
const showDialog = ref(false)

async function openDialog() {
  error.value = null

  try {
    const width = await GetReceiptWidth()

    if ([320, 384, 576].includes(width)) {
      selectedWidth.value = width
    } else {
      customWidth.value = width
      selectedWidth.value = 'custom'
    }
  } catch (err) {
    console.error('Failed to load receipt width:', err)
  }

  showDialog.value = true
}

function close() {
  showDialog.value = false
}

async function save() {
  loading.value = true

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

    await SetReceiptWidth(width)

    emit(
      'notify',
      'Receipt width updated successfully',
      'success'
    )

    close()
  } catch (err) {
    console.error('Failed to set receipt width:', err)

    error.value =
      err?.message ||
      err ||
      'Failed to save settings'
  } finally {
    loading.value = false
  }
}
</script>
