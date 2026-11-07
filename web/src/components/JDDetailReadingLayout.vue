<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import type {
  ComparisonBodySection,
  ComparisonFragment,
  JobDetailResponse,
  TrendDrilldownStatus,
} from '@/api/jd-api'
import HighlightedText from '@/components/HighlightedText.vue'
import {
  presentJDSection,
  type JDSectionItem,
  type JDSectionPresentation,
} from '@/lib/jd-body-presentation'
import {
  formatActivityState,
  formatEducation,
  formatExperience,
  formatFirstDiscoveredDate,
} from '@/lib/job-display'
import type { JDDetailBodySection } from '@/lib/highlighted-text'

type ListItem = Extract<JDSectionItem, { kind: 'bullet-item' } | { kind: 'ordered-item' }>

type ComparisonListLine =
  | { kind: 'lead' | 'group-heading'; text: string }
  | { children?: string[]; kind: 'item'; marker: string; text: string }

type ReadingSection = {
  id: string
  key: JDDetailBodySection
  label: string
  presentation: JDSectionPresentation
  text: string
}

const props = defineProps<{
  job: JobDetailResponse
  historicalContext?: { currentStatus: TrendDrilldownStatus }
}>()

const emit = defineEmits<{
  'retry-comparison': []
}>()

const dimensionComparisonExpanded = ref(false)
const bodyComparisonExpanded = ref(false)

function formatCurrentComparisonStatus(status: TrendDrilldownStatus): string {
  if (status === 'current') return '仍有效'
  if (status === 'changed') return '有改动'
  return '确认下线'
}

const sections = computed<ReadingSection[]>(() => {
  const { body } = props.job
  const candidates: Array<Pick<ReadingSection, 'key' | 'label' | 'text'>> = [
    { key: 'responsibilities', label: '岗位职责', text: body.responsibilities },
    { key: 'requirements', label: '任职要求', text: body.requirements },
    ...(body.supplemental
      ? [{ key: 'supplemental' as const, label: '补充说明', text: body.supplemental }]
      : []),
  ]

  return candidates.map((section) => ({
    ...section,
    id: `job-detail-reading-${section.key}`,
    presentation: presentJDSection(section.text),
  }))
})

const comparison = computed(() => props.job.comparison)
const dimensionChanges = computed(() =>
  comparison.value?.status === 'available' ? (comparison.value.dimension_changes ?? []) : [],
)
const bodyComparisons = computed(() =>
  comparison.value?.status === 'available' ? (comparison.value.body_sections ?? []) : [],
)
const bodyComparisonBySection = computed(() => {
  const entries = bodyComparisons.value.map((section) => [section.section, section] as const)
  return new Map(entries)
})

function bodyComparison(section: JDDetailBodySection): ComparisonBodySection | undefined {
  return bodyComparisonBySection.value.get(section)
}

function comparisonFragmentClass(fragment: ComparisonFragment): string {
  return `detail-comparison__fragment--${fragment.kind}`
}

function comparisonFragmentLabel(fragment: ComparisonFragment): string | undefined {
  if (fragment.kind === 'deleted') return '已删除'
  if (fragment.kind === 'inserted') return '最新新增'
  return undefined
}

function comparisonFragmentLines(fragment: ComparisonFragment): ComparisonListLine[] {
  const presentation = presentJDSection(fragment.text)
  if (presentation.kind === 'fallback') return []

  return presentation.items.map((item) => {
    if (item.kind === 'lead' || item.kind === 'group-heading') {
      return { kind: item.kind, text: item.text }
    }

    const mainText = item.childItems
      ? Array.from(fragment.text)
          .slice(item.start, item.childLeadEnd ?? item.end)
          .join('')
          .trim()
      : item.text

    return {
      children: item.childItems?.map((child) => child.text),
      kind: 'item',
      marker: sourceMarker(item),
      text: mainText,
    }
  })
}

watch(
  () => props.job.id,
  () => {
    dimensionComparisonExpanded.value = false
    bodyComparisonExpanded.value = false
  },
)

