<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { getJobDetail, type JobDetailResponse } from '@/api/jd-api'
import JobDetailSkeleton from '@/components/JobDetailSkeleton.vue'
import JDSectionList from '@/components/JDSectionList.vue'
import JDDetailReadabilityPrototype from '@/components/prototypes/JDDetailReadabilityPrototype.vue'
import {
  formatActivityState,
  formatEducation,
  formatExperience,
  formatFirstDiscoveredDate,
} from '@/lib/job-display'

const route = useRoute()
const router = useRouter()
const response = ref<JobDetailResponse>()
const error = ref('')
const isLoading = ref(false)
let latestDetailRequest = 0
const isReadabilityPrototype = computed(
  () => import.meta.env.DEV && route.query.prototype === 'jd-detail',
)
const technologyKeywordsExpanded = ref(false)
const hasCollapsibleTechnologyKeywords = computed(
  () => (response.value?.technology_keywords.length ?? 0) > 8,
)

watch(
  () => String(route.params.id ?? ''),
  async (id) => {
    const request = ++latestDetailRequest
    isLoading.value = true
    error.value = ''
    technologyKeywordsExpanded.value = false
    try {
      const nextResponse = await getJobDetail(id)
      if (request !== latestDetailRequest) {
        return
      }
      response.value = nextResponse
    } catch (reason) {
      if (request !== latestDetailRequest) {
        return
      }
      error.value = reason instanceof Error ? reason.message : '请求岗位详情失败'
    } finally {
      if (request === latestDetailRequest) {
        isLoading.value = false
      }
    }
  },
  { immediate: true },
)

function returnToList(): void {
  router.back()
}
</script>

<template>
  <main class="job-detail-page">
    <div class="tide-mark" aria-hidden="true"><span></span><span></span><span></span></div>
    <button class="back-button" type="button" @click="returnToList">← 返回岗位列表</button>
    <JobDetailSkeleton v-if="isLoading && !response" />
    <p v-else-if="error && !response" class="error-message" role="alert">{{ error }}</p>
    <JDDetailReadabilityPrototype v-else-if="response && isReadabilityPrototype" :job="response" />
    <article v-else-if="response">
      <p v-if="error" class="error-message job-detail-refresh-error" role="alert">{{ error }}</p>
      <header class="job-detail-header">
        <div>
          <p class="eyebrow">JD DETAIL</p>
          <h1>{{ response.title }}</h1>
          <p class="job-detail-location">{{ response.locations.join(' · ') || '地点未说明' }}</p>
        </div>
        <div class="job-detail-state">
          <span :class="['status-pill', `status-pill--${response.activity_state}`]">{{
            formatActivityState(response.activity_state)
          }}</span>
          <span v-if="response.is_urgent" class="urgent-pill">紧急招聘</span>
        </div>
      </header>

      <section class="formal-summary" aria-label="岗位建模信息">
        <div>
          <span>主岗位族</span>
          <strong>{{ response.main_job_family?.label ?? '未分类' }}</strong>
        </div>
        <div>
          <span>学历要求</span>
          <strong>{{ formatEducation(response.education) }}</strong>
        </div>
        <div>
          <span>最低工作年限</span>
          <strong>{{ formatExperience(response.experience) }}</strong>
        </div>
        <div>
          <span>首次发现</span>
          <strong>{{ formatFirstDiscoveredDate(response.first_discovered_date) }}</strong>
        </div>
      </section>

      <section v-if="response.other_job_families.length" class="model-section">
        <h2>其他命中岗位族</h2>
        <div class="tag-list">
          <em
            v-for="family in response.other_job_families"
            :key="family.value"
            class="tag tag--family"
            >{{ family.label }}</em
          >
        </div>
      </section>
      <section v-if="response.technology_keywords.length" class="model-section">
        <h2>技术关键词</h2>
        <div
          :id="`job-technology-keywords-${response.id}`"
          class="tag-list"
          :class="{
            'tag-list--collapsed': hasCollapsibleTechnologyKeywords && !technologyKeywordsExpanded,
          }"
        >
          <em
            v-for="technology in response.technology_keywords"
            :key="technology.value"
            class="tag"
            >{{ technology.label }}</em
          >
        </div>
        <button
          v-if="hasCollapsibleTechnologyKeywords"
          class="tag-list__toggle"
          type="button"
          :aria-controls="`job-technology-keywords-${response.id}`"
          :aria-expanded="technologyKeywordsExpanded"
          @click="technologyKeywordsExpanded = !technologyKeywordsExpanded"
        >
          {{
            technologyKeywordsExpanded
              ? '收起技术词'
              : `展开全部技术词（${response.technology_keywords.length}）`
          }}
        </button>
      </section>
      <section v-if="response.projects.length" class="model-section">
        <h2>项目</h2>
        <div class="tag-list">
          <em v-for="project in response.projects" :key="project.value" class="tag tag--project">{{
            project.label
          }}</em>
        </div>
      </section>

      <section class="body-section">
        <h2>岗位职责</h2>
        <JDSectionList
          :ranges="response.body.technology_highlights"
          section="responsibilities"
          :text="response.body.responsibilities"
        />
      </section>
      <section class="body-section">
        <h2>任职要求</h2>
        <JDSectionList
          :ranges="response.body.technology_highlights"
          section="requirements"
          :text="response.body.requirements"
        />
      </section>
      <section v-if="response.body.supplemental" class="body-section">
        <h2>补充说明</h2>
        <JDSectionList
          :ranges="response.body.technology_highlights"
          section="supplemental"
          :text="response.body.supplemental"
        />
      </section>
    </article>
  </main>
</template>

<style scoped>
.job-detail-page {
  margin: 0 auto;
  max-width: 900px;
  padding: 48px 24px 72px;
}

