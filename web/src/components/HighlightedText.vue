<script setup lang="ts">
import { computed } from 'vue'

import {
  fragmentHighlightedText,
  type JDDetailBodySection,
  type TechnologyHighlightRange,
} from '@/lib/highlighted-text'

const props = defineProps<{
  end: number
  ranges?: readonly TechnologyHighlightRange[]
  section: JDDetailBodySection
  sourceText: string
  start: number
}>()

const fragments = computed(() =>
  fragmentHighlightedText(props.sourceText, props.start, props.end, props.section, props.ranges),
)
</script>

<template>
  <template v-for="(fragment, index) in fragments" :key="`${index}-${fragment.text}`">
    <mark v-if="fragment.highlighted" class="technology-highlight">{{ fragment.text }}</mark>
    <template v-else>{{ fragment.text }}</template>
  </template>
</template>

<style scoped>
.technology-highlight {
  background: #dff4f0;
  border-bottom: 1px solid #008287;
  border-radius: 2px;
  color: inherit;
  padding: 0 0.08em;
}
</style>