function sourceMarker(item: ListItem): string {
  return item.kind === 'ordered-item' ? item.marker.replace(/[、.．）)]/gu, '') : '•'
}

function itemAriaLabel(item: ListItem): string {
  return item.kind === 'ordered-item' ? `第 ${sourceMarker(item)} 条` : '列表条目'
}

function scrollToSection(id: string): void {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<template>
  <div class="detail-reading-layout">
    <header class="detail-reading-layout__brief">
      <div>
        <p class="detail-reading-layout__kicker">岗位详情</p>
        <h1>{{ job.title }}</h1>
        <p class="detail-reading-layout__location">
          {{ job.locations.join(' · ') || '地点未说明' }}
        </p>
      </div>
      <div class="detail-reading-layout__states">
        <span class="detail-reading-layout__state">
          {{ historicalContext ? '节点当日：' : '' }}{{ formatActivityState(job.activity_state) }}
        </span>
        <span
          v-if="historicalContext"
          :class="[
            'detail-reading-layout__current-status',
            `detail-reading-layout__current-status--${historicalContext.currentStatus}`,
          ]"
        >
          当前：{{ formatCurrentComparisonStatus(historicalContext.currentStatus) }}
        </span>
        <span v-if="job.is_urgent" class="detail-reading-layout__urgent">紧急招聘</span>
      </div>
    </header>

    <dl class="detail-reading-layout__facts">
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

    <section v-if="comparison" class="detail-comparison" aria-label="JD 变动对比">
      <p
        v-if="comparison.status === 'unavailable'"
        class="detail-comparison__unavailable"
        role="status"
      >
        {{ comparison.message ?? '变动对比暂不可用' }}
        <button
          v-if="comparison.retryable"
          class="detail-comparison__retry"
          type="button"
          @click="emit('retry-comparison')"
        >
          重试
        </button>
      </p>
      <template v-else-if="dimensionChanges.length || bodyComparisons.length">
        <header class="detail-comparison__header">
          <div class="detail-comparison__heading">
            <span class="detail-comparison__eyebrow">变更摘要</span>
            <strong>{{ dimensionChanges.length + bodyComparisons.length }} 项变动</strong>
          </div>
        </header>
        <div class="detail-comparison__tags">
          <button
            v-if="dimensionChanges.length"
            class="detail-comparison__tag detail-comparison__tag--dimensions"
            type="button"
            :aria-expanded="dimensionComparisonExpanded"
            aria-controls="detail-comparison-dimensions"
            @click="dimensionComparisonExpanded = !dimensionComparisonExpanded"
          >
            <span class="detail-comparison__tag-marker" aria-hidden="true" />
            <span class="detail-comparison__tag-copy">
              <span>维度变动</span>
            </span>
            <strong>{{ dimensionChanges.length }}</strong>
            <span class="detail-comparison__tag-chevron" aria-hidden="true" />
          </button>
          <button
            v-if="bodyComparisons.length"
            class="detail-comparison__tag detail-comparison__tag--body"
            type="button"
            :aria-expanded="bodyComparisonExpanded"
            aria-controls="detail-comparison-body"
            @click="bodyComparisonExpanded = !bodyComparisonExpanded"
          >
            <span class="detail-comparison__tag-marker" aria-hidden="true" />
            <span class="detail-comparison__tag-copy">
              <span>正文变动</span>
            </span>
            <strong>{{ bodyComparisons.length }}</strong>
            <span class="detail-comparison__tag-chevron" aria-hidden="true" />
          </button>
        </div>
        <ul
          v-if="dimensionComparisonExpanded"
          id="detail-comparison-dimensions"
          class="detail-comparison__dimension-list"
        >
          <li v-for="change in dimensionChanges" :key="change.field">
            <strong>{{ change.field }}</strong>
            <span>{{ change.historical }} → {{ change.latest }}</span>
          </li>
        </ul>
        <div
          v-if="bodyComparisonExpanded"
          id="detail-comparison-body"
          class="detail-comparison__body-summary"
        >
          <span class="detail-comparison__body-label">涉及正文</span>
          <span
            v-for="section in bodyComparisons"
            :key="section.section"
            class="detail-comparison__body-section"
            >{{ section.label }}</span
          >
          <span class="detail-comparison__legend" aria-label="正文变动图例">
            <span class="detail-comparison__legend-item detail-comparison__legend-item--deleted"
              >已删除</span
            >
            <span class="detail-comparison__legend-item detail-comparison__legend-item--inserted"
              >最新新增</span
            >
          </span>
        </div>
      </template>
    </section>

    <div
      v-if="job.technology_keywords.length || job.other_job_families.length || job.projects.length"
      class="detail-reading-layout__metadata"
    >
      <section v-if="job.technology_keywords.length">
        <h2>技术关键词</h2>
        <div class="detail-reading-layout__tag-list">
          <em v-for="technology in job.technology_keywords" :key="technology.value">{{
            technology.label
          }}</em>
        </div>
      </section>
      <section v-if="job.other_job_families.length">
        <h2>副岗位族</h2>
        <div class="detail-reading-layout__tag-list detail-reading-layout__tag-list--family">
          <em v-for="family in job.other_job_families" :key="family.value">{{ family.label }}</em>
        </div>
      </section>
      <section v-if="job.projects.length">
        <h2>项目</h2>
        <div class="detail-reading-layout__tag-list detail-reading-layout__tag-list--project">
          <em v-for="project in job.projects" :key="project.value">{{ project.label }}</em>
        </div>
      </section>
    </div>

    <div class="detail-reading-layout__reading-frame">
      <aside class="detail-reading-layout__margin-index" aria-label="正文目录">
        <p>正文目录</p>
        <a
          v-for="section in sections"
          :key="section.key"
          :href="`#${section.id}`"
          @click.prevent="scrollToSection(section.id)"
          >{{ section.label }}</a
        >
      </aside>

      <div class="detail-reading-layout__sections">
        <section
          v-for="section in sections"
          :id="section.id"
          :key="section.key"
          class="margin-dossier__section"
        >
          <header class="margin-dossier__heading">
            <h2>{{ section.label }}</h2>
            <span v-if="bodyComparison(section.key)" class="margin-dossier__change-marker"
              >已变动</span
            >
          </header>
          <div
            v-if="bodyComparisonExpanded && bodyComparison(section.key)"
            class="detail-comparison__diff"
            :aria-label="`${section.label}变动内容`"
          >
            <template
              v-for="(fragment, index) in bodyComparison(section.key)?.fragments"
              :key="`${index}-${fragment.kind}`"
            >
              <div
                v-if="comparisonFragmentLines(fragment).length"
                class="detail-comparison__fragment-list"
              >
                <div
                  v-for="(line, lineIndex) in comparisonFragmentLines(fragment)"
                  :key="`${index}-${lineIndex}-${line.kind}`"
                  :class="[
                    line.kind === 'item'
                      ? 'margin-dossier__row'
                      : line.kind === 'lead'
                        ? 'margin-dossier__lead'
                        : 'margin-dossier__group',
                    'detail-comparison__fragment-line',
                    `detail-comparison__fragment-line--${line.kind}`,
                  ]"
                >
                  <span
                    v-if="line.kind === 'item'"
                    class="margin-dossier__marker detail-comparison__fragment-marker"
                  >
                    {{ line.marker }}
                  </span>
                  <div class="detail-comparison__fragment-copy">
                    <span
                      :class="['detail-comparison__fragment', comparisonFragmentClass(fragment)]"
                      :aria-label="comparisonFragmentLabel(fragment)"
                      >{{ line.text }}</span
                    >
                    <ul
                      v-if="line.kind === 'item' && line.children?.length"
                      class="margin-dossier__children detail-comparison__fragment-children"
                    >
                      <li
                        v-for="(child, childIndex) in line.children"
                        :key="`${index}-${lineIndex}-${childIndex}`"
                      >
                        <span
                          :class="[
                            'detail-comparison__fragment',
                            comparisonFragmentClass(fragment),
                          ]"
                          :aria-label="comparisonFragmentLabel(fragment)"
                          >{{ child }}</span
                        >
                      </li>
                    </ul>
                  </div>
                </div>
              </div>
              <span
                v-else
                :class="['detail-comparison__fragment', comparisonFragmentClass(fragment)]"
                :aria-label="comparisonFragmentLabel(fragment)"
                >{{ fragment.text }}</span
              >
            </template>
          </div>
          <div v-else-if="section.presentation.kind === 'list'" class="margin-dossier__items">
            <template
              v-for="item in section.presentation.items"
              :key="`${item.kind}-${item.start}`"
            >
              <p v-if="item.kind === 'lead'" class="margin-dossier__lead">
                <HighlightedText
                  :end="item.end"
                  :ranges="job.body.technology_highlights"
                  :section="section.key"
                  :source-text="section.text"
                  :start="item.start"
                />
              </p>
              <h3 v-else-if="item.kind === 'group-heading'" class="margin-dossier__group">
                <HighlightedText
                  :end="item.end"
                  :ranges="job.body.technology_highlights"
                  :section="section.key"
                  :source-text="section.text"
                  :start="item.start"
                />
              </h3>
              <div v-else class="margin-dossier__row">
                <span class="margin-dossier__marker" :aria-label="itemAriaLabel(item)">{{
                  sourceMarker(item)
                }}</span>
                <div>
                  <HighlightedText
                    v-if="!item.childItems || (item.childLeadEnd ?? item.start) > item.start"
                    :end="item.childLeadEnd ?? item.end"
                    :ranges="job.body.technology_highlights"
                    :section="section.key"
                    :source-text="section.text"
                    :start="item.start"
                  />
                  <ul v-if="item.childItems" class="margin-dossier__children">
                    <li v-for="child in item.childItems" :key="`${child.start}-${child.end}`">
                      <HighlightedText
                        :end="child.end"
                        :ranges="job.body.technology_highlights"
                        :section="section.key"
                        :source-text="section.text"
                        :start="child.start"
                      />
                    </li>
                  </ul>
                </div>
              </div>
            </template>
          </div>
          <p v-else class="margin-dossier__fallback">
            <HighlightedText
              :end="section.presentation.end"
              :ranges="job.body.technology_highlights"
              :section="section.key"
              :source-text="section.text"
              :start="section.presentation.start"
            />
          </p>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.detail-reading-layout {
  --detail-ink: #121c22;
  --detail-muted: #36525f;
  --detail-rule: #bfd8e1;
  --detail-paper: rgba(255, 255, 255, 0.94);
  --detail-tide: #007bab;
  --detail-lagoon: #008287;
  --detail-tag: #eaf5f8;
  --detail-tag-ink: #315f73;
  color: var(--detail-ink);
  margin: 0 auto;
  max-width: 1040px;
  padding: 30px 32px 110px;
}

