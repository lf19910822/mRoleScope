<script setup lang="ts">
// PROTOTYPE ONLY — Three JD-detail visual directions, switchable on the
// existing detail route with ?prototype=jd-detail&variant=A|B|C. The page
// deliberately uses the actual detail response; its marker/highlight helpers
// are visual stand-ins and must not be promoted to the production parser.
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import type { JobDetailResponse } from '@/api/jd-api'
import {
  formatActivityState,
  formatEducation,
  formatExperience,
  formatFirstDiscoveredDate,
} from '@/lib/job-display'

type PrototypeVariant = 'A' | 'B' | 'C'
type PrototypeSectionKey = 'responsibilities' | 'requirements' | 'supplemental'

type BodyItem = {
  marker?: string
  text: string
}

type BodySection = {
  items: BodyItem[]
  key: PrototypeSectionKey
  label: string
}

const props = defineProps<{ job: JobDetailResponse }>()

const route = useRoute()

const variant = computed<PrototypeVariant>(() => {
  const requested = route.query.variant
  return requested === 'B' || requested === 'C' ? requested : 'A'
})

const sections = computed<BodySection[]>(() => {
  const body = props.job.body
  return [
    { key: 'responsibilities', label: '岗位职责', items: splitForPrototype(body.responsibilities) },
    { key: 'requirements', label: '任职要求', items: splitForPrototype(body.requirements) },
    ...(body.supplemental
      ? [
          {
            key: 'supplemental' as const,
            label: '补充说明',
            items: splitForPrototype(body.supplemental),
          },
        ]
      : []),
  ]
})

const knownTechnologyLabels = computed(() =>
  props.job.technology_keywords
    .map((technology) => technology.label)
    .filter((label) => label.length > 1),
)
const technologyKeywordsExpanded = ref(false)
const hasCollapsibleTechnologyKeywords = computed(() => props.job.technology_keywords.length > 8)

function splitForPrototype(text: string): BodyItem[] {
  const ordinal = /(?:^|[\s；;])([1-9]\d?[、.．）)])(?=[^\d])/g
  const markers = Array.from(text.matchAll(ordinal)).map((match) => ({
    index: (match.index ?? 0) + match[0].lastIndexOf(match[1] ?? ''),
    marker: match[1] ?? '',
  }))
  const serials = markers.map(({ marker }) => Number.parseInt(marker, 10))
  const isConsecutive =
    markers.length >= 2 &&
    serials.every((value, index) => index === 0 || value === serials[index - 1]! + 1)

  if (isConsecutive) {
    return buildMarkedItems(text, markers)
  }

  const bullet = /(?:^|[\s；;])([-*•▪])[ \t]+/g
  const bullets = Array.from(text.matchAll(bullet)).map((match) => ({
    index: (match.index ?? 0) + match[0].lastIndexOf(match[1] ?? ''),
    marker: match[1] ?? '',
  }))
  if (bullets.length >= 2) {
    return buildMarkedItems(text, bullets)
  }
  return [{ text }]
}

function buildMarkedItems(
  text: string,
  markers: Array<{ index: number; marker: string }>,
): BodyItem[] {
  const items: BodyItem[] = []
  const leading = text.slice(0, markers[0]?.index ?? 0).trim()
  if (leading) items.push({ text: leading })
  for (const [index, marker] of markers.entries()) {
    const next = markers[index + 1]
    const itemText = text.slice(marker.index + marker.marker.length, next?.index).trim()
    if (itemText) items.push({ marker: marker.marker, text: itemText })
  }
  return items.length ? items : [{ text }]
}

function displayFloatingMarker(item: BodyItem, sourcePosition: number): string {
  // The floating-island direction treats the circle as an index, not a copy
  // of source punctuation. Keep an original ordinal where available, but
  // deliberately strip its visible `、`, `.`, or `)` suffix. A bullet starts
  // counting at one even when an unmarked lead-in precedes the source list.
  return item.marker?.match(/\d+/)?.[0] ?? String(sourcePosition)
}

