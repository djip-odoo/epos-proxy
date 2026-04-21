<template>
    <button @click="props.openDocs" name="Docs" class="absolute top-3 right-3
              w-12 h-12 flex items-center justify-center
              rounded-full bg-blue-600
              cursor-pointer
              text-white shadow-md
              hover:bg-blue-700 hover:shadow-lg
              active:scale-95
              transition-all duration-200" aria-label="Documentation">
        <!-- Document Icon -->
        <svg xmlns="http://www.w3.org/2000/svg" class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor"
            stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round"
                d="M7 3h7l5 5v13a1 1 0 01-1 1H7a2 2 0 01-2-2V5a2 2 0 012-2z" />
            <path stroke-linecap="round" stroke-linejoin="round" d="M14 3v5h5" />
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 13h6M9 17h4M9 9h2" />
        </svg>
    </button>
</template>
<script setup>
const props = defineProps({
    openDocs: {
        type: Function,
        required: true
    }
})
</script>