<template>
  <teleport to="body">
    <div v-if="modelValue" class="fixed inset-0 bg-black/30 flex items-center justify-center z-50">
      <div class="bg-white rounded-xl p-6 w-80 shadow-lg">
        <div class="text-lg font-semibold mb-4 text-center">
          Select Printer Type
        </div>

        <div class="flex gap-3">
          <button
            class="flex-1 bg-gray-200 rounded-lg py-2 hover:bg-odoo hover:text-white cursor-pointer"
            @click="select('EPOS')"
          >
            Thermal (EPOS)
          </button>

          <button
            class="flex-1 bg-gray-200 rounded-lg py-2 hover:bg-odoo hover:text-white cursor-pointer"
            @click="select('PDF')"
          >
            Normal (PDF)
          </button>
        </div>

        <button
          class="mt-4 w-full text-sm text-gray-500 bg-red-100 rounded-lg py-2 hover:bg-red-600 hover:text-white cursor-pointer"
          @click="close"
        >
          Cancel
        </button>
      </div>
    </div>
  </teleport>
</template>

<script setup id="h9m2qp">
const props = defineProps({
  modelValue: Boolean
})

const emit = defineEmits(['update:modelValue', 'select'])

function close() {
  emit('update:modelValue', false)
}

function select(type) {
  emit('select', type)
  close()
}
</script>