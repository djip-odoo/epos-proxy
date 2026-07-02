<template>
  <teleport to="body">
    <transition enter-active-class="transition duration-200 ease-out" enter-from-class="opacity-0"
      enter-to-class="opacity-100" leave-active-class="transition duration-150 ease-in" leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div v-if="showDialog" class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/55 backdrop-blur-sm" @click="close" />
        <div class="relative bg-white rounded-2xl w-full max-w-md shadow-2xl overflow-hidden border border-gray-100">

          <!-- Header -->
          <div class="bg-odoo px-6 py-4 flex items-center justify-between">
            <div class="flex items-center gap-2">
              <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                <path stroke-linecap="round" stroke-linejoin="round"
                  d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
              <span class="text-white font-semibold text-base">Enable Network Printing</span>
            </div>
            <CloseButton v-if="!loading" @click="close" />
          </div>

          <!-- Content -->
          <div class="p-6 space-y-5">
            <div v-if="authCancelled" class="space-y-4">
              <div class="rounded-xl bg-amber-50 border border-amber-200 p-4 flex gap-3">
                <svg class="w-5 h-5 text-amber-500 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24"
                  stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <div class="text-sm text-amber-800 leading-relaxed font-medium">
                  Firewall configuration was cancelled.<br />
                  You can enable network printing later from the Settings menu.
                </div>
              </div>
            </div>

            <div v-else-if="loading" class="flex flex-col items-center justify-center py-6 space-y-3">
              <svg class="w-8 h-8 text-odoo animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
              </svg>
              <span class="text-sm font-medium text-gray-600">Configuring firewall...</span>
            </div>

            <div v-else class="space-y-4 text-gray-600 leading-relaxed text-sm">
              <p>ePOS Proxy can receive print jobs from other devices on your local network.</p>
              <p>To allow this, a firewall rule must be created.</p>
              <p class="text-sm text-gray-600" v-if="props.os == 'darwin'">
                macOS uses an <strong>application-level firewall</strong>, which cannot be configured automatically by
                this application.
                If the macOS firewall is enabled, allow
                <strong class="text-amber-600">ePOS Proxy</strong> in
                <strong class="text-amber-600">System Settings → Privacy &amp; Security → Firewall</strong>
                to enable
                <strong class="text-red-600">network printing from other devices</strong>.
              </p>

              <p class="text-sm text-gray-600" v-else>
                If you choose
                <strong class="text-amber-600">"Not Now"</strong>,
                the application will continue to work normally, but
                <strong class="text-red-600">network printing from other devices will remain unavailable</strong>
                until firewall access is enabled.
              </p>
            </div>

            <!-- Error Display -->
            <div v-if="error && !loading" class="rounded-lg bg-red-50 border border-red-200 p-3">
              <p class="text-xs text-danger font-medium">{{ error }}</p>
            </div>

            <!-- Buttons -->
            <div class="flex gap-3 pt-2">
              <template v-if="authCancelled || props.os == 'darwin'">
                <button
                  class="flex-1 bg-stone-100 hover:bg-stone-200 text-stone-700 font-medium rounded-xl py-2.5 text-center cursor-pointer transition-colors"
                  @click="close">
                  Close
                </button>
              </template>
              <template v-else-if="!loading">
                <button
                  class="flex-1 border bg-odoo text-white hover:bg-odoo-dark font-medium rounded-xl py-2.5 text-center cursor-pointer transition-colors shadow-lg shadow-odoo/20"
                  @click="handleEnable">
                  Enable
                </button>
                <button
                  class="flex-1 bg-stone-100 hover:bg-stone-200 text-stone-700 font-medium rounded-xl py-2.5 text-center cursor-pointer transition-colors"
                  @click="handleNotNow">
                  Not Now
                </button>
              </template>
            </div>

          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import CloseButton from './close-button.vue'
import { ConfigureFirewall, SkipFirewallPrompt } from '../../wailsjs/go/main/App'

const emit = defineEmits(['notify'])
const props = defineProps({
  os: { type: String, default: "linux" }
})

const showDialog = ref(false)
const loading = ref(false)
const error = ref(null)
const authCancelled = ref(false)

onMounted(() => {
  EventsOn('open-firewall-prompt', () => {
    showDialog.value = true
    authCancelled.value = false
    error.value = null
    loading.value = false
  })
})

async function handleEnable() {
  if (loading.value) return
  loading.value = true
  error.value = null
  authCancelled.value = false

  try {
    await ConfigureFirewall()
    emit('notify', 'Network printing enabled successfully.', 'success')
    showDialog.value = false
  } catch (err) {
    const errorText = typeof err === 'string' ? err : err?.message || ''
    if (errorText.includes('authentication cancelled') || errorText.includes('canceled by the user') || errorText.includes('1223')) {
      authCancelled.value = true
    } else {
      error.value = errorText || 'Failed to configure firewall rule.'
    }
  } finally {
    loading.value = false
  }
}

async function handleNotNow() {
  if (loading.value) return
  loading.value = true
  try {
    await SkipFirewallPrompt()
    showDialog.value = false
  } catch (err) {
    console.error('Failed to skip firewall prompt:', err)
    showDialog.value = false
  } finally {
    loading.value = false
  }
}

function close() {
  if (loading.value) return
  showDialog.value = false
}
</script>