.back-button {
  background: transparent;
  border: 0;
  color: #007bab;
  cursor: pointer;
  font: inherit;
  font-weight: 600;
  margin-bottom: 36px;
  padding: 8px 0;
}

.eyebrow {
  color: #007bab;
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

h1 {
  color: #121c22;
  font-size: clamp(2rem, 6vw, 3.25rem);
  line-height: 1.15;
  margin: 8px 0 42px;
}

section + section {
  border-top: 1px solid #e3ebf0;
  margin-top: 32px;
  padding-top: 32px;
}

h2 {
  color: #2e373c;
  font-size: 1rem;
  margin-bottom: 12px;
}

.body-copy {
  color: #2e373c;
  line-height: 1.8;
  white-space: pre-wrap;
}

.error-message {
  color: #b44953;
}

.job-detail-page {
  max-width: 1040px;
  padding: 52px 32px 88px;
}

.tide-mark {
  align-items: center;
  display: flex;
  gap: 6px;
  height: 22px;
  margin-bottom: 16px;
}
.tide-mark span {
  background: #007bab;
  border-radius: 99px;
  display: block;
  height: 3px;
}
.tide-mark span:nth-child(1) {
  width: 44px;
}
.tide-mark span:nth-child(2) {
  background: #008287;
  width: 24px;
}
.tide-mark span:nth-child(3) {
  opacity: 0.38;
  width: 12px;
}

article {
  background: #fff;
  border-radius: 16px;
  box-shadow:
    0 5px 12px 4px rgba(11, 37, 50, 0.04),
    0 20px 42px -32px rgba(11, 55, 76, 0.32);
  overflow: hidden;
}
.back-button {
  margin-bottom: 20px;
}
.eyebrow {
  font-size: 0.72rem;
  letter-spacing: 0.14em;
}
h1 {
  font-size: clamp(2rem, 5vw, 3.25rem);
  letter-spacing: -0.05em;
  line-height: 1.08;
  margin: 9px 0 10px;
}

article section + section {
  border: 0;
  margin: 0;
  padding: 0;
}
.job-detail-header {
  align-items: flex-start;
  background: linear-gradient(125deg, #f0f8fd 0%, #fff 70%);
  display: flex;
  gap: 20px;
  justify-content: space-between;
  padding: 34px 38px 30px;
}
.job-detail-location {
  color: #626a6f;
  margin: 0;
}
.job-detail-state {
  align-items: flex-end;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  gap: 8px;
}

.status-pill,
.urgent-pill,
.tag {
  border-radius: 8px;
  font-size: 0.76rem;
  font-style: normal;
  font-weight: 650;
  line-height: 1;
  padding: 6px 8px;
  white-space: nowrap;
}
.status-pill--effective {
  background: #e5f5ee;
  color: #00734a;
}
.status-pill--confirmed_closed {
  background: #f7ebed;
  color: #9d3340;
}
.urgent-pill {
  background: #fff2d3;
  color: #9a6700;
}

.formal-summary {
  border-bottom: 1px solid #e3ebf0;
  border-top: 1px solid #e3ebf0;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  padding: 18px 38px;
}
.formal-summary div + div {
  border-left: 1px solid #e3ebf0;
  padding-left: 18px;
}
.formal-summary span {
  color: #72818a;
  display: block;
  font-size: 0.73rem;
  font-weight: 600;
  margin-bottom: 5px;
}
.formal-summary strong {
  color: #2e373c;
  font-size: 0.91rem;
}
.formal-summary div:first-child strong {
  color: #007bab;
}

.model-section {
  padding: 22px 38px 0;
}
.model-section h2,
.body-section h2 {
  color: #406980;
  font-size: 0.8rem;
  letter-spacing: 0.035em;
  margin: 0 0 11px;
}
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}
.tag {
  background: #eef7fb;
  color: #406980;
  display: inline-block;
}
.tag--family {
  background: #e6f3f4;
  color: #007077;
}
.tag--project {
  background: #f1effb;
  color: #625a9a;
}

.body-section {
  border-top: 1px solid #e3ebf0;
  margin: 30px 38px 0;
  padding-top: 30px;
}
.body-section:last-child {
  padding-bottom: 38px;
}
.body-section h2 {
  color: #121c22;
  font-size: 1rem;
  letter-spacing: 0;
  margin-bottom: 14px;
}
.body-copy {
  margin: 0;
}

@media (max-width: 650px) {
  .job-detail-page {
    padding: 36px 16px 56px;
  }
  .job-detail-header {
    flex-direction: column;
    gap: 12px;
    padding: 28px 22px 24px;
  }
  .job-detail-header > :first-child {
    width: 100%;
  }
  .job-detail-state {
    align-items: flex-start;
    flex-direction: row;
    gap: 6px;
    order: -1;
  }
  .job-detail-header h1 {
    font-size: clamp(1.75rem, 7.2vw, 1.9rem);
    line-height: 1.12;
  }
  .formal-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    padding: 16px 22px;
    row-gap: 16px;
  }
  .formal-summary div:nth-child(3) {
    border-left: 0;
    padding-left: 0;
  }
  .model-section {
    padding-left: 22px;
    padding-right: 22px;
  }
  .tag-list--collapsed {
    max-height: 48px;
    overflow: hidden;
  }
  .tag-list__toggle {
    background: transparent;
    border: 0;
    color: #007bab;
    cursor: pointer;
    display: inline-flex;
    font: inherit;
    font-size: 0.78rem;
    font-weight: 700;
    margin: 10px 0 0;
    padding: 2px 0;
  }
  .body-section {
    margin-left: 22px;
    margin-right: 22px;
  }
}
</style>
