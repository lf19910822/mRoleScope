<script setup lang="ts">
defineProps<{ visible: boolean }>()
</script>

<template>
  <div
    v-if="visible"
    class="app-request-loader app-request-loader--visible"
    role="status"
    aria-busy="true"
  >
    <span class="app-request-loader__tide" aria-hidden="true"><i></i><i></i><i></i></span>
    <span class="app-request-loader__copy">正在更新岗位数据</span>
  </div>
</template>

<style scoped>
.app-request-loader {
  align-items: center;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid rgba(185, 226, 234, 0.9);
  border-radius: 999px;
  box-shadow: 0 14px 32px -22px rgba(0, 91, 123, 0.72);
  color: #406980;
  display: flex;
  font-size: 0.73rem;
  font-weight: 700;
  gap: 8px;
  left: 50%;
  padding: 8px 12px;
  pointer-events: none;
  position: fixed;
  top: 16px;
  transform: translateX(-50%);
  z-index: 100;
}
.app-request-loader__tide {
  align-items: center;
  display: inline-flex;
  gap: 3px;
  height: 12px;
}
.app-request-loader__tide i {
  animation: app-request-loader-tide 900ms ease-in-out infinite;
  background: #007bab;
  border-radius: 99px;
  display: block;
  height: 3px;
  width: 13px;
}
.app-request-loader__tide i:nth-child(2) {
  animation-delay: 120ms;
  background: #008287;
  width: 8px;
}
.app-request-loader__tide i:nth-child(3) {
  animation-delay: 240ms;
  opacity: 0.45;
  width: 5px;
}

@keyframes app-request-loader-tide {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-2px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-request-loader__tide i {
    animation: none;
  }
}
</style>
