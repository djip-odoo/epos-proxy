<template>
  <div class="relative flex items-center">
    <!-- Overlay to close menu when clicking outside -->
    <div v-if="showMenu" class="fixed inset-0 z-40 bg-transparent" @click="showMenu = false" />

    <button @click="showMenu = !showMenu" class="p-1 rounded-lg hover:bg-odoo-dark/50 transition-colors cursor-pointer z-50" title="Settings">
      <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
      </svg>
    </button>
    
    <!-- Dropdown Menu -->
    <div v-if="showMenu" class="absolute right-0 top-full mt-2 w-56 rounded-xl bg-white text-gray-800 shadow-2xl border border-gray-150 z-50 overflow-hidden py-1">
      <button @click="handleDownloadLogs" class="w-full text-left px-4 py-2.5 hover:bg-gray-50 flex items-center gap-2 text-sm font-medium transition-colors cursor-pointer border-none bg-transparent">
        <svg class="w-4 h-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        Download Logs
      </button>
      
      <div class="px-4 py-2.5 hover:bg-gray-50 flex items-center justify-between border-t border-gray-100">
        <span class="text-sm font-medium flex items-center gap-2">
          <svg class="w-4 h-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
          Support Mode
        </span>
        <input type="checkbox" v-model="supportMode" @change="toggleSupportMode" class="w-4 h-4 rounded text-odoo accent-odoo cursor-pointer" />
      </div>

      <button @click="showAboutModal = true; showMenu = false" class="w-full text-left px-4 py-2.5 hover:bg-gray-50 flex items-center gap-2 text-sm font-medium border-t border-gray-100 transition-colors cursor-pointer border-none bg-transparent">
        <svg class="w-4 h-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        About
      </button>

      <button @click="handleQuit" class="w-full text-left px-4 py-2.5 hover:bg-red-50 text-danger flex items-center gap-2 text-sm font-medium border-t border-gray-100 transition-colors cursor-pointer border-none bg-transparent">
        <svg class="w-4 h-4 text-danger" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
        </svg>
        Quit
      </button>
    </div>
  </div>

  <!-- About Modal -->
  <teleport to="body">
    <transition enter-active-class="transition duration-200 ease-out" enter-from-class="opacity-0" enter-to-class="opacity-100" leave-active-class="transition duration-150 ease-in" leave-from-class="opacity-100" leave-to-class="opacity-0">
      <div v-if="showAboutModal" class="fixed inset-0 z-55 flex items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/55 backdrop-blur-sm" @click="showAboutModal = false" />
        <div class="relative bg-white rounded-2xl w-full max-w-xs shadow-2xl overflow-hidden border border-gray-100 p-6 text-center space-y-4">
          <h3 class="font-bold text-lg text-gray-900">About ePOS Proxy</h3>
          <p class="text-sm text-gray-600">Expose USB and network printers as HTTP endpoints</p>
          <div class="bg-gray-50 rounded-xl p-3 text-xs font-mono text-gray-500 text-left whitespace-pre-wrap">
            Version: 1.0.0
            Platform: Android
          </div>
          <button @click="showAboutModal = false" class="w-full bg-odoo text-white hover:bg-odoo-dark font-medium rounded-xl py-2 text-center transition-colors cursor-pointer border-none">
            Close
          </button>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import {
  DownloadLogs,
  IsAutostartEnabled,
  EnableAutostart,
  DisableAutostart,
  IsSupportModeEnabled,
  SetSupportMode,
  Quit
} from '../../bindings/epos-proxy/app'
import { useToast } from '../hooks/useToast.js'

const { notify } = useToast()

const showMenu = ref(false)
const showAboutModal = ref(false)
const supportMode = ref(false)
const autoStart = ref(false)

onMounted(() => {
  IsSupportModeEnabled().then(val => supportMode.value = val).catch(err => console.error(err))
  IsAutostartEnabled().then(val => autoStart.value = val).catch(err => console.error(err))
})

async function handleDownloadLogs() {
  showMenu.value = false
  try {
    await DownloadLogs()
    notify('Logs downloaded successfully', 'success')
  } catch (err) {
    console.error(err)
    notify('Failed to download logs', 'danger')
  }
}

async function toggleSupportMode() {
  try {
    await SetSupportMode(supportMode.value)
    notify(`Support mode ${supportMode.value ? 'enabled' : 'disabled'}`, 'success')
  } catch (err) {
    console.error(err)
    notify('Failed to update support mode', 'danger')
  }
}

async function toggleAutoStart() {
  try {
    if (autoStart.value) {
      await EnableAutostart()
    } else {
      await DisableAutostart()
    }
    notify(`Auto start ${autoStart.value ? 'enabled' : 'disabled'}`, 'success')
  } catch (err) {
    console.error(err)
    notify('Failed to update auto start', 'danger')
  }
}

async function handleQuit() {
  showMenu.value = false
  if (confirm('Stopping the proxy will prevent POS from printing receipts.\n\nAre you sure you want to quit?')) {
    try {
      await Quit()
    } catch (err) {
      console.error(err)
    }
  }
}
</script>