.detail-reading-layout__brief {
  align-items: flex-start;
  background: var(--detail-paper);
  border: 1px solid rgba(207, 228, 235, 0.9);
  border-radius: 20px 20px 0 0;
  display: flex;
  gap: 24px;
  justify-content: space-between;
  padding: 30px 38px 26px;
}

.detail-reading-layout__kicker {
  color: var(--detail-tide);
  font-size: 0.72rem;
  font-weight: 760;
  letter-spacing: 0.13em;
  margin: 0;
}

.detail-reading-layout h1 {
  color: var(--detail-ink);
  font-size: clamp(2rem, 4.2vw, 3rem);
  letter-spacing: -0.045em;
  line-height: 1.12;
  margin: 8px 0 10px;
  max-width: 22ch;
}

.detail-reading-layout__location {
  color: var(--detail-muted);
  margin: 0;
}

.detail-reading-layout__states {
  align-items: flex-end;
  display: flex;
  flex: 0 0 auto;
  flex-direction: column;
  gap: 7px;
}

.detail-reading-layout__state,
.detail-reading-layout__current-status,
.detail-reading-layout__urgent {
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 700;
  line-height: 1;
  padding: 6px 9px;
  white-space: nowrap;
}

.detail-reading-layout__state {
  background: #e5f5ee;
  color: #00734a;
}

