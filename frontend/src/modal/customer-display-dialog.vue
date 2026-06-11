<template>
  <button @click="openCustomerDisplaySettings" id="customer-display-btn" title="Customer Display Settings"
    class="absolute top-3 left-17 w-12 h-12 flex items-center justify-center rounded-full bg-odoo text-white shadow-lg shadow-odoo/30 hover:shadow-odoo/50 hover:scale-105 active:scale-95 transition-all duration-300 ease-out">
    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round"
        d="M9 17h6m-7 3h8a2 2 0 002-2V6a2 2 0 00-2-2H8a2 2 0 00-2 2v12a2 2 0 002 2zm-1-4h10" />
    </svg>
  </button>
  <teleport to="body">
    <transition enter-active-class="transition duration-200 ease-out" enter-from-class="opacity-0"
      enter-to-class="opacity-100" leave-active-class="transition duration-150 ease-in" leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div v-if="show" class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="close" />

        <div class="relative bg-white rounded-2xl w-full max-w-lg shadow-2xl overflow-hidden flex flex-col"
          style="max-height: 90vh;">

          <!-- Header -->
          <div class="bg-odoo px-6 py-4 flex items-center justify-between flex-shrink-0">
            <div class="flex items-center gap-3">
              <div class="w-8 h-8 rounded-lg bg-white/20 flex items-center justify-center">
                <svg class="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
              </div>
              <div>
                <p class="text-white font-semibold text-base leading-tight">Customer Display</p>
                <p class="text-white/60 text-xs">WebView URL Management</p>
              </div>
            </div>
            <CloseButton @click="close" />
          </div>

          <!-- Body -->
          <div class="flex-1 overflow-y-auto">

            <!-- Active URL Banner -->
            <div v-if="activeURL"
              class="mx-4 mt-4 rounded-xl border border-green-200 bg-green-50 p-3 flex items-start gap-3">
              <div class="w-2 h-2 rounded-full bg-green-500 flex-shrink-0 mt-1.5 animate-pulse"></div>
              <div class="flex-1 min-w-0">
                <p class="text-xs font-semibold text-green-800 mb-0.5">Active WebView URL</p>
                <p class="text-xs text-green-700 font-mono break-all">{{ activeURL.url }}</p>
                <p class="text-xs text-green-600 mt-0.5">{{ activeURL.name }}</p>
              </div>
              <button @click="openWebView"
                class="flex-shrink-0 px-3 py-1.5 bg-green-600 text-white text-xs font-medium rounded-lg hover:bg-green-700 transition-colors cursor-pointer"
                title="Open WebView">
                Open
              </button>
            </div>

            <div v-else class="mx-4 mt-4 rounded-xl border border-amber-200 bg-amber-50 p-3 flex items-center gap-3">
              <svg class="w-4 h-4 text-amber-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"
                stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round"
                  d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
              </svg>
              <p class="text-xs text-amber-700">No active URL configured. Add a URL and enable it to activate the
                customer display WebView.</p>
            </div>

            <!-- URL List -->
            <div class="px-4 py-3 space-y-2">
              <div v-if="loading" class="flex items-center justify-center py-8 gap-2">
                <svg class="w-4 h-4 text-odoo animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                </svg>
                <span class="text-sm text-gray-500">Loading...</span>
              </div>

              <div v-else-if="urls.length === 0 && !showForm" class="text-center py-8">
                <div class="w-12 h-12 mx-auto mb-3 rounded-full bg-gray-100 flex items-center justify-center">
                  <svg class="w-6 h-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"
                    stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round"
                      d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                  </svg>
                </div>
                <p class="text-sm font-medium text-gray-700 mb-1">No URLs configured</p>
                <p class="text-xs text-gray-500">Add a URL to enable the customer display WebView.</p>
              </div>

              <div v-for="url in urls" :key="url.id" class="rounded-xl border transition-all duration-150" :class="url.enabled
                ? 'border-green-200 bg-green-50/50'
                : 'border-gray-200 bg-white hover:border-gray-300'">
                <div class="p-3 flex items-start gap-3">
                  <!-- Status dot -->
                  <div class="mt-1 flex-shrink-0">
                    <div class="w-2 h-2 rounded-full" :class="url.enabled ? 'bg-green-500' : 'bg-gray-300'"></div>
                  </div>

                  <!-- Info -->
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2 flex-wrap">
                      <span class="text-sm font-semibold text-gray-900">{{ url.name }}</span>
                      <span v-if="url.enabled"
                        class="px-1.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700">Active</span>
                    </div>
                    <p class="text-xs font-mono text-gray-500 mt-0.5 break-all">{{ url.url }}</p>
                    <p v-if="url.description" class="text-xs text-gray-400 mt-0.5">{{ url.description }}</p>
                    <p class="text-xs text-gray-400 mt-1">Added {{ formatDate(url.createdAt) }}</p>
                  </div>

                  <!-- Actions -->
                  <div class="flex items-center gap-1 flex-shrink-0">
                    <button v-if="!url.enabled" @click="setActive(url)"
                      class="p-1.5 rounded-lg text-green-600 hover:bg-green-100 transition-colors cursor-pointer"
                      title="Set as active">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                      </svg>
                    </button>
                    <button v-else @click="deactivate(url)"
                      class="p-1.5 rounded-lg text-amber-600 hover:bg-amber-100 transition-colors cursor-pointer"
                      title="Deactivate">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
                      </svg>
                    </button>
                    <button @click="startEdit(url)"
                      class="p-1.5 rounded-lg text-gray-500 hover:bg-gray-100 hover:text-gray-700 transition-colors cursor-pointer"
                      title="Edit">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                      </svg>
                    </button>
                    <button @click="confirmDelete(url)"
                      class="p-1.5 rounded-lg text-red-400 hover:bg-red-50 hover:text-red-600 transition-colors cursor-pointer"
                      title="Delete">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Add / Edit Form -->
            <transition enter-active-class="transition-all duration-200 ease-out"
              enter-from-class="opacity-0 -translate-y-2" enter-to-class="opacity-100 translate-y-0">
              <div v-if="showForm" class="mx-4 mb-4 rounded-xl border border-odoo/30 bg-odoo/5 p-4 space-y-3">
                <p class="text-sm font-semibold text-odoo">{{ editingId ? 'Edit URL' : 'Add New URL' }}</p>

                <!-- Name -->
                <div>
                  <label class="block text-xs font-medium text-gray-700 mb-1">Name <span
                      class="text-red-400">*</span></label>
                  <input id="cd-name" v-model="form.name" type="text" placeholder="e.g. Main Customer Display"
                    class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-odoo focus:border-odoo outline-none"
                    :class="{ 'border-danger ring-1 ring-danger': formErrors.name }" />
                  <p v-if="formErrors.name" class="text-xs text-danger mt-1">{{ formErrors.name }}</p>
                </div>

                <!-- URL -->
                <div>
                  <label class="block text-xs font-medium text-gray-700 mb-1">URL <span
                      class="text-red-400">*</span></label>
                  <input id="cd-url" v-model="form.url" type="url" placeholder="https://example.com/customer-display"
                    class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm font-mono focus:ring-2 focus:ring-odoo focus:border-odoo outline-none"
                    :class="{ 'border-danger ring-1 ring-danger': formErrors.url }" @input="validateURL" />
                  <p v-if="formErrors.url" class="text-xs text-danger mt-1">{{ formErrors.url }}</p>
                  <p v-else-if="urlValid && form.url" class="text-xs text-success mt-1 flex items-center gap-1">
                    <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                    Valid URL
                  </p>
                </div>

                <!-- Description -->
                <div>
                  <label class="block text-xs font-medium text-gray-700 mb-1">Description <span
                      class="text-gray-400">(optional)</span></label>
                  <textarea id="cd-description" v-model="form.description" rows="2"
                    placeholder="Brief description of this display..."
                    class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-odoo focus:border-odoo outline-none resize-none"></textarea>
                </div>

                <!-- Enabled toggle -->
                <label class="flex items-center gap-3 cursor-pointer">
                  <div class="relative flex-shrink-0">
                    <input type="checkbox" class="sr-only" v-model="form.enabled" id="cd-enabled" />
                    <div class="w-9 h-5 rounded-full transition-colors duration-200"
                      :class="form.enabled ? 'bg-odoo' : 'bg-gray-200'">
                      <div
                        class="absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform duration-200"
                        :class="form.enabled ? 'translate-x-4' : 'translate-x-0'"></div>
                    </div>
                  </div>
                  <div>
                    <p class="text-sm font-medium text-gray-800">Set as active</p>
                    <p class="text-xs text-gray-500">This URL will be used for the customer display</p>
                  </div>
                </label>

                <!-- Form error -->
                <p v-if="formError" class="text-xs text-danger bg-red-50 border border-red-200 rounded-lg px-3 py-2">{{
                  formError }}</p>

                <!-- Form buttons -->
                <div class="flex gap-2 pt-1">
                  <button @click="cancelForm"
                    class="flex-1 border border-gray-300 rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors cursor-pointer">
                    Cancel
                  </button>
                  <button @click="submitForm" :disabled="formLoading"
                    class="flex-1 border rounded-lg px-4 py-2 text-sm font-medium bg-odoo text-white hover:bg-odoo-dark disabled:opacity-50 disabled:cursor-not-allowed transition-colors cursor-pointer">
                    {{ formLoading ? 'Saving…' : (editingId ? 'Update' : 'Add URL') }}
                  </button>
                </div>
              </div>
            </transition>

          </div>

          <!-- Footer -->
          <div class="px-4 py-3 border-t border-gray-100 flex gap-2 flex-shrink-0">
            <button v-if="!showForm" @click="startAdd"
              class="flex-1 flex items-center justify-center gap-2 border-2 border-dashed border-odoo/30 text-odoo rounded-xl px-4 py-2 text-sm font-medium hover:border-odoo/60 hover:bg-odoo/5 transition-all cursor-pointer">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
              </svg>
              Add URL
            </button>
            <button @click="close"
              class="flex-1 border border-gray-200 rounded-xl px-4 py-2 text-sm font-medium text-gray-600 hover:bg-gray-50 transition-colors cursor-pointer">
              Close
            </button>
          </div>

        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import CloseButton from './close-button.vue'
