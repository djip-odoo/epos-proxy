<template>
  <!-- Full-screen WebView overlay -->
  <teleport to="body">
    <transition
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="opacity-0 scale-98"
      enter-to-class="opacity-100 scale-100"
      leave-active-class="transition duration-200 ease-in"
      leave-from-class="opacity-100 scale-100"
      leave-to-class="opacity-0 scale-98"
    >
      <div
        v-if="isOpen"
        id="customer-display-webview"
        class="fixed inset-0 z-[100] bg-black flex flex-col"
        style="transform-origin: center;"
      >
        <!-- iframe -->
        <iframe
          v-if="currentURL"
          :src="currentURL"
          class="flex-1 w-full border-none"
          sandbox="allow-same-origin allow-scripts allow-forms allow-popups allow-top-navigation-by-user-activation"
          allow="autoplay; fullscreen"
          @load="onIframeLoad"
          @error="onIframeError"
          ref="iframeRef"
          title="Customer Display WebView"
        ></iframe>

        <!-- Error / No-URL fallback -->
        <div v-else class="flex-1 flex items-center justify-center bg-gray-900 text-white">
          <div class="text-center max-w-sm px-6">
            <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-white/10 flex items-center justify-center">
              <svg class="w-8 h-8 text-white/60" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
            </div>
            <h2 class="text-xl font-semibold mb-2">No URL Configured</h2>
            <p class="text-white/60 text-sm mb-6">Configure a customer display URL in the application settings to use the WebView mode.</p>
            <button @click="exitWebView" class="px-4 py-2 bg-white/10 hover:bg-white/20 rounded-lg text-sm transition-colors cursor-pointer">
              Back to Settings
            </button>
          </div>
        </div>

        <!-- Load error overlay -->
        <transition
          enter-active-class="transition duration-200 ease-out"
          enter-from-class="opacity-0"
          enter-to-class="opacity-100"
        >
          <div v-if="loadError" class="absolute inset-0 flex items-center justify-center bg-gray-900/95 text-white z-10">
            <div class="text-center max-w-sm px-6">
              <div class="w-16 h-16 mx-auto mb-4 rounded-full bg-red-500/20 flex items-center justify-center">
                <svg class="w-8 h-8 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
                </svg>
              </div>
              <h2 class="text-lg font-semibold mb-2">Failed to Load</h2>
              <p class="text-white/60 text-sm mb-2 font-mono break-all">{{ currentURL }}</p>
              <p class="text-white/40 text-xs mb-6">Check that the URL is reachable and try again.</p>
              <div class="flex gap-3 justify-center">
                <button @click="retry" class="px-4 py-2 bg-white text-gray-900 rounded-lg text-sm font-medium hover:bg-gray-100 transition-colors cursor-pointer">
                  Retry
                </button>
                <button @click="exitWebView" class="px-4 py-2 bg-white/10 hover:bg-white/20 rounded-lg text-sm transition-colors cursor-pointer">
                  Back to Settings
                </button>
              </div>
            </div>
          </div>
        </transition>

        <!-- Hidden corner zones (4 corners, 80x80px each, fully transparent) -->
        <!-- Clicking any corner 5x within 5s triggers PIN dialog -->
        <div id="corner-tl" class="absolute top-0 left-0 w-20 h-20 z-20 cursor-default" @click="registerCornerClick('tl')" />
        <div id="corner-tr" class="absolute top-0 right-0 w-20 h-20 z-20 cursor-default" @click="registerCornerClick('tr')" />
        <div id="corner-bl" class="absolute bottom-0 left-0 w-20 h-20 z-20 cursor-default" @click="registerCornerClick('bl')" />
        <div id="corner-br" class="absolute bottom-0 right-0 w-20 h-20 z-20 cursor-default" @click="registerCornerClick('br')" />

      </div>
    </transition>

    <!-- PIN Recovery Dialog (rendered above WebView) -->
    <transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 scale-95"
      enter-to-class="opacity-100 scale-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100 scale-100"
      leave-to-class="opacity-0 scale-95"
    >
      <div v-if="showPinDialog" class="fixed inset-0 z-[200] flex items-center justify-center bg-black/80 backdrop-blur-sm">
        <div class="bg-white rounded-2xl shadow-2xl w-full max-w-xs mx-4 overflow-hidden">

          <!-- PIN dialog header -->
          <div class="bg-odoo px-6 py-4 text-center">
            <div class="w-10 h-10 mx-auto mb-2 bg-white/20 rounded-full flex items-center justify-center">
              <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
            </div>
            <h2 class="text-white font-semibold text-base">Admin Access</h2>
            <p class="text-white/70 text-xs mt-0.5">Enter PIN to exit customer display</p>
          </div>

          <div class="p-6">
            <!-- PIN boxes -->
            <div class="flex justify-center gap-3 mb-4">
              <input
                v-for="(digit, index) in pinDigits"
                :key="index"
                :ref="el => pinRefs[index] = el"
                v-model="pinDigits[index]"
                type="text"
                inputmode="numeric"
                maxlength="1"
                class="w-12 h-12 border border-gray-300 rounded-lg text-center text-xl font-mono focus:outline-none focus:ring-2 focus:ring-odoo focus:border-odoo transition-all"
                :class="{ 'border-danger ring-1 ring-danger': pinError }"
                @input="handlePinInput(index, $event)"
                @keydown.backspace="handlePinBackspace(index)"
                @keyup.enter="submitPin"
              />
            </div>

            <!-- Numpad -->
            <div class="grid grid-cols-3 gap-2 mb-4">
              <button
                v-for="key in numpadKeys"
                :key="key"
                @click="handleNumpadKey(key)"
                class="h-11 rounded-xl text-base font-medium transition-all cursor-pointer"
                :class="key === '⌫' || key === 'C'
                  ? 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                  : 'bg-gray-50 text-gray-800 hover:bg-odoo/10 active:bg-odoo/20'"
              >
                {{ key }}
              </button>
            </div>

            <!-- Error -->
            <p v-if="pinError" class="text-danger text-xs text-center mb-3">{{ pinError }}</p>

            <!-- Buttons -->
            <button
              @click="submitPin"
              :disabled="pinLoading || pin.length !== 4"
              class="w-full rounded-xl px-4 py-3 text-sm font-semibold bg-odoo text-white hover:bg-odoo-dark disabled:opacity-50 disabled:cursor-not-allowed transition-all mb-2"
            >
              {{ pinLoading ? 'Verifying…' : 'Unlock' }}
            </button>
            <button @click="cancelPin" class="w-full rounded-xl px-4 py-3 text-sm font-medium bg-gray-100 text-gray-700 hover:bg-gray-200 transition-all cursor-pointer">
              Cancel
            </button>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { connector, safeEventsOn } from '../connector'
