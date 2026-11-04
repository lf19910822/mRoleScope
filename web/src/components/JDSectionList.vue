<script setup lang="ts">
import { computed } from 'vue'

import HighlightedText from '@/components/HighlightedText.vue'
import { presentJDSection, type JDSectionItem } from '@/lib/jd-body-presentation'
import type { JDDetailBodySection, TechnologyHighlightRange } from '@/lib/highlighted-text'

type ListCardItem = Extract<JDSectionItem, { kind: 'bullet-item' } | { kind: 'ordered-item' }>
type ListGroup = {
  heading?: Extract<JDSectionItem, { kind: 'group-heading' }>
  items: ListCardItem[]
}

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
const listGroups = computed<ListGroup[]>(() => {
  if (presentation.value.kind !== 'list') return []

  const groups: ListGroup[] = [{ items: [] }]
  for (const item of presentation.value.items) {
    if (item.kind === 'lead') continue
    if (item.kind === 'group-heading') {
      groups.push({ heading: item, items: [] })
      continue
    }
    groups[groups.length - 1]?.items.push(item)
  }
  return groups.filter((group) => group.items.length > 0)
})

function listTag(items: ListCardItem[]): 'ol' | 'ul' {
  return items.every((item) => item.kind === 'bullet-item') ? 'ul' : 'ol'
}

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
    <div
      v-for="(group, groupIndex) in listGroups"
      :key="group.heading?.start ?? groupIndex"
      class="jd-section-list__group"
    >
      <p v-if="group.heading" class="jd-section-list__group-heading">
        <HighlightedText
          :end="group.heading.end"
          :ranges="ranges"
          :section="section"
          :source-text="text"
          :start="group.heading.start"
        />
      </p>
      <component :is="listTag(group.items)" class="jd-section-list__items">
        <li
          v-for="item in group.items"
          :key="`${item.marker}-${item.start}`"
          class="jd-section-list__card"
        >
          <div class="jd-section-list__card-surface">
            <span
              class="jd-section-list__marker"
              :aria-label="`第 ${displayMarker(item.marker)} 条`"
            >
              <span aria-hidden="true">{{ displayMarker(item.marker) }}</span>
            </span>
            <div class="jd-section-list__copy">
              <HighlightedText
                v-if="!item.childItems || (item.childLeadEnd ?? item.start) > item.start"
                :end="item.childLeadEnd ?? item.end"
                :ranges="ranges"
                :section="section"
                :source-text="text"
                :start="item.start"
              />
              <ul v-if="item.childItems" class="jd-section-list__child-items">
                <li v-for="child in item.childItems" :key="`${child.marker}-${child.start}`">
                  <HighlightedText
                    :end="child.end"
                    :ranges="ranges"
                    :section="section"
                    :source-text="text"
                    :start="child.start"
                  />
                </li>
              </ul>
            </div>
          </div>
        </li>
      </component>
    </div>
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
.jd-section-list__group + .jd-section-list__group {
  margin-top: 24px;
}
.jd-section-list__group-heading {
  color: #406980;
  font-size: 0.79rem;
  font-weight: 750;
  letter-spacing: 0.04em;
  margin: 0 0 10px;
}
.jd-section-list__items {
  display: grid;
  gap: 11px;
  list-style: none;
  margin: 0;
  padding: 0;
}
.jd-section-list__items li {
  position: relative;
}
.jd-section-list__card-surface {
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
  position: relative;
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
  position: relative;
  width: 30px;
}
.jd-section-list__copy {
  min-width: 0;
  position: relative;
}
.jd-section-list__child-items {
  display: grid;
  gap: 7px;
  list-style: none;
  margin: 11px 0 0;
  padding: 0;
}
.jd-section-list__child-items > li {
  align-items: start;
  color: #456577;
  display: grid;
  gap: 9px;
  grid-template-columns: 7px minmax(0, 1fr);
}
.jd-section-list__child-items > li::before {
  background: #63b9cc;
  border-radius: 999px;
  content: '';
  height: 5px;
  margin-top: 0.73em;
  width: 5px;
}
</style>
