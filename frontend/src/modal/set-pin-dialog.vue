<template>
  <teleport to="body">
    <transition enter-active-class="transition duration-200 ease-out" enter-from-class="opacity-0"
      enter-to-class="opacity-100" leave-active-class="transition duration-150 ease-in" leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div v-if="show" class="fixed inset-0 z-50 bg-white flex flex-col">
        <div v-if="show" class="fixed inset-0 z-50 bg-white">
          <!-- Content -->
          <div class="h-[calc(100vh-73px)] flex items-center justify-center px-6">
            <div class="w-full max-w-md overflow-y-auto h-[80vh]" style="width: 20rem;">
              <div v-if="isVerifyMode"
                class="w-16 h-16 mx-auto mb-6 bg-odoo/10 text-odoo rounded-full flex items-center justify-center">
                <svg class="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
              </div>

              <h2 class="text-center text-2xl font-semibold text-gray-900 mb-3">
                {{ isVerifyMode ? 'Unlock Dashboard' : 'Create PIN' }}
              </h2>

              <p class="text-center text-sm text-gray-500 mb-8">
                <template v-if="isVerifyMode">
                  Enter the 4-digit PIN configured by the administrator.
                </template>

                <template v-else>
                  Enter a 4-digit PIN to protect dashboard access from LAN devices.
                </template>
              </p>

              <!-- PIN boxes -->
              <div class="flex justify-center gap-3 mb-6">
                <input v-for="(digit, index) in pinDigits" :key="index" :ref="el => pinRefs[index] = el"
                  v-model="pinDigits[index]" type="text" inputmode="numeric" maxlength="1"
                  class="w-14 h-14 border border-gray-300 rounded-lg text-center text-2xl font-mono focus:outline-none focus:ring-2 focus:ring-odoo focus:border-odoo transition-all"
                  :class="{ 'border-danger ring-1 ring-danger': error }" @input="handleDigitInput(index, $event)"
                  @keydown.backspace="handleBackspace(index)" @paste="handlePaste" @keyup.enter="submit" />
              </div>

              <Numpad :pin="pinDigits" :submit="submit" @update:pin="pinDigits = $event" />

              <div v-if="error" class="text-danger text-sm text-center mb-4">
                {{ error }}
              </div>

              <button @click="submit" :disabled="loading || pin.length !== 4"
                class="w-full rounded-lg px-4 py-3 mb-3 text-sm font-medium bg-odoo text-white hover:bg-odoo-dark disabled:opacity-50 disabled:cursor-not-allowed transition-all">
                {{ loading
                  ? (isVerifyMode ? 'Verifying...' : 'Saving...')
                  : (isVerifyMode ? 'Unlock' : 'Save PIN')
                }}
              </button>
              <button v-if="isSetMode" @click="close" :disabled="loading"
                class="w-full rounded-lg px-4 py-3 text-sm font-medium bg-gray-100 text-gray-800 hover:bg-gray-200 disabled:opacity-50 disabled:cursor-not-allowed transition-all">
                Cancel
              </button>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import { ref, computed, watch, onMounted, nextTick } from 'vue'
import CloseButton from './close-button.vue'
import { connector, safeEventsOn, isDesktopApp } from '../connector'
import { useToast } from '../hooks/useToast.js'
import Numpad from '../components/numpad.vue'

// ── Props / Emits ──────────────────────────────────────────────────────────────
// modelValue represents `isAuthenticated`:
//   true  → user is authenticated (dialog hidden in verify mode)
//   false → user needs to authenticate (dialog shown in verify mode)
const props = defineProps({
  modelValue: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['update:modelValue'])

// ── Mode ───────────────────────────────────────────────────────────────────────
const isSetMode = computed(isDesktopApp)           // desktop app  → set PIN
const isVerifyMode = computed(() => !isDesktopApp())  // browser/LAN  → verify PIN

// ── State ──────────────────────────────────────────────────────────────────────
const internalShow = ref(false)   // only used in set-mode (menu-driven)
const error = ref(null)
const loading = ref(false)
const pinDigits = ref(['', '', '', ''])
const pinRefs = []

const pin = computed(() => pinDigits.value.join(''))

// Dialog is visible when:
//   set mode    → triggered by menu event  (internalShow)
//   verify mode → whenever NOT authenticated  (!modelValue)
const show = computed(() =>
  isSetMode.value ? internalShow.value : !props.modelValue
)

const { notify } = useToast()

// ── Auto-focus when verify dialog appears ──────────────────────────────────────
watch(show, (val) => {
  if (val) nextTick(() => pinRefs[0]?.focus())
})

// ── Lifecycle ──────────────────────────────────────────────────────────────────
onMounted(() => {
  // Set mode: triggered via native app menu event
  safeEventsOn('open-set-pin', () => {
    internalShow.value = true
    resetForm()
    nextTick(() => pinRefs[0]?.focus())
  })
})

// ── Helpers ────────────────────────────────────────────────────────────────────
function resetForm() {
  error.value = null
  pinDigits.value = ['', '', '', '']
}

function close() {
  internalShow.value = false  // only reachable in set-mode (close btn hidden in verify)
  resetForm()
}

function handleDigitInput(index, event) {
  const value = event.target.value.replace(/\D/g, '').slice(-1)
  pinDigits.value[index] = value
  error.value = null
  if (value && index < 3) {
    pinRefs[index + 1]?.focus()
  } else if (value && index === 3) {
    submit()
  }
}

function handleBackspace(index) {
  if (!pinDigits.value[index] && index > 0) {
    pinDigits.value[index - 1] = ''
    pinRefs[index - 1]?.focus()
  }
}

function handlePaste(event) {
  event.preventDefault()
  const pasted = event.clipboardData.getData('text').replace(/\D/g, '').slice(0, 4)
  pinDigits.value = ['', '', '', '']
  pasted.split('').forEach((digit, i) => { pinDigits.value[i] = digit })
  pinRefs[Math.min(pasted.length, 3)]?.focus()
  if (pasted.length === 4) submit()
}

// ── Submit ─────────────────────────────────────────────────────────────────────
async function submit() {
  if (pin.value.length !== 4) {
    error.value = 'PIN must be exactly 4 digits'
    return
  }

  loading.value = true
  error.value = null

  try {
    if (isSetMode.value) {
      await connector.setLANPin(pin.value)
      notify('LAN Access PIN set successfully', 'success')
      close()
    } else {
      const res = await connector.verifyPin(pin.value)
      // Store the token on the HTTP connector for subsequent requests
      if (connector.token !== undefined) {
        connector.token = res.token
      }
      // Signal parent that authentication succeeded
      emit('update:modelValue', true)
      resetForm()
    }
  } catch (err) {
    console.error(err)
    error.value = err?.message || 'Operation failed'
    if (isSetMode.value) notify(error.value, 'danger')
    pinDigits.value = ['', '', '', '']
    nextTick(() => pinRefs[0]?.focus())
  } finally {
    loading.value = false
  }
}
</script>
