<script setup lang="ts">
import QRCodeVue from 'qrcode.vue'

defineProps<{
  value: string
  size?: number
}>()
</script>

<template>
  <div class="qr-code-wrapper">
    <QRCodeVue
      :value="value"
      :size="size || 200"
      level="M"
      render-as="svg"
    />
  </div>
</template>

<style scoped>
.qr-code-wrapper {
  display: flex;
  justify-content: center;
  padding: 1rem;
  background: white;
  border-radius: 8px;
}
</style>
