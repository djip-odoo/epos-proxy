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
  </div>
  <PrinterTypeModal v-if="showTypeSelect" v-model="showTypeSelect" :selectedPrinter="printer" @select="selectType" />
  <PrinterOptionDialog v-model="showPrinterOptionSelect" @select="selectOption"/>
</template>
<script setup>
import { ref } from 'vue'
import PrinterTypeModal from "../modal/printer-type-modal.vue"
import PrinterOptionDialog from "../modal/printer-option-dialog.vue"
import { executePrint } from "./printer-actions.js"

const props = defineProps({
  printer: Object,
  copiedIds: Object,
})

const emit = defineEmits(['copy', 'notify'])

const isTestPrinting = ref(false)
const showTypeSelect = ref(false)
const showPrinterOptionSelect = ref(false)

function onCopy() { emit('copy', props.printer) }

function onTest() {
  const type = props.printer.type
  if (type === 'ANY') {
    showTypeSelect.value = true
  } else if (type === 'OFFICE') {
    showPrinterOptionSelect.value = true
  } else {
    doTestPrint(type, {})
  }
}

function selectType(type) {
  showTypeSelect.value = false
  if (type === 'OFFICE') {
    showPrinterOptionSelect.value = true
  } else {
    doTestPrint(type, {})
  }
}

function selectOption(mode) {
  showPrinterOptionSelect.value = false
  doTestPrint('OFFICE', { duplex: mode })
}

async function doTestPrint(type, options) {
  isTestPrinting.value = true
  try {
    await executePrint(props.printer, type, options)
    emit('notify', `Test print sent to ${props.printer.name}`, 'success')
  } catch (err) {
    emit('notify', `Test failed: ${err.message}`, 'error')
  } finally {
    isTestPrinting.value = false
  }
}
</script>
