<template>
  <button @click="showLANDialog = true" id="lan-access-btn" title="LAN Access Settings"
    class="absolute top-3 left-3 w-12 h-12 flex items-center justify-center rounded-full shadow-lg transition-all duration-300 bg-odoo text-white shadow-odoo/30">
    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round"
        d="M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.141 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0" />
    </svg>

    <span class="absolute bottom-1 right-1 w-2.5 h-2.5 rounded-full border-2 border-white"
      :class="lanEnabled ? 'bg-green-500' : 'bg-red-500'" />
  </button>
  <teleport to="body">
    <transition enter-active-class="transition duration-200 ease-out" enter-from-class="opacity-0"
      enter-to-class="opacity-100" leave-active-class="transition duration-150 ease-in" leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div v-if="showLANDialog" class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/50" @click="close" />
        <div class="relative bg-white rounded-2xl w-full max-w-sm shadow-xl overflow-hidden">

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

          <div class="p-6 space-y-5 overflow-y-auto" style="max-height: 70vh;">
            <label class="flex items-start gap-3 cursor-pointer group">
              <div class="relative mt-0.5 flex-shrink-0">
                <input type="checkbox" class="sr-only" :checked="lanEnabled" :disabled="loading"
                  @change="handleToggle" />
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

            <!-- macOS-specific guidance: the OS uses an app-level firewall, not port rules -->
            <div v-if="isMacOS" class="rounded-lg bg-blue-50 mt-2 border border-blue-200 p-3 flex gap-2">
              <svg class="w-4 h-4 text-blue-500 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24"
                stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round"
                  d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" />
              </svg>
              <p class="text-xs text-blue-700 leading-relaxed">
                <strong>macOS note:</strong> macOS uses an application-level firewall. You may need to allow this
                application in <strong>System Settings → Privacy &amp; Security → Firewall</strong> for LAN access to
                work.
              </p>
            </div>

            <div class="space-y-3">
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-gray-500 uppercase tracking-wide">LAN Address</span>
                <div class="flex items-center gap-2">
                  <span class="text-sm font-mono font-medium text-gray-800">
                    {{ settings.ip ? `${settings.ip}:${settings.port}` : 'Loading…' }}
                  </span>
                  <button aria-label="Information" v-if="lanEnabled"
                    title="Verify that the displayed LAN address belongs to your current Wi-Fi or Ethernet network. Devices on the same network can use this address to send print job to the printer. If the address is incorrect, check your network connection and router settings."
                    class="flex h-8 w-8 items-center justify-center rounded-full bg-amber-100 text-amber-700 hover:bg-amber-200 hover:text-amber-800 transition-colors">
                    <svg xmlns="http://www.w3.org/2000/svg" width="25" height="25" fill="currentColor"
                      viewBox="0 0 16 16">
                      <path
                        d="m8.93 6.588-2.29.287-.082.38.45.083c.294.07.352.176.288.469l-.738 3.468c-.194.897.105 1.319.808 1.319.545 0 1.178-.252 1.465-.598l.088-.416c-.2.176-.492.246-.686.246-.275 0-.375-.193-.304-.533zM9 4.5a1 1 0 1 1-2 0 1 1 0 0 1 2 0" />
                    </svg>
                  </button>
                </div>
              </div>
            </div>

            <div v-if="error" class="rounded-lg bg-red-50 border border-red-200 px-3 py-2">
              <p class="text-sm text-danger">{{ error }}</p>
            </div>

            <div v-if="loading" class="flex items-center justify-center gap-2 py-1">
              <svg class="w-4 h-4 text-odoo animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
              </svg>
              <span class="text-sm text-gray-500">{{ loadingMessage }}</span>
            </div>

          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import { ref, watch, onMounted, computed } from 'vue'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import CloseButton from './close-button.vue'
import { GetLANSettings, EnableLANAccess, DisableLANAccess } from '../../wailsjs/go/main/App'

const props = defineProps({
  os: { type: String, default: null }
})

const emit = defineEmits(['notify', 'refresh'])
const settings = ref({ enabled: false, ip: '', port: 0 })
const lanEnabled = ref(false)
const loading = ref(false)
const loadingMessage = ref('')
const error = ref(null)
const showLANDialog = ref(false)

const isMacOS = computed(() => props.os?.toLowerCase().includes('darwin') ?? false)

onMounted(async () => {
  await refreshSettings()
  EventsOn('open-lan-settings', () => { showLANDialog.value = true })
})

watch(showLANDialog, async (opened) => {
  if (!opened) {
    return
  }

  error.value = null
  await refreshSettings()
})

async function refreshSettings() {
  try {
    const s = await GetLANSettings()
    settings.value = s
    lanEnabled.value = s.enabled
    emit('refresh')
  } catch (err) {
    error.value = 'Failed to load LAN settings.'
  }
}

async function handleToggle() {
  if (loading.value) {
    return
  }
  loading.value = true

  const enabled = !lanEnabled.value
  error.value = null
  loadingMessage.value = isMacOS.value
    ? (enabled ? 'Enabling LAN access…' : 'Disabling LAN access…')
    : (enabled ? 'Applying firewall rule…' : 'Removing firewall rule…')

  try {
    if (enabled) {
      await EnableLANAccess()
      const successMsg = isMacOS.value
        ? 'LAN access enabled. Please ensure this app is allowed in macOS Firewall settings.'
        : 'LAN access enabled. Server is now accessible on your local network.'
      emit('notify', successMsg, 'success')
    } else {
      await DisableLANAccess()
      emit('notify', 'LAN access disabled. Server is localhost only.', 'success')
    }
    await refreshSettings()
  } catch (err) {
    const message = getLANAccessErrorMessage(err)
    error.value = message
    await refreshSettings()
  } finally {
    loading.value = false
    loadingMessage.value = ''
  }
}

function close() {
  if (loading.value) return
  loadingMessage.value = ''
  showLANDialog.value = false
}

function getLANAccessErrorMessage(err) {
  const errorText = typeof err === "string" ? err : err?.message?.trim() || "";
  return errorText.length > 100 ? `${errorText.slice(0, 100)}...` : errorText || "Failed to update LAN access settings.";
}

</script>