.detail-reading-layout__current-status {
  background: #edf3f5;
  color: #3c5b67;
}

.detail-reading-layout__current-status--changed {
  background: #fff0c8;
  color: #8a5a00;
}

.detail-reading-layout__current-status--confirmed_closed {
  background: #f6dfe1;
  color: #993d4c;
}

.detail-reading-layout__urgent {
  background: #fff2d3;
  color: #986400;
}

.detail-reading-layout__facts {
  background: var(--detail-paper);
  border: 1px solid rgba(207, 228, 235, 0.9);
  border-radius: 0 0 20px 20px;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin: 0;
  padding: 16px 38px;
}

.detail-reading-layout__facts div + div {
  border-left: 1px solid var(--detail-rule);
  padding-left: 16px;
}

.detail-reading-layout__facts dt,
.detail-reading-layout__metadata h2 {
  color: var(--detail-muted);
  font-size: 0.72rem;
  font-weight: 720;
  margin: 0 0 5px;
}

.detail-reading-layout__facts dd {
  color: var(--detail-ink);
  font-size: 0.92rem;
  font-weight: 700;
  margin: 0;
}

.detail-reading-layout__facts div:first-child dd {
  color: var(--detail-tide);
}

.detail-reading-layout__metadata {
  display: flex;
  flex-wrap: wrap;
  gap: 14px 32px;
  padding: 20px 38px 8px;
}

