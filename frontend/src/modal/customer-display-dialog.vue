<template>
  <button @click="openSettings" id="customer-display-btn" title="Customer Display Settings"
    class="absolute top-3 left-16 w-12 h-12 flex items-center justify-center rounded-full bg-odoo text-white shadow-lg shadow-odoo/30 hover:shadow-odoo/50 hover:scale-105 active:scale-95 transition-all duration-300 ease-out">
    <svg class="w-6 h-6" viewBox="0 0 36 36" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
      <path
        d="M32.5,3H3.5A1.5,1.5,0,0,0,2,4.5v21A1.5,1.5,0,0,0,3.5,27h29A1.5,1.5,0,0,0,34,25.5V4.5A1.5,1.5,0,0,0,32.5,3ZM32,25H4V5H32Z" />
      <polygon points="7.7 8.76 28.13 8.76 29.94 7.16 6.1 7.16 6.1 23 7.7 23 7.7 8.76" />
      <path
        d="M26,32H24.26a3.61,3.61,0,0,1-1.5-2.52V28.13H21.24V29.5A4.2,4.2,0,0,0,22.17,32H13.83a4.2,4.2,0,0,0,.93-2.52V28.13H13.24V29.5A3.61,3.61,0,0,1,11.74,32H9.94a1,1,0,1,0,0,2H26.06a.92.92,0,0,0,1-1A1,1,0,0,0,26,32Z" />
    </svg>
  </button>

  <teleport to="body">
    <transition enter-active-class="transition duration-200 ease-out" enter-from-class="opacity-0"
      enter-to-class="opacity-100" leave-active-class="transition duration-150 ease-in" leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div v-if="show" class="fixed inset-0 flex items-end sm:items-center justify-center p-4"
        :style="{ zIndex: zIndex }">
        <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="close" />

        <div class="relative bg-white rounded-2xl w-full max-w-md shadow-2xl overflow-hidden flex flex-col"
          style="max-height: 85vh;">

          <!-- Header -->
          <div class="bg-odoo px-5 py-4 flex items-center justify-between flex-shrink-0">
            <div class="flex items-center gap-3">
              <div class="w-8 h-8 rounded-lg bg-white/20 flex items-center justify-center">
                <svg class="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
              </div>
              <div>
                <p class="text-white font-semibold text-sm leading-tight">Customer Display</p>
                <p class="text-white/60 text-xs">WebView Configuration</p>
              </div>
            </div>
            <CloseButton @click="close" />
          </div>

          <!-- Body -->
          <div class="flex-1 overflow-y-auto p-5 space-y-4">

            <!-- Loading -->
            <div v-if="loading" class="flex items-center justify-center py-10 gap-2">
              <svg class="w-4 h-4 text-odoo animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
              </svg>
              <span class="text-sm text-gray-500">Loading...</span>
            </div>

            <template v-else>

              <!-- Current URL card -->
              <div v-if="currentURL" class="rounded-xl border border-green-200 bg-green-50 p-4">
                <div class="flex items-start gap-3">
                  <div class="w-2 h-2 rounded-full bg-green-500 flex-shrink-0 mt-1.5 animate-pulse"></div>
                  <div class="flex-1 min-w-0">
                    <p class="text-xs font-semibold text-green-800 mb-0.5">Configured URL</p>
                    <p class="text-sm font-semibold text-gray-900">{{ currentURL.name }}</p>
                    <p class="text-xs font-mono text-green-700 break-all mt-0.5">{{ currentURL.url }}</p>
                    <p v-if="currentURL.description" class="text-xs text-gray-400 mt-1">{{ currentURL.description }}</p>
                  </div>
                </div>

                <!-- Actions -->
                <div class="flex gap-2 mt-3">
                  <button @click="emitOpenWebView"
                    class="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 bg-green-600 text-white text-xs font-semibold rounded-lg hover:bg-green-700 transition-colors cursor-pointer">
                    <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                      <path stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    Open WebView
                  </button>
                  <button @click="deleteURL" :disabled="deleteLoading"
                    class="flex items-center justify-center gap-1.5 px-3 py-2 bg-red-50 text-red-600 text-xs font-semibold rounded-lg hover:bg-red-100 border border-red-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors cursor-pointer">
                    <svg v-if="!deleteLoading" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"
                      stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round"
                        d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                    <svg v-else class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
                      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z">
                      </path>
                    </svg>
                    Delete
                  </button>
                </div>
              </div>

              <!-- No URL state -->
              <div v-else class="rounded-xl border border-dashed border-gray-300 bg-gray-50 p-6 text-center">
                <div class="w-10 h-10 mx-auto mb-3 rounded-full bg-gray-200 flex items-center justify-center">
                  <svg class="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"
                    stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round"
                      d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                  </svg>
                </div>
                <p class="text-sm font-medium text-gray-600 mb-1">No URL configured</p>
                <p class="text-xs text-gray-400">Add a URL below to enable the customer display WebView.</p>
              </div>

              <!-- Add URL form (shown when no URL exists) -->
              <transition enter-active-class="transition-all duration-200 ease-out"
                enter-from-class="opacity-0 -translate-y-1" enter-to-class="opacity-100 translate-y-0">
                <div v-if="!currentURL" class="rounded-xl border border-odoo/25 bg-odoo/5 p-4 space-y-3">
                  <p class="text-xs font-semibold text-odoo uppercase tracking-wide">Add Customer Display URL</p>

                  <!-- Name -->
                  <div>
                    <label class="block text-xs font-medium text-gray-700 mb-1">Name <span
                        class="text-red-400">*</span></label>
                    <input id="cd-name" v-model="form.name" type="text" placeholder="e.g. Main Customer Display"
                      class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-odoo focus:border-odoo outline-none transition-all"
                      :class="{ 'border-danger ring-1 ring-danger': formErrors.name }" />
                    <p v-if="formErrors.name" class="text-xs text-danger mt-1">{{ formErrors.name }}</p>
                  </div>

                  <!-- URL -->
                  <div>
                    <label class="block text-xs font-medium text-gray-700 mb-1">URL <span
                        class="text-red-400">*</span></label>
                    <input id="cd-url" v-model="form.url" type="url" placeholder="https://example.com/customer-display"
                      class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm font-mono focus:ring-2 focus:ring-odoo focus:border-odoo outline-none transition-all"
                      :class="{ 'border-danger ring-1 ring-danger': formErrors.url }" @input="validateURL" />
                    <p v-if="formErrors.url" class="text-xs text-danger mt-1">{{ formErrors.url }}</p>
                    <p v-else-if="urlValid && form.url" class="text-xs text-success mt-1 flex items-center gap-1">
                      <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                      </svg>
                      Valid URL
                    </p>
                  </div>

                  <!-- Description (optional) -->
                  <div>
                    <label class="block text-xs font-medium text-gray-700 mb-1">
                      Description <span class="text-gray-400">(optional)</span>
                    </label>
                    <input id="cd-description" v-model="form.description" type="text" placeholder="Brief description..."
                      class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-odoo focus:border-odoo outline-none transition-all" />
                  </div>

                  <!-- Error -->
                  <p v-if="formError" class="text-xs text-danger bg-red-50 border border-red-200 rounded-lg px-3 py-2">
                    {{ formError }}
                  </p>

                  <!-- Save button -->
                  <button @click="addURL" :disabled="formLoading"
                    class="w-full rounded-lg px-4 py-2.5 text-sm font-semibold bg-odoo text-white hover:bg-odoo-dark disabled:opacity-50 disabled:cursor-not-allowed transition-colors cursor-pointer">
                    {{ formLoading ? 'Saving…' : 'Save & Activate' }}
                  </button>
                </div>
              </transition>

            </template>
          </div>

          <!-- Footer -->
          <div class="px-5 py-3 border-t border-gray-100 flex-shrink-0">
            <button @click="close"
              class="w-full border border-gray-200 rounded-xl px-4 py-2 text-sm font-medium text-gray-600 hover:bg-gray-50 transition-colors cursor-pointer">
              Close
            </button>
          </div>

        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import CloseButton from './close-button.vue'