import { useToast } from '../hooks/useToast'

const CLICK_WINDOW_MS = 5000   // 5 seconds to accumulate 5 clicks
const CLICKS_REQUIRED = 5

const emit = defineEmits(['exit'])
const { notify } = useToast()

// WebView state
const isOpen = ref(false)
const currentURL = ref(null)
const iframeRef = ref(null)
const loadError = ref(false)

// Corner-click state (shared across corners — any corner counts)
const cornerClickTimestamps = ref([])
let cornerResetTimer = null

// PIN dialog state
const showPinDialog = ref(false)
const pinDigits = ref(['', '', '', ''])
const pinRefs = []
const pinError = ref(null)
const pinLoading = ref(false)

const pin = computed(() => pinDigits.value.join(''))
const numpadKeys = ['1','2','3','4','5','6','7','8','9','C','0','⌫']

// ── Lifecycle ────────────────────────────────────────────────────────────────

onMounted(() => {
  // Listen for startup auto-open from Go backend
  safeEventsOn('open-customer-display-webview', (url) => {
    openWebView(url)
  })
  // Also listen for manual open-webview events emitted by the settings dialog
  window.addEventListener('cd-open-webview', onOpenWebViewEvent)
})

onUnmounted(() => {
  window.removeEventListener('cd-open-webview', onOpenWebViewEvent)
  clearCornerTimer()
})

function onOpenWebViewEvent(e) {
  openWebView(e.detail)
}

// ── Public API ───────────────────────────────────────────────────────────────

function openWebView(url) {
  if (!url) {
    notify('No customer display URL configured', 'danger')
    return
  }
  currentURL.value = url
  loadError.value = false
  isOpen.value = true
  // Go fullscreen so the iframe fills the entire screen
  connector.setWindowFullscreen(true).catch(err => {
    console.warn('[CustomerDisplay] Could not set fullscreen:', err)
  })
  console.info('[CustomerDisplay] WebView opened:', url)
}

function exitWebView() {
  isOpen.value = false
  currentURL.value = null
  loadError.value = false
  clearCornerTimer()
  cornerClickTimestamps.value = []
  // Restore window from fullscreen
  connector.setWindowFullscreen(false).catch(err => {
    console.warn('[CustomerDisplay] Could not exit fullscreen:', err)
  })
  console.info('[CustomerDisplay] WebView closed, returning to settings')
  // Reopen the settings dialog after closing
  nextTick(() => {
    window.dispatchEvent(new CustomEvent('cd-settings-reopened'))
  })
  emit('exit')
}

// ── iframe Events ─────────────────────────────────────────────────────────────

