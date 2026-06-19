import { ref } from 'vue'
import getFixSteps from "./fix-step";
import getNetworkPrinterSteps from "./network-printer-steps";
import { OSDriver } from "../../wailsjs/go/main/App";

const show = ref(false)
const steps = ref([])
let os = ref(null)

OSDriver().then(result => {
  os.value = result
})

function openDocs(docSteps) {
  steps.value = docSteps
  show.value = true
}

function closeDocs() {
  show.value = false
}

export function useDocs() {
  return { show, steps, openDocs, closeDocs }
}

export function hasLibUsbErrorFix(error = "") {
  return error.toLowerCase().includes('libusb')
}

export function getFixErrorText() {

  if (os.value && os.value.toLowerCase().includes('windows')) {
    return 'Fix - Install WinUSB driver'
  }
  return 'Fix - Install libusb'
}

export function openFixModal(type, args) {
  let steps = []
  if (type === "ERROR") {
    steps = getFixSteps(os.value.toLowerCase(), args)
  } else if (type === "NETWORK") {
    steps = getNetworkPrinterSteps(os.value.toLowerCase())
  }
  if (steps.length) openDocs(steps)
}