function isSingleDigitFloatingMarker(item: BodyItem, sourcePosition: number): boolean {
  return displayFloatingMarker(item, sourcePosition).length === 1
}

function sourceItemPosition(items: BodyItem[], itemIndex: number): number {
  return items.slice(0, itemIndex + 1).filter((item) => item.marker !== undefined).length
}

function activateFloatingCard(event: PointerEvent): void {
  if (variant.value !== 'B' || event.pointerType !== 'mouse') return

  const card = event.currentTarget as HTMLElement | null
  if (!card) return

  card.classList.add('prototype-floating-card--active')
  card.style.setProperty('--prototype-scale', '1.05')
}

function onFloatingCardPointerMove(event: PointerEvent): void {
  if (variant.value !== 'B' || event.pointerType !== 'mouse') return

  const card = event.currentTarget as HTMLElement | null
  if (!card) return

  activateFloatingCard(event)

  const bounds = card.getBoundingClientRect()
  if (!bounds.width || !bounds.height) return

  const horizontal = (event.clientX - bounds.left) / bounds.width - 0.5
  const vertical = (event.clientY - bounds.top) / bounds.height - 0.5
  const rotation = 8

  card.style.setProperty('--prototype-rotate-x', `${-vertical * rotation}deg`)
  card.style.setProperty('--prototype-rotate-y', `${horizontal * rotation}deg`)
  card.style.setProperty('--prototype-glow-x', `${(horizontal + 0.5) * 100}%`)
  card.style.setProperty('--prototype-glow-y', `${(vertical + 0.5) * 100}%`)
}

function resetFloatingCardPointer(event: PointerEvent): void {
  const card = event.currentTarget as HTMLElement | null
  if (!card) return

  card.style.setProperty('--prototype-rotate-x', '0deg')
  card.style.setProperty('--prototype-rotate-y', '0deg')
  card.style.setProperty('--prototype-glow-x', '50%')
  card.style.setProperty('--prototype-glow-y', '50%')
  card.style.setProperty('--prototype-scale', '1')
  card.classList.remove('prototype-floating-card--active')
}

function highlightedFragments(text: string): Array<{ marked: boolean; text: string }> {
  const ranges: Array<{ end: number; start: number }> = []
  const foldedText = text.toLocaleLowerCase()
  for (const label of knownTechnologyLabels.value) {
    const foldedLabel = label.toLocaleLowerCase()
    let from = 0
    while (from < text.length) {
      const start = foldedText.indexOf(foldedLabel, from)
      if (start < 0) break
      const end = start + label.length
      if (isStandaloneTechnology(text, start, end, label)) ranges.push({ start, end })
      from = end
    }
  }
  const ordered = ranges
    .sort((left, right) => left.start - right.start || right.end - left.end)
    .filter((range, index, values) => index === 0 || range.start >= values[index - 1]!.end)
  const fragments: Array<{ marked: boolean; text: string }> = []
  let cursor = 0
  for (const range of ordered) {
    if (cursor < range.start)
      fragments.push({ marked: false, text: text.slice(cursor, range.start) })
    fragments.push({ marked: true, text: text.slice(range.start, range.end) })
    cursor = range.end
  }
  if (cursor < text.length) fragments.push({ marked: false, text: text.slice(cursor) })
  return fragments.length ? fragments : [{ marked: false, text }]
}