.detail-reading-layout__metadata section {
  min-width: 0;
}

.detail-reading-layout__tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.detail-reading-layout__tag-list em {
  background: var(--detail-tag);
  border-radius: 7px;
  color: var(--detail-tag-ink);
  font-size: 0.74rem;
  font-style: normal;
  font-weight: 680;
  padding: 4px 7px;
}

.detail-reading-layout__tag-list--family em {
  background: #e6f3f4;
  color: #007077;
}

.detail-reading-layout__tag-list--project em {
  background: #f1effb;
  color: #625a9a;
}

.detail-reading-layout__reading-frame {
  display: grid;
  gap: 38px;
  grid-template-columns: 148px minmax(0, 1fr);
  margin-top: 22px;
}

.detail-reading-layout__sections {
  min-width: 0;
}

.detail-reading-layout__margin-index {
  align-self: start;
  border-top: 2px solid var(--detail-tide);
  display: grid;
  gap: 5px;
  padding-top: 10px;
  position: sticky;
  top: 26px;
}

.detail-reading-layout__margin-index p {
  color: var(--detail-muted);
  font-size: 0.69rem;
  font-weight: 760;
  letter-spacing: 0.09em;
  margin: 0 0 3px;
}

.detail-reading-layout__margin-index a {
  color: var(--detail-ink);
  font-size: 0.82rem;
  padding: 4px 0;
  text-decoration: none;
}

.detail-reading-layout__margin-index a:hover {
  color: var(--detail-tide);
}

.margin-dossier__section {
  border-top: 1px solid var(--detail-rule);
  padding-top: 26px;
}

.margin-dossier__section + .margin-dossier__section {
  margin-top: 36px;
}

.margin-dossier__heading {
  align-items: center;
  border-left: 3px solid var(--detail-tide);
  display: flex;
  margin-bottom: 12px;
  min-height: 24px;
  padding-left: 10px;
}

.margin-dossier__heading h2 {
  color: var(--detail-ink);
  font-size: 1.05rem;
  letter-spacing: -0.015em;
  margin: 0;
}

.margin-dossier__items {
  display: grid;
}

.margin-dossier__row {
  border-top: 1px dashed var(--detail-rule);
  color: var(--detail-muted);
  display: grid;
  font-size: 0.96rem;
  gap: 22px;
  grid-template-columns: 48px minmax(0, 1fr);
  line-height: 1.86;
  padding: 17px 0;
}

