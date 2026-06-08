<template>
  <button @click="showLANDialog = true" id="lan-access-btn" title="LAN Access Settings"
    class="absolute top-3 left-3 w-12 h-12 flex items-center justify-center rounded-full bg-odoo text-white shadow-lg shadow-odoo/30 hover:shadow-odoo/50 hover:scale-105 active:scale-95 transition-all duration-300 ease-out">
    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round"
        d="M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.141 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0" />
    </svg>
  </button>
  <teleport to="body">
    <transition enter-active-class="transition duration-200 ease-out" enter-from-class="opacity-0"
      enter-to-class="opacity-100" leave-active-class="transition duration-150 ease-in" leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div v-if="showLANDialog" class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/50" @click="close" />
        <div class="relative bg-white rounded-2xl w-full max-w-sm shadow-xl overflow-hidden">

          <!-- Header -->
          <div class="bg-odoo px-6 py-4 flex items-center justify-between">
            <div class="flex items-center gap-2">
              <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round"
                  d="M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.141 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0" />
              </svg>
              <span class="text-white font-semibold text-base">LAN Access</span>
            </div>
            <CloseButton @click="close" />
          </div>

          <!-- Content -->
          <div class="p-6 space-y-5">

            <!-- Toggle Row -->
            <label class="flex items-start gap-3 cursor-pointer group">
              <div class="relative mt-0.5 flex-shrink-0">
                <input type="checkbox" class="sr-only" :checked="lanEnabled" :disabled="loading" @change="handleToggle"
                  id="lan-access-toggle" />
                <div class="w-11 h-6 rounded-full transition-colors duration-200"
                  :class="lanEnabled ? 'bg-odoo' : 'bg-gray-200'">
                  <div
                    class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full shadow transition-transform duration-200"
                    :class="lanEnabled ? 'translate-x-5' : 'translate-x-0'"></div>
                </div>
              </div>
              <div>
                <p class="text-sm font-medium text-gray-900 leading-tight">Allow LAN access to this device</p>
                <p class="text-xs text-gray-500 mt-0.5 leading-relaxed">
                  Allow other devices on your local network to send print jobs to this service.
                </p>
              </div>
            </label>

            <div class="h-px bg-gray-100"></div>

            <!-- Elevation notice (shown when toggling) -->
            <transition enter-active-class="transition-all duration-200" enter-from-class="opacity-0 -translate-y-1"
              enter-to-class="opacity-100 translate-y-0">
              <div class="rounded-lg bg-amber-50 border border-amber-200 p-3 flex gap-2">
                <svg class="w-4 h-4 text-amber-500 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24"
                  stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                </svg>
                <p class="text-xs text-amber-700 leading-relaxed">
                  This action requires <strong>administrator permission</strong> to modify the firewall and allow LAN
                  devices to access the printer service.
                </p>
              </div>
            </transition>

            <!-- Connection Information -->
            <div class="space-y-4">
              <div>
                <p class="text-xs font-medium text-gray-500 uppercase tracking-wide mb-2">
                  Server Address
                </p>

                <div class="flex items-center justify-between rounded-lg border border-gray-200 px-3 py-2">
                  <span class="font-mono text-sm">
                    {{ networkUrl }}
                  </span>

                  <button class="text-odoo text-sm hover:underline" @click="copyUrl(localhostUrl)">
                    Copy
                  </button>
                </div>
              </div>
            </div>

            <!-- Error message -->
            <div v-if="error" class="rounded-lg bg-red-50 border border-red-200 px-3 py-2">
              <p class="text-sm text-danger">{{ error }}</p>
            </div>

            <!-- Loading state -->
            <div v-if="loading" class="flex items-center justify-center gap-2 py-1">
              <svg class="w-4 h-4 text-odoo animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
              </svg>
              <span class="text-sm text-gray-500">{{ loading }}</span>
            </div>

          </div>

          <!-- Footer -->
          <div class="px-6 pb-4">
            <button @click="close"
              class="w-full border border-gray-200 rounded-lg px-4 py-2 text-sm text-gray-600 hover:bg-gray-50 transition-colors cursor-pointer">
              Close
            </button>
          </div>

        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import { ref, watch, onMounted, computed } from 'vue'
import CloseButton from './close-button.vue'
import { GetLANSettings, EnableLANAccess, DisableLANAccess } from '../../wailsjs/go/main/App'
import { useToast } from '../hooks/useToast'
import { EventsOn } from '../../wailsjs/runtime'
import QrcodeVue from 'qrcode.vue'

const { notify } = useToast()

const settings = ref({ enabled: false, ip: '', port: 0 })
const lanEnabled = ref(false)
const loading = ref(null)
const error = ref(null)
const showLANDialog = ref(false)

onMounted(() => {
  try { EventsOn('open-lan-settings', () => { showLANDialog.value = true }) } catch (e) { /* no-op outside wails */ }
})

watch(() => showLANDialog.value, async (val) => {
  if (val) {
    error.value = null
    await refreshSettings()
  }
})

async function refreshSettings() {
  try {
    const s = await GetLANSettings()
    settings.value = s
    lanEnabled.value = s.enabled
  } catch (err) {
    console.error('Failed to load LAN settings:', err)
    error.value = 'Failed to load settings.'
  }
}

async function handleToggle(event) {
  const newValue = event.target.checked
  error.value = null

  loading.value = newValue ? 'Applying firewall rule…' : 'Removing firewall rule…'

  try {
    if (newValue) {
      await EnableLANAccess()
      notify('LAN access enabled. Server is now accessible on your local network.', 'success')
    } else {
      await DisableLANAccess()
      notify('LAN access disabled. Server is localhost only.', 'success')
    }
    await refreshSettings()
  } catch (err) {
    error.value = err || 'Failed to update LAN access settings.'
    notify(err || 'Failed to update settings.', 'danger')
    // Revert toggle on failure
    lanEnabled.value = !newValue
  } finally {
    loading.value = null
  }
}

function close() {
  showLANDialog.value = false
}

const networkUrl = computed(() => {
  if (!settings.value.ip) return ''
  return `http://${settings.value.ip}:${settings.value.port}`
})

const localhostUrl = computed(() => {
  return `http://127.0.0.1:${settings.value.port}`
})

async function copyUrl(url) {
  try {
    await navigator.clipboard.writeText(url)
    notify('Address copied to clipboard', 'success')
  } catch {
    notify('Failed to copy address', 'danger')
  }
}
</script>
