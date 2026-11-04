<script setup lang="ts">
import { NDatePicker, NPopover } from 'naive-ui'
import { computed, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    ariaLabel?: string
    modelValue: string
    placeholder?: string
  }>(),
  {
    ariaLabel: '选择日期',
    placeholder: '选择日期',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const isOpen = ref(false)
const popoverThemeOverrides = { boxShadow: 'none' }
const displayValue = computed(() => props.modelValue.split('-').join('.') || props.placeholder)

function updateDate(value: string | [string, string] | null): void {
  if (typeof value !== 'string') return
  emit('update:modelValue', value)
  isOpen.value = false
}

function clearDate(): void {
  emit('update:modelValue', '')
  isOpen.value = false
}
</script>

<template>
  <div class="filter-date-picker">
    <NPopover
      v-model:show="isOpen"
      content-class="filter-date-picker__popover"
      placement="bottom-start"
      raw
      :show-arrow="false"
      :theme-overrides="popoverThemeOverrides"
      trigger="click"
    >
      <template #trigger>
        <button
          type="button"
          class="filter-date-picker__trigger"
          :class="{ 'filter-date-picker__trigger--open': isOpen }"
          :aria-expanded="isOpen"
          :aria-label="ariaLabel"
          aria-haspopup="dialog"
          data-testid="filter-date-picker-trigger"
        >
          <svg aria-hidden="true" viewBox="0 0 24 24">
            <rect x="3.5" y="5.5" width="17" height="15" rx="2.5" />
            <path d="M7.5 3.5v4M16.5 3.5v4M3.5 10h17" />
          </svg>
          <span :class="{ 'filter-date-picker__placeholder': !modelValue }">{{
            displayValue
          }}</span>
          <svg class="filter-date-picker__chevron" aria-hidden="true" viewBox="0 0 24 24">
            <path d="m7 10 5 5 5-5" />
          </svg>
        </button>
      </template>

      <div class="filter-date-picker__calendar" role="dialog" :aria-label="ariaLabel">
        <NDatePicker
          :formatted-value="modelValue || null"
          :value-format="'yyyy-MM-dd'"
          panel
          type="date"
          :actions="null"
          :close-on-select="true"
          @update:formatted-value="updateDate"
        />
        <button
          type="button"
          class="filter-date-picker__clear"
          data-testid="filter-date-picker-clear"
          :disabled="!modelValue"
          @click="clearDate"
        >
          清除日期
        </button>
      </div>
    </NPopover>
  </div>
</template>

<style scoped>
.filter-date-picker {
  min-width: 0;
  width: 100%;
}

.filter-date-picker__trigger {
  align-items: center;
  background: #f0f8fd;
  border: 1px solid transparent;
  border-radius: 10px;
  color: #2e373c;
  cursor: pointer;
  display: flex;
  font-size: 0.88rem;
  gap: 9px;
  height: 40px;
  padding: 0 12px;
  text-align: left;
  transition:
    background-color 180ms ease,
    border-color 180ms ease,
    box-shadow 180ms ease;
  width: 100%;
}

.filter-date-picker__trigger:hover,
.filter-date-picker__trigger--open {
  background: #fff;
  border-color: #7dbbd2;
}

.filter-date-picker__trigger:focus-visible {
  border-color: #007bab;
  box-shadow: 0 0 0 3px rgba(0, 123, 171, 0.16);
  outline: 0;
}

.filter-date-picker__trigger > svg {
  color: #007bab;
  fill: none;
  flex: 0 0 auto;
  height: 17px;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.8;
  width: 17px;
}

.filter-date-picker__trigger > span {
  flex: 1;
  font-variant-numeric: tabular-nums;
  min-width: 0;
}

.filter-date-picker__trigger > .filter-date-picker__placeholder {
  color: #868d92;
}

.filter-date-picker__trigger > .filter-date-picker__chevron {
  color: #66808d;
  height: 15px;
  width: 15px;
}

.filter-date-picker__calendar {
  min-width: 276px;
}

.filter-date-picker__clear {
  background: transparent;
  border: 0;
  border-top: 1px solid #e3ebf0;
  color: #406980;
  cursor: pointer;
  display: block;
  font-size: 0.78rem;
  font-weight: 700;
  margin: 2px 4px 0;
  padding: 9px 0 3px;
  text-align: center;
  transition: color 160ms ease;
  width: calc(100% - 8px);
}

.filter-date-picker__clear:hover:not(:disabled) {
  color: #007bab;
}

.filter-date-picker__clear:disabled {
  color: #a3afb5;
  cursor: not-allowed;
}

.app-shell[data-color-scheme='dark'] .filter-date-picker__trigger {
  background: #1b3440;
  color: #c6dce1;
}

.app-shell[data-color-scheme='dark'] .filter-date-picker__trigger:hover,
.app-shell[data-color-scheme='dark'] .filter-date-picker__trigger--open {
  background: #203d48;
  border-color: #4da3ba;
}

.app-shell[data-color-scheme='dark']
  .filter-date-picker__trigger
  > .filter-date-picker__placeholder {
  color: #8eabb5;
}

:global(.filter-date-picker__popover) {
  background: #ffffff;
  border: 1px solid #cfe4eb !important;
  border-radius: 14px !important;
  box-shadow: 0 18px 42px -24px rgba(11, 55, 76, 0.46) !important;
  padding: 6px !important;
}

:global(.filter-date-picker__calendar > .n-date-panel) {
  background: transparent !important;
  box-shadow: none !important;
}

:global(.filter-date-picker__popover .n-date-panel--month) {
  background: #ffffff !important;
  box-shadow: 0 14px 28px -18px rgba(11, 55, 76, 0.44) !important;
  position: relative;
  z-index: 2;
}

:global(body:has(.app-shell[data-color-scheme='dark']) .filter-date-picker__popover) {
  background: #17303a;
  border-color: #31505c !important;
  box-shadow: 0 20px 46px -22px rgba(0, 0, 0, 0.7) !important;
}

:global(
  body:has(.app-shell[data-color-scheme='dark']) .filter-date-picker__popover .n-date-panel--month
) {
  background: #17303a !important;
  box-shadow: 0 16px 30px -16px rgba(0, 0, 0, 0.76) !important;
}

:global(body:has(.app-shell[data-color-scheme='dark']) .filter-date-picker__clear) {
  border-top-color: #294752;
  color: #9bb8c0;
}
</style>