.margin-dossier__marker {
  color: var(--detail-tide);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.84rem;
  font-weight: 820;
  padding-top: 0.14em;
}

.margin-dossier__lead,
.margin-dossier__fallback {
  color: var(--detail-muted);
  font-size: 0.96rem;
  line-height: 1.86;
  margin: 0;
  white-space: pre-wrap;
}

.margin-dossier__lead {
  margin-bottom: 10px;
}

.margin-dossier__group {
  color: var(--detail-lagoon);
  font-size: 0.84rem;
  letter-spacing: 0.035em;
  margin: 14px 0 4px;
}

.margin-dossier__children {
  display: grid;
  gap: 6px;
  list-style: none;
  margin: 10px 0 0;
  padding: 0;
}

.margin-dossier__children li {
  display: grid;
  gap: 9px;
  grid-template-columns: 6px minmax(0, 1fr);
}

.margin-dossier__children li::before {
  background: var(--detail-lagoon);
  border-radius: 999px;
  content: '';
  height: 5px;
  margin-top: 0.84em;
  width: 5px;
}

.detail-reading-layout :deep(.technology-highlight) {
  background: #b7efdf;
  border-bottom: 2px solid #008287;
  border-radius: 3px;
  color: #064b4d;
  font-weight: 750;
  padding: 0 0.12em;
}

.detail-comparison {
  border-left: 2px solid var(--detail-rule);
  box-sizing: border-box;
  margin-top: 12px;
  max-width: 100%;
  padding: 0 0 0 10px;
  width: min(100%, 360px);
}

