<template>
  <div class="flex gap-2 mt-4 flex-wrap">
    <button @click="onCopy" class="flex-1 border text-sm rounded-lg px-3 py-2 cursor-pointer whitespace-nowrap" :class="copiedIds?.[printer.id]?.ip
      ? 'bg-success text-white'
      : 'bg-odoo text-white hover:bg-odoo-dark'">
      {{ copiedIds?.[printer.id]?.ip ? '✓ Copied!' : 'Copy IP' }}
    </button>

    <button @click="onTest" :disabled="isTestPrinting"
      class="flex-1 border rounded-lg text-sm px-3 py-2 cursor-pointer border-stone-300 text-stone-600 hover:bg-stone-50 hover:border-stone-400 disabled:opacity-50 disabled:cursor-not-allowed">
      {{ isTestPrinting ? 'Printing...' : 'Test' }}
    </button>
    <PrinterSettingsDialog :printer="printer" />
  </div>
</template>
<script setup>
import { ref } from 'vue'
import { executePrint, copyPrinterFieldValue } from "./printer-actions.js"
import PrinterSettingsDialog from '../modal/printer-settings-dialog.vue'
import { useToast } from '../hooks/useToast.js'

const props = defineProps({
  printer: Object,
})

const copiedIds = ref({})
const { notify } = useToast()

const isTestPrinting = ref(false)

async function onCopy() {
  try {
    await copyPrinterFieldValue(props.printer, 'ip', copiedIds.value)
  } catch (err) {
    notify(`Copy failed: ${err.message}`, 'danger')
  }
}

async function onTest() {
  isTestPrinting.value = true
  try {
    await executePrint(props.printer)
    notify(`Test print sent to ${props.printer.name}`, 'success')
  } catch (err) {
    notify(`Test failed: ${err.message}`, 'danger')
  } finally {
    isTestPrinting.value = false
  }
}
</script>