function isStandaloneTechnology(text: string, start: number, end: number, label: string): boolean {
  if (!/^[A-Za-z0-9+#.-]+$/.test(label)) return true
  const previous = text[start - 1] ?? ''
  const next = text[end] ?? ''
  return !/[A-Za-z0-9]/.test(previous) && !/[A-Za-z0-9]/.test(next)
}
</script>

<template>
  <main class="prototype-page" :class="`prototype-page--${variant.toLowerCase()}`">
    <article class="prototype-sheet">
      <header class="prototype-header">
        <div>
          <p class="prototype-kicker">职位档案 · JD DETAIL</p>
          <h1>{{ job.title }}</h1>
          <p class="prototype-location">{{ job.locations.join(' · ') || '地点未说明' }}</p>
        </div>
        <div class="prototype-statuses">
          <span class="prototype-status">{{ formatActivityState(job.activity_state) }}</span>
          <span v-if="job.is_urgent" class="prototype-urgent">紧急招聘</span>
        </div>
      </header>

      <dl class="prototype-facts">
        <div>
          <dt>主岗位族</dt>
          <dd>{{ job.main_job_family?.label ?? '未分类' }}</dd>
        </div>
        <div>
          <dt>学历要求</dt>
          <dd>{{ formatEducation(job.education) }}</dd>
        </div>
        <div>
          <dt>最低工作年限</dt>
          <dd>{{ formatExperience(job.experience) }}</dd>
        </div>
        <div>
          <dt>首次发现</dt>
          <dd>{{ formatFirstDiscoveredDate(job.first_discovered_date) }}</dd>
        </div>
      </dl>

      <div
        v-if="job.technology_keywords.length"
        class="prototype-tech-list"
        aria-label="技术关键词"
      >
        <span class="prototype-tech-list__label">技术关键词</span>
        <div
          :id="`prototype-technology-keywords-${job.id}`"
          class="prototype-tech-list__values"
          :class="{
            'prototype-tech-list__values--collapsed':
              hasCollapsibleTechnologyKeywords && !technologyKeywordsExpanded,
          }"
        >
          <em v-for="technology in job.technology_keywords" :key="technology.value">{{
            technology.label
          }}</em>
        </div>
        <button
          v-if="hasCollapsibleTechnologyKeywords"
          class="prototype-tech-list__toggle"
          type="button"
          :aria-controls="`prototype-technology-keywords-${job.id}`"
          :aria-expanded="technologyKeywordsExpanded"
          @click="technologyKeywordsExpanded = !technologyKeywordsExpanded"
        >
          {{
            technologyKeywordsExpanded
              ? '收起技术词'
              : `展开全部技术词（${job.technology_keywords.length}）`
          }}
        </button>
      </div>

      <div class="prototype-sections">
        <section v-for="section in sections" :key="section.key" class="prototype-section">
          <header class="prototype-section__header">
            <h2>{{ section.label }}</h2>
          </header>

          <ol v-if="section.items.some((item) => item.marker)" class="prototype-items">
            <li
              v-for="(item, itemIndex) in section.items"
              :key="`${item.marker ?? 'lead'}-${itemIndex}`"
              class="prototype-floating-card"
              @pointerenter="activateFloatingCard"
              @pointerleave="resetFloatingCardPointer"
              @pointermove="onFloatingCardPointerMove"
            >
              <div class="prototype-floating-card__body">
                <span
                  v-if="item.marker"
                  class="prototype-marker"
                  :aria-label="
                    variant === 'B'
                      ? `第 ${displayFloatingMarker(item, sourceItemPosition(section.items, itemIndex))} 条`
                      : undefined
                  "
                  :class="{
                    'prototype-marker--lead': !item.marker,
                    'prototype-marker--single-digit':
                      variant === 'B' &&
                      isSingleDigitFloatingMarker(
                        item,
                        sourceItemPosition(section.items, itemIndex),
                      ),
                  }"
                >
                  <span aria-hidden="true">{{
                    variant === 'B'
                      ? displayFloatingMarker(item, sourceItemPosition(section.items, itemIndex))
                      : item.marker
                  }}</span>
                  <span v-if="variant === 'B'" class="prototype-marker__screen-reader"
                    >第
                    {{ displayFloatingMarker(item, sourceItemPosition(section.items, itemIndex)) }}
                    条</span
                  >
                </span>
                <span v-else class="prototype-marker prototype-marker--lead" aria-hidden="true"
                  >—</span
                >
                <span class="prototype-copy">
                  <template
                    v-for="(fragment, fragmentIndex) in highlightedFragments(item.text)"
                    :key="fragmentIndex"
                  >
                    <mark v-if="fragment.marked">{{ fragment.text }}</mark
                    ><template v-else>{{ fragment.text }}</template>
                  </template>
                </span>
              </div>
            </li>
          </ol>
          <p v-else class="prototype-fallback">
            <template
              v-for="(fragment, fragmentIndex) in highlightedFragments(
                section.items[0]?.text ?? '',
              )"
              :key="fragmentIndex"
            >
              <mark v-if="fragment.marked">{{ fragment.text }}</mark
              ><template v-else>{{ fragment.text }}</template>
            </template>
          </p>
        </section>
      </div>
    </article>
  </main>