import { connector, safeEventsOn } from '../connector'
import { useToast } from '../hooks/useToast'

const emit = defineEmits(['open-webview'])
const { notify } = useToast()

const show = ref(false)
const loading = ref(false)
const urls = ref([])
const activeURL = computed(() => urls.value.find(u => u.enabled) || null)

// Form state
const showForm = ref(false)
const editingId = ref(null)
const formLoading = ref(false)
const formError = ref(null)
const urlValid = ref(false)
const form = ref({ name: '', url: '', description: '', enabled: false })
const formErrors = ref({ name: '', url: '' })

watch(() => show.value, (val) => {
  if (val) {
    loadURLs()
    cancelForm()
  }
})

async function loadURLs() {
  loading.value = true
  try {
    urls.value = (await connector.getCustomerDisplayURLs()) || []
  } catch (err) {
    console.error('Failed to load customer display URLs:', err)
    notify('Failed to load customer display URLs', 'danger')
  } finally {
    loading.value = false
  }
}

function formatDate(isoString) {
  if (!isoString) return ''
  try {
    return new Date(isoString).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
  } catch {
    return isoString
  }
}

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
  form.value = { name: '', url: '', description: '', enabled: false }
  formErrors.value = { name: '', url: '' }
  formError.value = null
  urlValid.value = false
  editingId.value = null
}

