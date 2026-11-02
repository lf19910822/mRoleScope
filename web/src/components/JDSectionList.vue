<script setup lang="ts">
import { computed } from 'vue'

import HighlightedText from '@/components/HighlightedText.vue'
import { presentJDSection, type JDSectionItem } from '@/lib/jd-body-presentation'
import type { JDDetailBodySection, TechnologyHighlightRange } from '@/lib/highlighted-text'

const props = withDefaults(
  defineProps<{
    ranges?: readonly TechnologyHighlightRange[]
    section?: JDDetailBodySection
    text: string
  }>(),
  { ranges: () => [], section: 'requirements' },
)

const presentation = computed(() => presentJDSection(props.text))
const lead = computed(() =>
  presentation.value.kind === 'list'
    ? presentation.value.items.find((item) => item.kind === 'lead')
    : undefined,
)
const listItems = computed(() =>
  presentation.value.kind === 'list'
    ? presentation.value.items.filter(
        (item): item is Exclude<JDSectionItem, { kind: 'lead' }> => item.kind !== 'lead',
      )
    : [],
)
const listTag = computed(() =>
  listItems.value.every((item) => item.kind === 'bullet-item') ? 'ul' : 'ol',
)

function displayMarker(marker: string): string {
  return marker.match(/\d+/)?.[0] ?? marker.replace(/[、.．）)]/gu, '')
}
</script>

<template>
  <p v-if="presentation.kind === 'fallback'" class="jd-section-list__fallback">
    <HighlightedText
      :end="presentation.end"
      :ranges="ranges"
      :section="section"
      :source-text="text"
      :start="presentation.start"
    />
  </p>
  <div v-else class="jd-section-list">
    <p v-if="lead" class="jd-section-list__lead">
      <HighlightedText
        :end="lead.end"
        :ranges="ranges"
        :section="section"
        :source-text="text"
        :start="lead.start"
      />
    </p>
    <component :is="listTag" class="jd-section-list__items">
      <li v-for="item in listItems" :key="`${item.marker}-${item.start}`">
        <span class="jd-section-list__marker" :aria-label="`第 ${displayMarker(item.marker)} 条`">
          <span aria-hidden="true">{{ displayMarker(item.marker) }}</span>
        </span>
        <span class="jd-section-list__copy">
          <HighlightedText
            :end="item.end"
            :ranges="ranges"
            :section="section"
            :source-text="text"
            :start="item.start"
          />
        </span>
      </li>
    </component>
  </div>
</template>

<style scoped>
.jd-section-list__fallback,
.jd-section-list__lead {
  color: #52636d;
  line-height: 1.85;
  margin: 0;
  white-space: pre-wrap;
}
.jd-section-list__lead {
  margin-bottom: 11px;
}
.jd-section-list__items {
  display: grid;
  gap: 11px;
  list-style: none;
  margin: 0;
  padding: 0;
}
.jd-section-list__items li {
  align-items: center;
  background: linear-gradient(145deg, #f8fcfd, #eef8fa);
  border: 1px solid #dceef2;
  border-radius: 15px;
  color: #52636d;
  display: grid;
  gap: 12px;
  grid-template-columns: 30px minmax(0, 1fr);
  line-height: 1.82;
  padding: 15px;
  transition:
    border-color 240ms ease,
    box-shadow 280ms ease,
    transform 360ms cubic-bezier(0.16, 1, 0.3, 1);
}
.jd-section-list__marker {
  align-items: center;
  align-self: center;
  background: rgba(234, 247, 250, 0.92);
  border: 1.5px solid #007bab;
  border-radius: 999px;
  box-shadow: inset 0 0 0 3px rgba(255, 255, 255, 0.86);
  box-sizing: border-box;
  color: #196582;
  display: inline-flex;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 1.1rem;
  font-weight: 900;
  height: 30px;
  justify-content: center;
  line-height: 1;
  width: 30px;
}
.jd-section-list__copy {
  min-width: 0;
}

@media (hover: hover) and (pointer: fine) {
  .jd-section-list__items li:hover {
    border-color: #b9e2ea;
    box-shadow: 0 22px 36px -25px rgba(0, 91, 123, 0.72);
    transform: scale(1.02);
  }
}
</style>
