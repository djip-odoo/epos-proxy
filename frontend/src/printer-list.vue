<template>
  <div>
    <div
        class="w-full max-w-full sm:max-w-md md:max-w-lg lg:max-w-xl bg-white/85 rounded-2xl shadow-lg overflow-hidden px-4 sm:px-6 py-2 sm:py-4">
      <PrinterFilter v-model="activeFilter" @refresh="updatePrinters" :loading="isUpdating"/>
      <div v-if="printers.length || unavailablePrinters.length" class="p-6 overflow-y-auto max-h-[60vh]">
        <ul class="divide-y divide-gray-300">
          <li v-for="printer in printers" :key="printer.id" class="text-left first:pt-0 py-6 last:pb-0 relative">
            <div class="flex items-center gap-2">
              <span class="w-3 h-3 rounded-full shrink-0" :class="getPrinterStatusClass(printer)"></span>
              <div class="flex items-center gap-2 min-w-0 flex-1">
                <span class="font-medium text-gray-900 break-all">
                  {{ printer.name }}
                </span>
                <button
                  @click="copyField(printer, 'name')"
                  class="text-gray-400 hover:text-gray-700 text-xs px-1 cursor-pointer"
                  title="Copy name"
                >
                  <img
                    v-if="copiedIds[printer.id]?.name"
                    :src="done_svg"
                    alt="Copied"
                    class="w-4 h-4"
                  />
                  <img
                    v-else
                    :src="copy_svg"
                    alt="Copy"
                    class="w-4 h-4"
                  />
                </button>
              </div>
              <span class="px-2 py-1 text-xs font-semibold rounded bg-gray-100 text-gray-800">{{ printer.type }}</span>
              <span
                  v-if="printer.isLAN"
                  @click="removeLanPrinter(printer)"
                  class="text-gray-600 hover:text-danger cursor-pointer text-xl font-bold"
                  title="Remove printer"
              >×</span>
            </div>
            <div class="text-slate-600 mt-2 text-sm break-all">{{ printer.ip }}</div>
            <PrinterActions
              :printer="printer"
              :copiedIds="copiedIds"
              :testPrintIds="testPrintIds"
              @copy="copyField"
              @test="testPrint"
            />
          </li>

          <li v-for="printer in unavailablePrinters" :key="printer.name"
              class="text-left first:pt-0 py-6 last:pb-0 relative">
            <div class="flex items-center gap-2">
              <span class="w-3 h-3 rounded-full shrink-0 bg-danger"></span>
              <span class="min-w-0 font-medium text-gray-900">{{ printer.name }}</span>
            </div>
            <div class="text-danger mt-1 text-wrap">Unable to communicate with this printer: {{
                printer.errorMsg
              }}
            </div>
            <div v-if="hasLibUsbErrorFix(printer.errorMsg)" class="flex gap-2 mt-4 flex-wrap">
              <button
                  class="flex-1 border bg-odoo text-white hover:bg-odoo-dark rounded-lg px-4 py-2 text-center cursor-pointer"
                  @click="openFixModal(printer)"
              >{{ getFixErrorText(printer.errorMsg) }}
              </button>
            </div>
          </li>

        </ul>
      </div>

      <div v-if="loading" class="p-6">
        <div class="font-medium text-lg text-center">Searching for printers...</div>
      </div>
      <div v-else-if="!printers.length && !unavailablePrinters.length" class="p-6">
        <div class="font-medium text-lg text-center">No printers found</div>
        <div class="mt-2 text-gray-600 text-center">Make sure your printer is powered on and connected via USB.</div>
      </div>

      <div v-if="errorMsg">
        <div class="text-red-700 mt-4 text-center">Error: {{ errorMsg }}</div>
      </div>

      <StepModal v-model="showFixModal" :steps="fixSteps"/>

    </div>
  </div>
  <teleport to="body">
    <div v-if="showTypeSelect" class="fixed inset-0 bg-black/30 flex items-center justify-center z-50">
      <div class="bg-white rounded-xl p-6 w-80 shadow-lg">
        <div class="text-lg font-semibold mb-4 text-center">
          Select Printer Type
        </div>

        <div class="flex gap-3">
          <button
            class="flex-1 bg-gray-200 rounded-lg py-2 hover:bg-odoo hover:text-white"
            @click="selectType('EPOS')"
          >
            Thermal (EPOS)
          </button>

          <button
            class="flex-1 bg-gray-200 rounded-lg py-2 hover:bg-odoo hover:text-white"
            @click="selectType('PDF')"
          >
            Normal (PDF)
          </button>
        </div>

        <button
          class="mt-4 w-full text-sm text-gray-500 bg-red-100 rounded-lg py-2 hover:bg-red-200"
          @click="showTypeSelect = false"
        >
          Cancel
        </button>
      </div>
    </div>
  </teleport>
  <div class="mt-6 text-center">
    <div
        @click="showAddDialog = true"
        class="border-2 border-dashed border-gray-300 bg-gray-50 rounded-lg px-4 py-3 text-gray-600 hover:border-gray-400 hover:bg-gray-100 cursor-pointer"
    >+ Add Network Printer
    </div>
  </div>

  <NetworkIpDialog :show="showAddDialog" @close="onNetworkDialogClose"/>

  <teleport to="body">
    <transition
        enter-active-class="transition duration-300 ease-out"
        enter-from-class="opacity-0 translate-x-4"
        enter-to-class="opacity-100 translate-x-0"
        leave-active-class="transition duration-200 ease-in"
        leave-from-class="opacity-100 translate-x-0"
        leave-to-class="opacity-0 translate-x-4"
    >
      <div
          v-if="toast.show"
          class="fixed top-4 right-4 z-50 px-4 py-3 rounded-lg shadow-lg text-white text-sm max-w-xs"
          :class="toast.type === 'success' ? 'bg-success' : 'bg-danger'"
      >
        {{ toast.message }}
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {CheckLANPrinterStatus, ConfirmRemoveLANPrinter, Status} from '../wailsjs/go/main/App'
import {brewSteps, linuxSteps, zadigSteps} from "./modal/fix-step";
import StepModal from "./modal/step-modal.vue";
import NetworkIpDialog from "./modal/network-ip-dialog.vue";
import copy_svg from "./assets/images/copy.svg"
import done_svg from "./assets/images/done.svg"
import PrinterActions from './components/printer-actions.vue'
import {copyPrinterFieldValue, handleTestPrint} from "./components/printer-actions.js";
import PrinterFilter from './components/top-bar.vue'

