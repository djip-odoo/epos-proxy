<template>
  <!-- App root: renders the printer list and mounts the global docs modal once. -->
  <PrinterList />
  <!-- Hook-driven docs modal — no props needed, controlled via useDocs() anywhere. -->
  <StepModal />
</template>

<script setup>
import PrinterList from './printer-list.vue'
import StepModal from './modal/step-modal.vue'
</script>