import { connector, safeEventsOn, isDesktopApp } from '../connector'
import { useToast } from '../hooks/useToast'

const props = defineProps({
  // When rendered inside the WebView overlay, pass a higher z-index
  zIndex: { type: Number, default: 150 }
})

const emit = defineEmits(['open-webview', 'close'])
const { notify } = useToast()

const show = ref(false)
const loading = ref(false)
const currentURL = ref(null)   // The single configured URL (or null)

// Delete
const deleteLoading = ref(false)

// Form state
const formLoading = ref(false)
const formError = ref(null)
const urlValid = ref(false)
const form = ref({ name: '', url: '', description: '' })
const formErrors = ref({ name: '', url: '' })

// ── Lifecycle ─────────────────────────────────────────────────────────────────

onMounted(() => {
  safeEventsOn('open-customer-display-settings', () => openSettings())
})

watch(() => show.value, (val) => {
  if (val) {
    loadURL()
    resetForm()
  }
})

// ── Data ──────────────────────────────────────────────────────────────────────

async function loadURL() {
  loading.value = true
  try {
    currentURL.value = await connector.getActiveCustomerDisplayURL()
  } catch (err) {
    console.error('Failed to load customer display URL:', err)
    notify('Failed to load customer display URL', 'danger')
  } finally {
    loading.value = false
  }
}