</template>

<style scoped>
.prototype-page {
  --ink: #121c22;
  --slate: #52636d;
  --sea-glass: #eaf7fa;
  --tide: #007bab;
  --lagoon: #008287;
  --mint: #dff4f0;
  margin: 0 auto;
  max-width: 1080px;
  padding: 38px 32px 132px;
}

.prototype-sheet {
  background: rgba(255, 255, 255, 0.93);
  box-shadow: 0 22px 50px -34px rgba(11, 55, 76, 0.42);
  overflow: hidden;
}
.prototype-header {
  align-items: flex-start;
  background: linear-gradient(126deg, #edf9fc 0%, #fff 72%);
  display: flex;
  justify-content: space-between;
  padding: 36px 42px 31px;
}
.prototype-kicker {
  color: var(--tide);
  font-size: 0.7rem;
  font-weight: 760;
  letter-spacing: 0.14em;
  margin: 0;
}
h1 {
  color: var(--ink);
  font-size: clamp(2.1rem, 5vw, 3.45rem);
  letter-spacing: -0.055em;
  line-height: 1.05;
  margin: 9px 0 9px;
}
.prototype-location {
  color: #66737a;
  margin: 0;
}
.prototype-statuses {
  align-items: flex-end;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.prototype-status,
.prototype-urgent {
  border-radius: 999px;
  font-size: 0.74rem;
  font-style: normal;
  font-weight: 700;
  padding: 6px 10px;
  white-space: nowrap;
}
.prototype-status {
  background: #e5f5ee;
  color: #00734a;
}
.prototype-urgent {
  background: #fff2d3;
  color: #966500;
}

.prototype-facts {
  border-bottom: 1px solid #e3ebf0;
  border-top: 1px solid #e3ebf0;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin: 0;
  padding: 16px 42px;
}
.prototype-facts div + div {
  border-left: 1px solid #e3ebf0;
  padding-left: 17px;
}
.prototype-facts dt {
  color: #72818a;
  font-size: 0.7rem;
  font-weight: 700;
  margin-bottom: 4px;
}
.prototype-facts dd {
  color: #2e373c;
  font-size: 0.88rem;
  font-weight: 650;
  margin: 0;
}
.prototype-facts div:first-child dd {
  color: var(--tide);
}

.prototype-tech-list {
  border-bottom: 1px solid #e3ebf0;
  padding: 14px 42px;
}
.prototype-tech-list__label {
  color: #72818a;
  display: block;
  font-size: 0.71rem;
  font-weight: 700;
  margin: 0 0 7px;
}
.prototype-tech-list__values {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.prototype-tech-list__values em {
  background: #eef7fb;
  border-radius: 8px;
  color: #406980;
  font-size: 0.72rem;
  font-style: normal;
  font-weight: 650;
  padding: 4px 7px;
}
.prototype-tech-list__toggle {
  display: none;
}

.prototype-sections {
  padding: 6px 42px 42px;
}
.prototype-section {
  border-top: 1px solid #e6edf1;
  padding: 29px 0 0;
}
.prototype-section:first-child {
  border-top: 0;
}
.prototype-section + .prototype-section {
  margin-top: 30px;
}
.prototype-section__header {
  align-items: center;
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
}
.prototype-section__header h2 {
  color: var(--ink);
  font-size: 1.03rem;
  margin: 0;
}

.prototype-items {
  list-style: none;
  margin: 0;
  padding: 0;
}
.prototype-items li {
  color: var(--slate);
  font-size: 0.95rem;
  line-height: 1.82;
}
.prototype-marker {
  align-items: center;
  display: inline-flex;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.75rem;
  font-weight: 800;
  justify-content: center;
}
.prototype-marker__screen-reader {
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  height: 1px;
  overflow: hidden;
  position: absolute;
  white-space: nowrap;
  width: 1px;
}
.prototype-copy {
  min-width: 0;
}
mark {
  background: var(--mint);
  border-bottom: 2px solid rgba(0, 130, 135, 0.54);
  border-radius: 3px;
  color: #126368;
  padding: 0 0.1em;
}
.prototype-fallback {
  color: var(--slate);
  line-height: 1.85;
  margin: 0;
  white-space: pre-wrap;
}

/* A — Tide ledger: one restrained reading rail for the full section. */
.prototype-page--a .prototype-sheet {
  border-radius: 18px;
}
.prototype-page--a .prototype-items {
  border-left: 2px solid #c5e9f2;
  display: grid;
  gap: 10px;
  padding-left: 19px;
}
.prototype-page--a .prototype-items li {
  display: grid;
  gap: 13px;
  grid-template-columns: 31px minmax(0, 1fr);
  position: relative;
}
.prototype-page--a .prototype-items li::before {
  background: var(--tide);
  border-radius: 50%;
  content: '';
  height: 7px;
  left: -24px;
  position: absolute;
  top: 0.82rem;
  width: 7px;
}
.prototype-page--a .prototype-marker {
  background: #e6f5fa;
  border-radius: 7px;
  color: var(--tide);
  height: 25px;
  margin-top: 2px;
}
.prototype-page--a .prototype-marker--lead {
  background: transparent;
  color: #95a4aa;
}

/* B — Floating islands: one reading-sized island per source item. */
.prototype-page--b .prototype-sheet {
  border-radius: 26px;
}
.prototype-page--b .prototype-section__header {
  background: #f3fafc;
  border-radius: 12px;
  padding: 10px 13px;
}
.prototype-page--b .prototype-items {
  display: grid;
  gap: 11px;
  grid-template-columns: 1fr;
}
.prototype-page--b .prototype-items li {
  --prototype-glow-x: 50%;
  --prototype-glow-y: 50%;
  --prototype-rotate-x: 0deg;
  --prototype-rotate-y: 0deg;
  --prototype-scale: 1;
  perspective: 1000px;
  position: relative;
  z-index: 0;
}
.prototype-page--b .prototype-items li.prototype-floating-card--active {
  z-index: 1;
}
.prototype-page--b .prototype-floating-card__body {
  background:
    radial-gradient(
      circle at var(--prototype-glow-x) var(--prototype-glow-y),
      rgba(255, 255, 255, 0.95),
      rgba(255, 255, 255, 0) 42%
    ),
    linear-gradient(145deg, #f8fcfd, #eef8fa);
  border: 1px solid #dceef2;
  border-radius: 15px;
  display: grid;
  align-items: center;
  gap: 12px;
  grid-template-columns: 34px minmax(0, 1fr);
  padding: 15px;
  position: relative;
  transform: rotateX(var(--prototype-rotate-x)) rotateY(var(--prototype-rotate-y))
    scale(var(--prototype-scale));
  transform-style: preserve-3d;
  transition:
    border-color 240ms ease,
    box-shadow 280ms ease,
    transform 360ms cubic-bezier(0.16, 1, 0.3, 1);
  will-change: transform;
  z-index: 1;
}
.prototype-page--b
  .prototype-items
  li.prototype-floating-card--active
  .prototype-floating-card__body {
  border-color: #b9e2ea;
  box-shadow: 0 22px 36px -25px rgba(0, 91, 123, 0.72);
}
.prototype-page--b .prototype-marker {
  align-self: center;
  background: rgba(234, 247, 250, 0.92);
  border-radius: 999px;
  box-sizing: border-box;
  border: 1.5px solid var(--tide);
  box-shadow: inset 0 0 0 3px rgba(255, 255, 255, 0.86);
  color: #196582;
  font-size: 0.9rem;
  font-weight: 900;
  height: 30px;
  line-height: 1;
  margin-top: 0;
  min-width: 30px;
  padding: 0;
  width: 30px;
}
.prototype-page--b .prototype-marker--lead {
  border-color: #a9bec5;
  color: #66818a;
  visibility: hidden;
}
.prototype-page--b .prototype-marker--single-digit {
  font-size: 1.1rem;
}
.prototype-page--b .prototype-fallback {
  background: #f4fafb;
  border-radius: 14px;
  padding: 17px;
}
.prototype-page--b .prototype-marker {
  position: relative;
  z-index: 1;
}
.prototype-page--b .prototype-copy {
  position: relative;
  transform: translateZ(16px);
}

/* Variants A/C retain their direct marker-and-copy layout. */
.prototype-page--a .prototype-floating-card__body,
.prototype-page--c .prototype-floating-card__body {
  display: contents;
}

/* This prototype intentionally keeps its selected 3D interaction even when
   the app-wide motion fallback is active. */
@media (prefers-reduced-motion: reduce) {
  .prototype-page--b .prototype-floating-card__body {
    transition-duration: 240ms, 280ms, 360ms !important;
  }
}

/* C — Margin index: source order moves into a wide editorial gutter. */
.prototype-page--c .prototype-sheet {
  border-radius: 4px;
  box-shadow: 0 26px 54px -38px rgba(18, 28, 34, 0.48);
}
.prototype-page--c .prototype-sections {
  padding-left: 0;
  padding-right: 0;
}
.prototype-page--c .prototype-section {
  padding-left: 42px;
  padding-right: 42px;
}
.prototype-page--c .prototype-section__header {
  margin-left: -42px;
  padding-left: 42px;
}
.prototype-page--c .prototype-section__ordinal {
  color: var(--lagoon);
  font-size: 0.82rem;
  min-width: 27px;
}
.prototype-page--c .prototype-items {
  display: grid;
  gap: 0;
}
.prototype-page--c .prototype-items li {
  border-top: 1px dashed #d9e8eb;
  display: grid;
  gap: 21px;
  grid-template-columns: 48px minmax(0, 1fr);
  padding: 15px 0;
}
.prototype-page--c .prototype-items li:first-child {
  border-top: 2px solid var(--ink);
}
.prototype-page--c .prototype-marker {
  color: var(--tide);
  font-size: 0.82rem;
  justify-content: flex-start;
  padding-top: 3px;
}
.prototype-page--c .prototype-marker--lead {
  color: #9aafb6;
}
.prototype-page--c .prototype-fallback {
  border-left: 3px solid #b9e5e3;
  padding-left: 17px;
}

@media (max-width: 700px) {
  .prototype-page {
    padding: 32px 15px 56px;
  }
  .prototype-header {
    flex-direction: column;
    gap: 12px;
    padding: 29px 22px 24px;
  }
  .prototype-header > :first-child {
    width: 100%;
  }
  .prototype-statuses {
    align-items: flex-start;
    flex-direction: row;
    gap: 6px;
    order: -1;
  }
  h1 {
    font-size: clamp(1.75rem, 7.2vw, 1.9rem);
    line-height: 1.12;
  }
  .prototype-facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    padding: 16px 22px;
    row-gap: 15px;
  }
  .prototype-facts div:nth-child(3) {
    border-left: 0;
    padding-left: 0;
  }
  .prototype-tech-list {
    padding-left: 22px;
    padding-right: 22px;
  }
  .prototype-tech-list__values--collapsed {
    max-height: 48px;
    overflow: hidden;
  }
  .prototype-tech-list__toggle {
    background: transparent;
    border: 0;
    color: var(--tide);
    cursor: pointer;
    display: inline-flex;
    font: inherit;
    font-size: 0.78rem;
    font-weight: 700;
    margin: 10px 0 0;
    padding: 2px 0;
  }
  .prototype-sections {
    padding-left: 22px;
    padding-right: 22px;
  }
  .prototype-page--c .prototype-section {
    padding-left: 22px;
    padding-right: 22px;
  }
  .prototype-page--c .prototype-section__header {
    margin-left: -22px;
    padding-left: 22px;
  }
}
</style>