function startAdd() {
  resetForm()
  showForm.value = true
}

function startEdit(url) {
  resetForm()
  editingId.value = url.id
  form.value = { name: url.name, url: url.url, description: url.description || '', enabled: url.enabled }
  urlValid.value = true
  showForm.value = true
}

function cancelForm() {
  showForm.value = false
  resetForm()
}

async function submitForm() {
  // Validate
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
    if (editingId.value) {
      await connector.updateCustomerDisplayURL(
        editingId.value,
        form.value.name.trim(),
        form.value.url.trim(),
        form.value.description.trim(),
        form.value.enabled
      )
      notify('Customer display URL updated', 'success')
    } else {
      const record = await connector.addCustomerDisplayURL(
        form.value.name.trim(),
        form.value.url.trim(),
        form.value.description.trim()
      )
      // If user checked "enabled", activate it
      if (form.value.enabled && record?.id) {
        await connector.setActiveCustomerDisplayURL(record.id)
      }
      notify('Customer display URL added', 'success')
    }

    cancelForm()
    await loadURLs()
  } catch (err) {
    formError.value = err?.message || 'Failed to save URL'
  } finally {
    formLoading.value = false
  }
}

async function setActive(url) {
  try {
    await connector.setActiveCustomerDisplayURL(url.id)
    notify(`"${url.name}" set as active customer display`, 'success')
    await loadURLs()
  } catch (err) {
    notify(err?.message || 'Failed to set active URL', 'danger')
  }
}

async function deactivate(url) {
  try {
    await connector.disableCustomerDisplayURL(url.id)
    notify('Customer display WebView deactivated', 'success')
    await loadURLs()
  } catch (err) {
    notify(err?.message || 'Failed to deactivate URL', 'danger')
  }
}

async function confirmDelete(url) {
  if (!window.confirm(`Delete "${url.name}"?\n\nThis cannot be undone.`)) return
  try {
    await connector.deleteCustomerDisplayURL(url.id)
    notify('URL deleted', 'success')
    await loadURLs()
  } catch (err) {
    notify(err?.message || 'Failed to delete URL', 'danger')
  }
}

function openWebView() {
  if (!activeURL.value) {
    notify('No active URL configured', 'danger')
    return
  }
  emit('open-webview', activeURL.value.url)
  close()
}

function close() {
  show.value = false
  cancelForm()
}

function openCustomerDisplaySettings() {
  show.value = true
  loadURLs()
}
</script>