// ── URL Validation ─────────────────────────────────────────────────────────────

function validateURL() {
  formErrors.value.url = ''
  urlValid.value = false
  const v = form.value.url.trim()
  if (!v) return
  try {
    const parsed = new URL(v)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      formErrors.value.url = 'URL must use http:// or https://'
      return
    }
    urlValid.value = true
  } catch {
    formErrors.value.url = 'Invalid URL format'
  }
}

function resetForm() {
  form.value = { name: '', url: '', description: '' }
  formErrors.value = { name: '', url: '' }
  formError.value = null
  urlValid.value = false
}

// ── Add ───────────────────────────────────────────────────────────────────────

async function addURL() {
  formErrors.value = { name: '', url: '' }
  formError.value = null

  if (!form.value.name.trim()) {
    formErrors.value.name = 'Name is required'
    return
  }

  validateURL()
  if (!urlValid.value) return

  formLoading.value = true
  try {
    // Add the URL and immediately set it as active
    const record = await connector.addCustomerDisplayURL(
      form.value.name.trim(),
      form.value.url.trim(),
      form.value.description.trim()
    )
    if (record?.id) {
      await connector.setActiveCustomerDisplayURL(record.id)
    }
    notify('Customer display URL saved', 'success')
    resetForm()
    await loadURL()
  } catch (err) {
    formError.value = err?.message || 'Failed to save URL'
  } finally {
    formLoading.value = false
  }
}

// ── Delete ────────────────────────────────────────────────────────────────────

async function deleteURL() {
  if (!currentURL.value) return
  if (!window.confirm(`Delete "${currentURL.value.name}"?\n\nThe customer display WebView will be disabled.`)) return

  deleteLoading.value = true
  try {
    await connector.deleteCustomerDisplayURL(currentURL.value.id)
    notify('Customer display URL deleted', 'success')
    currentURL.value = null
  } catch (err) {
    notify(err?.message || 'Failed to delete URL', 'danger')
  } finally {
    deleteLoading.value = false
  }
}

// ── Open WebView ──────────────────────────────────────────────────────────────

function emitOpenWebView() {
  if (!currentURL.value) {
    notify('No URL configured', 'danger')
    return
  }
  emit('open-webview', currentURL.value.url)
  close()
}

// ── Show / Hide ───────────────────────────────────────────────────────────────

function openSettings() {
  show.value = true
}

function close() {
  show.value = false
  emit('close')
}

// ── Expose ────────────────────────────────────────────────────────────────────
defineExpose({ openSettings, close, loadURL })
</script>