function onIframeLoad() {
  loadError.value = false
  console.info('[CustomerDisplay] iframe loaded:', currentURL.value)
}

function onIframeError() {
  loadError.value = true
  console.error('[CustomerDisplay] iframe failed to load:', currentURL.value)
}

function retry() {
  const url = currentURL.value
  currentURL.value = null
  loadError.value = false
  nextTick(() => {
    currentURL.value = url
  })
  console.info('[CustomerDisplay] Retrying URL:', url)
}

// ── Corner-Click Recovery ─────────────────────────────────────────────────────

function registerCornerClick(corner) {
  const now = Date.now()
  // Add click timestamp
  cornerClickTimestamps.value.push(now)
  // Prune old timestamps outside the window
  cornerClickTimestamps.value = cornerClickTimestamps.value.filter(t => now - t <= CLICK_WINDOW_MS)

  console.info(`[CustomerDisplay] Corner click [${corner}]: ${cornerClickTimestamps.value.length}/${CLICKS_REQUIRED}`)

  if (cornerClickTimestamps.value.length >= CLICKS_REQUIRED) {
    cornerClickTimestamps.value = []
    clearCornerTimer()
    openPinDialog()
    return
  }

  // Reset the counter after the window expires
  clearCornerTimer()
  cornerResetTimer = setTimeout(() => {
    cornerClickTimestamps.value = []
  }, CLICK_WINDOW_MS)
}

function clearCornerTimer() {
  if (cornerResetTimer) {
    clearTimeout(cornerResetTimer)
    cornerResetTimer = null
  }
}

// ── PIN Dialog ────────────────────────────────────────────────────────────────

function openPinDialog() {
  pinDigits.value = ['', '', '', '']
  pinError.value = null
  showPinDialog.value = true
  nextTick(() => pinRefs[0]?.focus())
  console.info('[CustomerDisplay] PIN recovery dialog opened')
}

function cancelPin() {
  showPinDialog.value = false
  pinDigits.value = ['', '', '', '']
  pinError.value = null
  console.info('[CustomerDisplay] PIN dialog cancelled, WebView remains active')
}

function handlePinInput(index, event) {
  const value = event.target.value.replace(/\D/g, '').slice(-1)
  pinDigits.value[index] = value
  pinError.value = null
  if (value && index < 3) {
    pinRefs[index + 1]?.focus()
  } else if (value && index === 3) {
    submitPin()
  }
}

function handlePinBackspace(index) {
  if (!pinDigits.value[index] && index > 0) {
    pinDigits.value[index - 1] = ''
    pinRefs[index - 1]?.focus()
  }
}

function handleNumpadKey(key) {
  if (key === 'C') {
    pinDigits.value = ['', '', '', '']
    pinError.value = null
    nextTick(() => pinRefs[0]?.focus())
    return
  }
  if (key === '⌫') {
    const lastFilled = [...pinDigits.value].reverse().findIndex(d => d !== '')
    if (lastFilled !== -1) {
      const idx = 3 - lastFilled
      pinDigits.value[idx] = ''
      nextTick(() => pinRefs[idx]?.focus())
    }
    return
  }
  // Digit
  const emptyIdx = pinDigits.value.findIndex(d => d === '')
  if (emptyIdx !== -1) {
    pinDigits.value[emptyIdx] = key
    if (emptyIdx < 3) {
      nextTick(() => pinRefs[emptyIdx + 1]?.focus())
    } else {
      nextTick(() => submitPin())
    }
  }
}

async function submitPin() {
  if (pin.value.length !== 4) return

  pinLoading.value = true
  pinError.value = null

  try {
    const valid = await connector.validateAdminPin(pin.value)
    if (valid) {
      console.info('[CustomerDisplay] Admin PIN verified — exiting WebView')
      showPinDialog.value = false
      exitWebView()
      // Emit event so printer-list opens the settings dialog
      setTimeout(() => {
        window.dispatchEvent(new Event('cd-settings-reopened'))
      }, 300)
    } else {
      pinError.value = 'Incorrect PIN. Please try again.'
      pinDigits.value = ['', '', '', '']
      nextTick(() => pinRefs[0]?.focus())
      console.warn('[CustomerDisplay] Admin PIN validation failed')
    }
  } catch (err) {
    pinError.value = err?.message || 'PIN verification failed'
    pinDigits.value = ['', '', '', '']
    nextTick(() => pinRefs[0]?.focus())
    console.error('[CustomerDisplay] PIN error:', err)
  } finally {
    pinLoading.value = false
  }
}

// ── Expose for parent ─────────────────────────────────────────────────────────
defineExpose({ openWebView, exitWebView, isOpen })
</script>