.detail-comparison__header {
  align-items: baseline;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.detail-comparison__heading {
  align-items: baseline;
  display: flex;
  gap: 9px;
  min-width: 0;
}

.detail-comparison__eyebrow {
  color: var(--detail-tide);
  font-size: 0.7rem;
  font-weight: 760;
  letter-spacing: 0.08em;
  white-space: nowrap;
}

.detail-comparison__heading strong {
  color: var(--detail-ink);
  font-size: 0.82rem;
}

.detail-comparison__tags {
  display: grid;
  gap: 0;
  grid-template-columns: 1fr;
  margin-top: 4px;
}

.detail-comparison__tag {
  align-items: center;
  background: transparent;
  border: 0;
  border-top: 1px solid var(--detail-rule);
  border-radius: 0;
  color: var(--detail-ink);
  cursor: pointer;
  display: grid;
  font: inherit;
  gap: 0 8px;
  grid-template-columns: 4px minmax(0, 1fr) auto 9px;
  min-height: 31px;
  padding: 5px 4px;
  text-align: left;
  transition:
    background-color 150ms ease,
    border-color 150ms ease,
    box-shadow 150ms ease;
}

.detail-comparison__tag:hover {
  background: color-mix(in srgb, var(--detail-tag) 55%, transparent);
}

.detail-comparison__tag:focus-visible,
.detail-comparison__retry:focus-visible {
  outline: 2px solid var(--detail-tide);
  outline-offset: 2px;
}

.detail-comparison__tag-marker {
  align-self: stretch;
  background: var(--detail-tide);
  border-radius: 3px;
  min-height: 18px;
}

.detail-comparison__tag--body .detail-comparison__tag-marker {
  background: #b04b55;
}

.detail-comparison__tag-copy {
  align-items: center;
  display: flex;
  min-width: 0;
}

.detail-comparison__tag-copy > span {
  font-size: 0.75rem;
  font-weight: 780;
  line-height: 1.2;
}

.detail-comparison__tag strong {
  align-self: center;
  color: var(--detail-tide);
  font-size: 0.82rem;
  min-width: 1.2em;
  text-align: right;
}

.detail-comparison__tag--body strong {
  color: #a4434d;
}

.detail-comparison__tag-chevron {
  border-bottom: 1.5px solid var(--detail-muted);
  border-right: 1.5px solid var(--detail-muted);
  height: 6px;
  justify-self: end;
  transform: rotate(45deg) translateY(-2px);
  transition: transform 150ms ease;
  width: 6px;
}

.detail-comparison__tag[aria-expanded='true'] {
  background: var(--detail-tag);
}

.detail-comparison__tag[aria-expanded='true'] .detail-comparison__tag-chevron {
  transform: rotate(225deg) translate(-1px, -1px);
}

.detail-comparison__tag--body[aria-expanded='true'] {
  background: color-mix(in srgb, #b04b55 8%, transparent);
}

.detail-comparison__dimension-list {
  background: transparent;
  border: 0;
  border-top: 1px dashed var(--detail-rule);
  border-radius: 0;
  display: grid;
  gap: 8px;
  list-style: none;
  margin: 0 0 0 12px;
  padding: 6px 8px;
}

.detail-comparison__dimension-list li {
  align-items: baseline;
  display: grid;
  gap: 8px;
  grid-template-columns: minmax(70px, auto) minmax(0, 1fr);
  line-height: 1.35;
}

.detail-comparison__dimension-list li + li {
  border-top: 1px dashed var(--detail-rule);
  padding-top: 6px;
}

.detail-comparison__dimension-list li strong {
  color: var(--detail-ink);
  font-size: 0.74rem;
}

.detail-comparison__dimension-list li span {
  color: var(--detail-muted);
  font-size: 0.76rem;
  overflow-wrap: anywhere;
}

.detail-comparison__body-summary {
  align-items: center;
  background: transparent;
  border: 0;
  border-top: 1px dashed var(--detail-rule);
  border-radius: 0;
  color: var(--detail-muted);
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0 0 0 12px;
  padding: 6px 8px;
}

.detail-comparison__body-label {
  font-size: 0.7rem;
  font-weight: 760;
  margin-right: 2px;
}

.detail-comparison__body-section {
  background: var(--detail-tag);
  border-radius: 5px;
  color: var(--detail-tag-ink);
  font-size: 0.7rem;
  font-weight: 700;
  padding: 3px 6px;
}

.detail-comparison__legend {
  display: inline-flex;
  gap: 6px;
  margin-left: auto;
}

.detail-comparison__legend-item {
  border-radius: 3px;
  font-size: 0.68rem;
  padding: 3px 7px;
}

.detail-comparison__legend-item--deleted,
.detail-comparison__fragment--deleted {
  background: var(--detail-diff-deleted-bg, #f0a5ad);
  color: var(--detail-diff-deleted-ink, #721c24);
  text-decoration: line-through;
  text-decoration-thickness: 1px;
}

.detail-comparison__legend-item--inserted,
.detail-comparison__fragment--inserted {
  background: var(--detail-diff-inserted-bg, #9edcaf);
  color: var(--detail-diff-inserted-ink, #14532d);
}

.detail-comparison__unavailable {
  align-items: center;
  color: var(--detail-muted);
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin: 0;
}

.detail-comparison__retry {
  background: transparent;
  border: 1px solid var(--detail-rule);
  border-radius: 6px;
  color: var(--detail-tide);
  cursor: pointer;
  font: inherit;
  padding: 3px 8px;
}

.detail-comparison__diff {
  color: inherit;
  line-height: 1.6;
  white-space: pre-wrap;
}

.detail-comparison__fragment-list {
  display: grid;
  gap: 0;
  white-space: normal;
}

.detail-comparison__fragment-line {
  min-width: 0;
}

.detail-comparison__fragment-copy {
  min-width: 0;
}

.detail-comparison__fragment-line--lead .detail-comparison__fragment-copy,
.detail-comparison__fragment-line--group-heading .detail-comparison__fragment-copy {
  display: block;
}

.detail-comparison__fragment-marker {
  padding-top: 0.14em;
}

.detail-comparison__fragment {
  border-radius: 2px;
  box-decoration-break: clone;
  padding: 0 0.1em;
}

.margin-dossier__change-marker {
  background: var(--detail-tag);
  border-radius: 4px;
  color: var(--detail-tag-ink);
  font-size: 0.7rem;
  font-weight: 700;
  padding: 3px 6px;
}

@media (max-width: 700px) {
  .detail-reading-layout {
    padding: 26px 16px 56px;
  }

  .detail-reading-layout__brief {
    border-radius: 18px 18px 0 0;
    flex-direction: column;
    gap: 13px;
    padding: 25px 22px 22px;
  }

  .detail-reading-layout__states {
    align-items: flex-start;
    flex-direction: row;
    order: -1;
  }

  .detail-reading-layout h1 {
    font-size: clamp(1.75rem, 7.2vw, 2rem);
    line-height: 1.14;
  }

  .detail-reading-layout__facts {
    border-radius: 0 0 18px 18px;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    padding: 16px 22px;
    row-gap: 16px;
  }

  .detail-reading-layout__facts div:nth-child(3) {
    border-left: 0;
    padding-left: 0;
  }

  .detail-reading-layout__metadata {
    gap: 14px;
    padding: 18px 22px 4px;
  }

  .detail-comparison {
    padding: 0 0 0 8px;
  }

  .detail-comparison__legend {
    flex-basis: 100%;
    margin-left: 0;
  }

  .detail-reading-layout__reading-frame {
    display: block;
    margin-top: 20px;
  }

  .detail-reading-layout__margin-index {
    border-bottom: 1px solid var(--detail-rule);
    border-top: 0;
    display: flex;
    gap: 12px;
    margin-bottom: 23px;
    overflow-x: auto;
    padding: 0 0 10px;
    position: static;
  }

  .detail-reading-layout__margin-index p {
    display: none;
  }

  .detail-reading-layout__margin-index a {
    flex: 0 0 auto;
  }

  .margin-dossier__section {
    padding-top: 21px;
  }

  .margin-dossier__section + .margin-dossier__section {
    margin-top: 27px;
  }

  .margin-dossier__row {
    gap: 12px;
    grid-template-columns: 31px minmax(0, 1fr);
  }
}

:global(.app-shell[data-color-scheme='dark'] .detail-reading-layout) {
  --detail-ink: #ecf6f7;
  --detail-muted: #afc9d0;
  --detail-rule: #31505c;
  --detail-paper: #17303a;
  --detail-tide: #62c6df;
  --detail-lagoon: #6ad0c0;
  --detail-tag: #214550;
  --detail-tag-ink: #c9e9ef;
  --detail-diff-deleted-bg: #71343d;
  --detail-diff-deleted-ink: #ffd4d8;
  --detail-diff-inserted-bg: #245a3d;
  --detail-diff-inserted-ink: #d4f8df;
}

:global(.app-shell[data-color-scheme='dark'] .detail-reading-layout__brief),
:global(.app-shell[data-color-scheme='dark'] .detail-reading-layout__facts) {
  border-color: #31505c;
}

:global(.app-shell[data-color-scheme='dark'] .detail-reading-layout__tag-list--family em) {
  background: #1b4b4c;
  color: #a9e5d7;
}

:global(.app-shell[data-color-scheme='dark'] .detail-reading-layout__tag-list--project em) {
  background: #363455;
  color: #d7d3ff;
}

:global(.app-shell[data-color-scheme='dark'] .detail-reading-layout__state) {
  background: #1e4d46;
  color: #abebd6;
}

:global(.app-shell[data-color-scheme='dark'] .detail-reading-layout__current-status) {
  background: #294752;
  color: #c8dbe0;
}

:global(.app-shell[data-color-scheme='dark'] .detail-reading-layout__current-status--changed) {
  background: #5a4724;
  color: #f5d991;
}

:global(
  .app-shell[data-color-scheme='dark'] .detail-reading-layout__current-status--confirmed_closed
) {
  background: #5a3039;
  color: #ffd0d7;
}

:global(.app-shell[data-color-scheme='dark'] .detail-reading-layout__urgent) {
  background: #563d2c;
  color: #ffd2a1;
}

:global(.app-shell[data-color-scheme='dark'] .detail-reading-layout :deep(.technology-highlight)) {
  background: #176a65;
  border-bottom-color: #73e5d6;
  color: #e2fffa;
}
</style>
