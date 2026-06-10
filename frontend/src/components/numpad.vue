<template>
  <transition enter-active-class="transition duration-200" enter-from-class="opacity-0 scale-95"
    enter-to-class="opacity-100 scale-100" leave-active-class="transition duration-150"
    leave-from-class="opacity-100 scale-100" leave-to-class="opacity-0 scale-95">
    <div v-if="showNumpad" class="grid grid-cols-3 gap-2 m-3">
      <button v-for="n in 9" :key="n" @click="appendDigit(String(n))"
        class="h-12 rounded-lg border border-gray-300 hover:bg-gray-50 text-lg font-medium">
        {{ n }}
      </button>

      <button @click="clearPin" class="h-12 rounded-lg border border-gray-300 hover:bg-gray-50">
        C
      </button>

      <button @click="appendDigit('0')"
        class="h-12 rounded-lg border border-gray-300 hover:bg-gray-50 text-lg font-medium">
        0
      </button>

      <button @click="removeDigit" class="h-12 rounded-lg border border-gray-300 hover:bg-gray-50">
        ⌫
      </button>
    </div>
  </transition>
</template>
<script setup>
import { ref } from 'vue'

const showNumpad = ref(true)

const props = defineProps({
  pin: {
    type: Array,
    required: true
  },
  submit: {
    type: Function,
    required: true
  }
})

const emit = defineEmits(['update:pin'])

function appendDigit(digit) {
  const next = [...props.pin]

  const index = next.findIndex(v => !v)

  if (index === -1) {
    return
  }

  next[index] = digit

  emit('update:pin', next)

  if (index === 3) {
    props.submit()
  }
}

function removeDigit() {
  const next = [...props.pin]

  for (let i = 3; i >= 0; i--) {
    if (next[i]) {
      next[i] = ''
      emit('update:pin', next)
      break
    }
  }
}

function clearPin() {
  emit('update:pin', ["", "", "", ""])
}

</script>