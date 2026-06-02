<template>
  <div
    @click="showBluetoothDialog = true"
    class="mt-2 border-2 border-dashed border-blue-200 bg-blue-50 rounded-lg px-4 py-3 text-blue-600 hover:border-blue-400 hover:bg-blue-100 cursor-pointer flex items-center justify-center gap-2"
  >
    <span>🔵</span> Add Bluetooth Printer
  </div>
  <teleport to="body">
    <transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition duration-150 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
    >
      <div v-if="showBluetoothDialog" class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/75" @click="close"/>
        <div class="relative bg-white rounded-2xl w-full max-w-sm shadow-xl overflow-hidden p-6">

          <div class="flex items-center justify-between mb-4">
            <div class="flex items-center gap-2">
              <span class="text-xl">🔵</span>
              <div class="text-lg font-medium">Add Bluetooth Printer</div>
            </div>
            <CloseButton @click="close"/>
          </div>

          <!-- Scan Button -->
          <button
              @click="scan"
              :disabled="scanning"
              class="w-full border rounded-lg px-4 py-2 mb-4 text-sm cursor-pointer flex items-center justify-center gap-2 border-stone-300 text-stone-700 hover:bg-stone-50 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <span v-if="scanning" class="animate-spin inline-block w-4 h-4 border-2 border-current border-t-transparent rounded-full"></span>
            <span>{{ scanning ? 'Scanning...' : '🔍 Scan for Devices' }}</span>
          </button>

          <!-- Scan error -->
          <div v-if="scanError" class="text-danger text-xs mb-3 text-center">{{ scanError }}</div>

          <!-- Device list from scan -->
          <div v-if="devices.length" class="mb-4 border border-gray-200 rounded-lg overflow-hidden">
            <div class="text-xs text-gray-500 px-3 pt-2 pb-1 font-medium uppercase tracking-wide bg-gray-50">
              Paired
            </div>
            <ul class="divide-y divide-gray-100 max-h-44 overflow-y-auto">
              <li
                  v-for="device in devices"
                  :key="device.mac"
                  @click="selectDevice(device)"
                  class="flex items-center gap-3 px-3 py-2.5 cursor-pointer hover:bg-blue-50 transition-colors"
                  :class="selectedMac === device.mac ? 'bg-blue-50 ring-1 ring-inset ring-blue-400' : ''"
              >
                <span class="text-base">🖨️</span>
                <div class="flex-1 min-w-0">
                  <div class="text-sm font-medium text-gray-800 truncate">{{ device.name }}</div>
                  <div class="text-xs text-gray-400 font-mono">{{ device.mac }}</div>
                </div>
                <span v-if="selectedMac === device.mac" class="text-blue-500 text-sm font-bold">✓</span>
              </li>
            </ul>
          </div>

          <!-- Divider -->
          <div class="flex items-center gap-2 mb-3">
            <div class="flex-1 h-px bg-gray-200"></div>
            <span class="text-xs text-gray-400">or enter manually</span>
            <div class="flex-1 h-px bg-gray-200"></div>
          </div>

          <!-- Manual MAC input -->
          <input
              v-model="macInput"
              type="text"
              placeholder="MAC address (e.g. AA:BB:CC:DD:EE:FF)"
              class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-1 focus:ring-blue-400 focus:border-transparent mb-2"
              @keyup.enter="submit"
              @input="selectedMac = ''"
              ref="inputRef"
          />
          <input
              v-model="nameInput"
              type="text"
              placeholder="Printer name (optional)"
              class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-400 focus:border-transparent mb-3"
              @keyup.enter="submit"
          />

          <div v-if="error" class="text-danger text-sm mb-3">{{ error }}</div>

          <button
              @click="submit"
              :disabled="loading || (!macInput.trim() && !selectedMac)"
              class="w-full border rounded-lg px-4 py-2 cursor-pointer text-sm bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >{{ loading ? 'Connecting...' : 'Add Printer' }}
          </button>

        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import {ref, watch, nextTick} from 'vue'
import CloseButton from './close-button.vue'
import {AddBluetoothPrinter, ScanBluetoothPrinters} from '../../bindings/epos-proxy/app'

const emit = defineEmits(['refresh'])

const macInput = ref('')
const nameInput = ref('')
const error = ref(null)
const loading = ref(false)
const scanning = ref(false)
const scanError = ref(null)
const devices = ref([])
const selectedMac = ref('')
const inputRef = ref(null)
const showBluetoothDialog = ref(false)

watch(showBluetoothDialog, (val) => {
  if (val) {
    macInput.value = ''
    nameInput.value = ''
    error.value = null
    scanError.value = null
    devices.value = []
    selectedMac.value = ''
    nextTick(() => inputRef.value?.focus())
  }
})

function selectDevice(device) {
  selectedMac.value = device.mac
  macInput.value = device.mac
  nameInput.value = nameInput.value || device.name
}

async function scan() {
  scanning.value = true
  scanError.value = null
  devices.value = []
  try {
    const found = await ScanBluetoothPrinters()
    devices.value = found || []
    if (!devices.value.length) {
      scanError.value = 'No paired Bluetooth devices found. Pair your printer first via system Bluetooth settings.'
    }
  } catch (err) {
    scanError.value = err?.toString() || 'Scan failed'
  } finally {
    scanning.value = false
  }
}

function close(shouldRefresh = false) {
  error.value = null
  showBluetoothDialog.value = false
  if (shouldRefresh) emit('refresh')
}

async function submit() {
  const mac = (selectedMac.value || macInput.value).trim()
  if (!mac) {
    error.value = 'Please enter or scan a MAC address'
    return
  }

  loading.value = true
  error.value = null

  try {
    await AddBluetoothPrinter(mac, nameInput.value.trim())
    close(true)
  } catch (err) {
    error.value = err?.toString() || 'Failed to add Bluetooth printer'
  } finally {
    loading.value = false
  }
}
</script>
