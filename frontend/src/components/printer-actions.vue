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

    <button @click="showSettings = true" title="Settings"
      class="border rounded-lg text-sm px-3 py-2 cursor-pointer border-stone-300 text-stone-600 hover:bg-stone-50 hover:border-stone-400">
      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round"
          d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
        <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
      </svg>
    </button>
  </div>

  <PrinterSettingsDialog :show="showSettings" :printer="printer" @close="showSettings = false" @notify="(msg, type) => emit('notify', msg, type)" />
</template>
<script setup>
import { ref } from 'vue'
import { executePrint, copyPrinterFieldValue } from "./printer-actions.js"
import PrinterSettingsDialog from '../modal/printer-settings-dialog.vue'

const props = defineProps({
  printer: Object,
})

const copiedIds = ref({})
const emit = defineEmits(['notify'])

const isTestPrinting = ref(false)
const showSettings = ref(false)

async function onCopy() {
  try {
    await copyPrinterFieldValue(props.printer, 'ip', copiedIds.value)
  } catch (err) {
    emit('notify', `Copy failed: ${err.message}`, 'error')
  }
}

async function onTest() {
  isTestPrinting.value = true
  try {
    await executePrint(props.printer)
    emit('notify', `Test print sent to ${props.printer.name}`, 'success')
  } catch (err) {
    emit('notify', `Test failed: ${err.message}`, 'error')
  } finally {
    isTestPrinting.value = false
  }
}
</script>