const activeFilter = ref('EPOS')
const printers = ref([])
const unavailablePrinters = ref([])
const errorMsg = ref(null)
const loading = ref(true)
const copiedIds = ref({})
const testPrintIds = ref({})
const lanStatus = ref({})
const pendingChecks = ref(new Set())
const showFixModal = ref(false)
const fixPrinterName = ref(null)
const os = ref(null)
const showAddDialog = ref(false)
const toast = ref({ show: false, message: '', type: 'success' })
const showTypeSelect = ref(false)
const selectedPrinter = ref(null)

let toastTimeout = null
let intervalId = null
let isTabVisible = true
let isUpdating = ref(false)

const copyField = (printer, field) =>
  copyPrinterFieldValue(printer, field, {copiedIds, showToast})
const testPrint = (printer) =>
  handleTestPrint(printer, {testPrintIds, selectedPrinter, showTypeSelect, showToast})

const handleVisibilityChange = () => {
  isTabVisible = !document.hidden
  if (isTabVisible) updatePrinters()
}
function selectType(type) {
  showTypeSelect.value = false
  if (selectedPrinter.value) {
    executePrint(selectedPrinter.value, type)
  }
}

async function updatePrinters() {
  if (isUpdating.value) return

  isUpdating.value = true
  try {
    const res = await Status(activeFilter.value)
    printers.value = res.printers
    unavailablePrinters.value = res.unavailablePrinters
    errorMsg.value = res.errorMsg
    os.value = res.os
    loading.value = false

    // Check status for each LAN printer
    for (const printer of res.printers) {
      if (printer.isLAN && printer.lanIp) {
        checkLanPrinterStatus(printer.lanIp)
      }
    }
    
  } catch (error) {
    console.error('Failed to update printers:', error)
    errorMsg.value = 'Failed to retrieve printer status. Please try again.'
  }  finally{
    isUpdating.value = false
  }
}

function checkLanPrinterStatus(ip) {
  if (pendingChecks.value.has(ip)) return

  pendingChecks.value.add(ip)
  if (lanStatus.value[ip] === undefined) {
    lanStatus.value[ip] = 'loading'
  }
  CheckLANPrinterStatus(ip).then((online) => {
    lanStatus.value[ip] = online ? 'online' : 'offline'
  }).finally(() => {
    pendingChecks.value.delete(ip)
  })
}

function getPrinterStatusClass(printer) {
  if (!printer.isLAN) {
    return printer.online ? 'bg-success' : 'bg-danger'
  }
  const status = lanStatus.value[printer.lanIp]
  if (status === 'online') return 'bg-success'
  if (status === 'offline') return 'bg-danger'
  return 'bg-warning'
}

onMounted(() => {
  isTabVisible = true
  document.addEventListener('visibilitychange', handleVisibilityChange)
  updatePrinters()
})

onUnmounted(() => {
  clearInterval(intervalId)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})

const fixSteps = computed(() => {
  if (!showFixModal.value) {
    return []
  }

  if (isWindows()) return zadigSteps(fixPrinterName.value)
  if (isMac()) return brewSteps(fixPrinterName.value)
  if (isLinux()) return linuxSteps(fixPrinterName.value)
  return []
})

function hasLibUsbErrorFix(error="") {
  return error.toLowerCase().includes('libusb')
}


function isWindows() {
  return os.value && os.value.toLowerCase().includes('windows')
}

function isMac() {
  return os.value && os.value.toLowerCase().includes('darwin')
}

function isLinux() {
  return os.value && os.value.toLowerCase().includes('linux')
}


function getFixErrorText() {

  if (isWindows()) {
    return 'Fix - Install WinUSB driver'
  }

  if (isMac() || isLinux()) {
    return 'Fix - Install libusb'
  }

}

function openFixModal(printer) {
  fixPrinterName.value = printer.name
  showFixModal.value = true
}

function showToast(message, type = 'success') {
  if (toastTimeout) clearTimeout(toastTimeout)
  toast.value = { show: true, message, type }

  toastTimeout = setTimeout(() => {
    toast.value.show = false
  }, type === 'success' ? 2000: 3000)
}

async function removeLanPrinter(printer) {
  if (!printer.lanIp) return

  try {
    const removed = await ConfirmRemoveLANPrinter(printer.lanIp)
    if (removed) updatePrinters()
  } catch (err) {
    console.error('Failed to remove LAN printer:', err)
  }
}

function onNetworkDialogClose(shouldRefresh) {
  showAddDialog.value = false
  if (shouldRefresh) updatePrinters()
}
</script>