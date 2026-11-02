<script setup lang="ts">
import { NButton, NInput, NPagination, NSelect, type SelectOption } from 'naive-ui'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  getFilterOptions,
  getJobList,
  type FilterOptionsResponse,
  type JobListResponse,
} from '@/api/jd-api'
import JobListSkeleton from '@/components/JobListSkeleton.vue'
import TechnologyKeywordSelect from '@/components/TechnologyKeywordSelect.vue'
import {
  createDefaultListState,
  listStateFromRoute,
  listStateToRoute,
  type ActivityState,
  type JDListState,
  type RequirementSort,
  type SortDirection,
  type SortKind,
} from '@/lib/list-state'
import {
  formatActivityState,
  formatEducation,
  formatExperience,
  formatFirstDiscoveredDate,
} from '@/lib/job-display'

const route = useRoute()
const router = useRouter()
const response = ref<JobListResponse>()
const error = ref('')
const isLoading = ref(false)
let latestListRequest = 0
const listState = computed(() => listStateFromRoute(route.query))
const filterOptions = ref<FilterOptionsResponse>({
  activity_states: [],
  education_levels: [],
  experience_years: [],
  job_families: [],
  locations: [],
  main_job_families: [],
  other_job_families: [],
  projects: [],
  technology_keywords: [],
  urgent_options: [],
})
const filterOptionsError = ref('')
const draft = reactive<JDListState>(createDefaultListState())
const activityMode = ref<ActivityState | 'all'>('effective')
const urgentValue = ref<string | null>(null)
const sortKindToAdd = ref<SortKind | null>(null)
const sortDirectionToAdd = ref<SortDirection | null>(null)
const maximumSortRules = 4
const pageSizes = [20, 50, 100]

const minimumExperienceOptions: SelectOption[] = Array.from({ length: 9 }, (_, index) => ({
  label: `${index + 1} 年及以上`,
  value: index + 1,
}))

const sortKindOptions: SelectOption[] = [
  { value: 'experience', label: '最低工作年限' },
  { value: 'education', label: '学历要求' },
  { value: 'first_discovered', label: '首次发现日期' },
  { value: 'urgent', label: '紧急标记' },
  { value: 'activity_state', label: '活跃状态' },
  { value: 'title', label: '岗位标题' },
]

const sortDirectionOptions: Record<SortKind, SelectOption[]> = {
  experience: [
    { value: 'asc', label: '从低到高' },
    { value: 'desc', label: '从高到低' },
  ],
  education: [
    { value: 'asc', label: '从低到高' },
    { value: 'desc', label: '从高到低' },
  ],
  first_discovered: [
    { value: 'asc', label: '由早到晚' },
    { value: 'desc', label: '由新到旧' },
  ],
  urgent: [
    { value: 'asc', label: '非紧急优先' },
    { value: 'desc', label: '紧急优先' },
  ],
  activity_state: [
    { value: 'asc', label: '仍有效优先' },
    { value: 'desc', label: '已确认下线优先' },
  ],
  title: [
    { value: 'asc', label: '正序' },
    { value: 'desc', label: '倒序' },
  ],
}

function sortRuleParts(rule: RequirementSort): { direction: SortDirection; kind: SortKind } {
  const direction: SortDirection = rule.endsWith('_desc') ? 'desc' : 'asc'
  return {
    direction,
    kind: rule.slice(0, -(direction.length + 1)) as SortKind,
  }
}

function buildSortRule(kind: SortKind, direction: SortDirection): RequirementSort {
  return `${kind}_${direction}` as RequirementSort
}

function sortKindOptionsForRule(index: number): SelectOption[] {
  const currentKind = sortRuleParts(draft.sortRules[index] as RequirementSort).kind
  return sortKindOptions.filter(
    (option) =>
      option.value === currentKind ||
      !draft.sortRules.some((rule) => sortRuleParts(rule).kind === option.value),
  )
}

const availableSortKindOptions = computed(() =>
  sortKindOptions.filter(
    (option) => !draft.sortRules.some((rule) => sortRuleParts(rule).kind === option.value),
  ),
)

function replaceDraft(state: JDListState): void {
  Object.assign(draft, {
    ...state,
    activityStates: [...state.activityStates],
    educationLevels: [...state.educationLevels],
    jobFamilies: [...state.jobFamilies],
    locations: [...state.locations],
    projects: [...state.projects],
    sortRules: [...state.sortRules],
    technologyKeywords: [...state.technologyKeywords],
  })
  activityMode.value =
    state.activityStates.length === 2 ? 'all' : (state.activityStates[0] ?? 'effective')
  urgentValue.value = state.isUrgent === undefined ? null : String(state.isUrgent)
}

function stateFromDraft(): JDListState {
  return {
    ...draft,
    activityStates:
      activityMode.value === 'all' ? ['effective', 'confirmed_closed'] : [activityMode.value],
    isUrgent: urgentValue.value === null ? undefined : urgentValue.value === 'true',
    jobFamilies: [...draft.jobFamilies],
    minimumExperienceYear: draft.minimumExperienceYear ?? undefined,
    sortRules: [...draft.sortRules],
    page: 1,
  }
}

function updateSortRuleKind(index: number, nextKind: SortKind | null): void {
  if (nextKind === null) {
    return
  }
  const current = draft.sortRules[index]
  if (current === undefined || sortRuleParts(current).kind === nextKind) {
    return
  }
  if (
    draft.sortRules.some(
      (rule, ruleIndex) => ruleIndex !== index && sortRuleParts(rule).kind === nextKind,
    )
  ) {
    return
  }
  draft.sortRules.splice(index, 1, buildSortRule(nextKind, sortRuleParts(current).direction))
}

function updateSortRuleDirection(index: number, nextDirection: SortDirection | null): void {
  const current = draft.sortRules[index]
  if (nextDirection === null || current === undefined) {
    return
  }
  draft.sortRules.splice(index, 1, buildSortRule(sortRuleParts(current).kind, nextDirection))
}

function addSortRule(): void {
  if (
    sortKindToAdd.value === null ||
    sortDirectionToAdd.value === null ||
    draft.sortRules.length >= maximumSortRules ||
    draft.sortRules.some((rule) => sortRuleParts(rule).kind === sortKindToAdd.value)
  ) {
    return
  }
  draft.sortRules.push(buildSortRule(sortKindToAdd.value, sortDirectionToAdd.value))
  sortKindToAdd.value = null
  sortDirectionToAdd.value = null
}

function removeSortRule(index: number): void {
  draft.sortRules.splice(index, 1)
}

function moveSortRule(index: number, direction: -1 | 1): void {
  const nextIndex = index + direction
  if (nextIndex < 0 || nextIndex >= draft.sortRules.length) {
    return
  }
  const rule = draft.sortRules[index]
  if (rule === undefined) {
    return
  }
  draft.sortRules.splice(index, 1)
  draft.sortRules.splice(nextIndex, 0, rule)
}

function applyFilters(): void {
  void router.push({ name: 'jobs', query: listStateToRoute(stateFromDraft()) })
}

function resetFilters(): void {
  void router.push({ name: 'jobs', query: listStateToRoute(createDefaultListState()) })
}

function changePage(page: number): void {
  if (
    page < 1 ||
    (response.value !== undefined &&
      page > Math.ceil(response.value.page.total / listState.value.pageSize))
  ) {
    return
  }
  void router.push({
    name: 'jobs',
    query: listStateToRoute({ ...listState.value, page }),
  })
}

function changePageSize(pageSize: number): void {
  if (!pageSizes.includes(pageSize)) {
    return
  }
  void router.push({
    name: 'jobs',
    query: listStateToRoute({ ...listState.value, page: 1, pageSize }),
  })
}

async function loadFilterOptions(): Promise<void> {
  filterOptionsError.value = ''
  try {
    filterOptions.value = await getFilterOptions()
  } catch (reason) {
    filterOptionsError.value = reason instanceof Error ? reason.message : '读取筛选项失败'
  }
}

watch(
  listState,
  async (state) => {
    replaceDraft(state)
    const request = ++latestListRequest
    isLoading.value = true
    error.value = ''
    try {
      const nextResponse = await getJobList(state)
      if (request !== latestListRequest) {
        return
      }
      response.value = nextResponse
    } catch (reason) {
      if (request !== latestListRequest) {
        return
      }
      error.value = reason instanceof Error ? reason.message : '请求岗位数据失败'
    } finally {
      if (request === latestListRequest) {
        isLoading.value = false
      }
    }
  },
  { immediate: true },
)

onMounted(() => {
  void loadFilterOptions()
})
</script>

<template>
  <main class="job-list-page">
    <div class="tide-mark" aria-hidden="true"><span></span><span></span><span></span></div>
    <header class="page-heading">
      <div>
        <p class="eyebrow">JD OBSERVATORY · CURRENT INVENTORY</p>
        <h1>岗位检索</h1>
        <p class="heading-copy">查找符合条件的岗位。</p>
      </div>
    </header>

    <section class="filter-panel" aria-label="岗位筛选">
      <div class="filter-panel__heading">
        <div>
          <p class="section-kicker">检索条件</p>
          <h2>筛选岗位</h2>
        </div>
      </div>
      <div class="title-search">
        <label for="job-title-search">标题</label>
        <NInput
          id="job-title-search"
          v-model:value="draft.title"
          clearable
          placeholder="搜索岗位标题，支持模糊匹配"
          @keyup.enter="applyFilters"
        />
      </div>
      <div class="filter-layout">
        <div class="filter-grid">
          <label class="filter-field">
            <span>岗位族</span>
            <NSelect
              v-model:value="draft.jobFamilies"
              :options="filterOptions.job_families"
              clearable
              filterable
              multiple
              placeholder="搜索或选择岗位族"
            />
          </label>
          <label class="filter-field">
            <span>技术关键词</span>
            <TechnologyKeywordSelect
              v-model="draft.technologyKeywords"
              :options="filterOptions.technology_keywords"
            />
          </label>
          <label class="filter-field">
            <span>学历要求</span>
            <NSelect
              v-model:value="draft.educationLevels"
              :options="filterOptions.education_levels"
              clearable
              filterable
              multiple
              placeholder="选择学历要求"
            />
          </label>
          <label class="filter-field">
            <span>最低工作年限</span>
            <NSelect
              v-model:value="draft.minimumExperienceYear"
              :options="minimumExperienceOptions"
              clearable
              data-testid="minimum-experience-select"
              placeholder="不限"
            />
          </label>
          <label class="filter-field">
            <span>项目</span>
            <NSelect
              v-model:value="draft.projects"
              :options="filterOptions.projects"
              clearable
              filterable
              multiple
              placeholder="搜索或选择项目"
            />
          </label>
          <label class="filter-field">
            <span>工作地点</span>
            <NSelect
              v-model:value="draft.locations"
              :options="filterOptions.locations"
              clearable
              filterable
              multiple
              placeholder="搜索或选择工作地点"
            />
          </label>
          <label class="filter-field">
            <span>活跃状态</span>
            <NSelect
              v-model:value="activityMode"
              :options="filterOptions.activity_states"
              placeholder="选择活跃状态"
            />
          </label>
          <label class="filter-field">
            <span>紧急标记</span>
            <NSelect
              v-model:value="urgentValue"
              :options="filterOptions.urgent_options"
              clearable
              placeholder="是否紧急招聘"
            />
          </label>
          <label class="filter-field">
            <span>首次发现日期（起）</span>
            <input v-model="draft.firstDiscoveredFrom" type="date" />
          </label>
          <label class="filter-field">
            <span>首次发现日期（止）</span>
            <input v-model="draft.firstDiscoveredUntil" type="date" />
          </label>
        </div>

        <section class="sort-panel" aria-label="排序优先级">
          <div class="sort-panel__heading">
            <div>
              <p class="section-kicker">排序方案</p>
              <h3>排序</h3>
            </div>
          </div>
          <ol v-if="draft.sortRules.length" class="sort-rule-list">
            <li
              v-for="(rule, index) in draft.sortRules"
              :key="rule"
              :data-testid="`sort-rule-${index + 1}`"
              class="sort-rule"
            >
              <span class="sort-rule__priority">{{ index + 1 }}</span>
              <label>
                <span>排序种类</span>
                <div :data-testid="`sort-kind-${index + 1}`">
                  <NSelect
                    :value="sortRuleParts(rule).kind"
                    :options="sortKindOptionsForRule(index)"
                    :aria-label="`第 ${index + 1} 项排序种类`"
                    @update:value="updateSortRuleKind(index, $event as SortKind)"
                  />
                </div>
              </label>
              <label>
                <span>排序方式</span>
                <div :data-testid="`sort-direction-${index + 1}`">
                  <NSelect
                    :value="sortRuleParts(rule).direction"
                    :options="sortDirectionOptions[sortRuleParts(rule).kind]"
                    :aria-label="`第 ${index + 1} 项排序方式`"
                    @update:value="updateSortRuleDirection(index, $event as SortDirection)"
                  />
                </div>
              </label>
              <div class="sort-rule__actions">
                <NButton
                  size="small"
                  tertiary
                  :data-testid="`move-sort-rule-up-${index + 1}`"
                  :disabled="index === 0"
                  @click="moveSortRule(index, -1)"
                  >上移</NButton
                >
                <NButton
                  size="small"
                  tertiary
                  :data-testid="`move-sort-rule-down-${index + 1}`"
                  :disabled="index === draft.sortRules.length - 1"
                  @click="moveSortRule(index, 1)"
                  >下移</NButton
                >
                <NButton size="small" tertiary @click="removeSortRule(index)">移除</NButton>
              </div>
            </li>
          </ol>
          <div
            v-if="draft.sortRules.length < maximumSortRules"
            class="sort-rule-add"
            data-testid="sort-rule-add"
          >
            <label>
              <span>排序种类</span>
              <NSelect
                v-model:value="sortKindToAdd"
                :options="availableSortKindOptions"
                clearable
                placeholder="选择种类"
              />
            </label>
            <label>
              <span>排序方式</span>
              <NSelect
                v-model:value="sortDirectionToAdd"
                :disabled="sortKindToAdd === null"
                :options="sortKindToAdd === null ? [] : sortDirectionOptions[sortKindToAdd]"
                clearable
                placeholder="选择方式"
              />
            </label>
            <NButton
              secondary
              :disabled="sortKindToAdd === null || sortDirectionToAdd === null"
              @click="addSortRule"
              >添加排序</NButton
            >
          </div>
        </section>
      </div>
      <div class="filter-actions">
        <NButton type="primary" @click="applyFilters">应用筛选</NButton>
        <NButton tertiary @click="resetFilters">重置</NButton>
        <p v-if="filterOptionsError" class="error-message" role="alert">{{ filterOptionsError }}</p>
      </div>
    </section>

    <section aria-live="polite" class="list-result">
      <JobListSkeleton v-if="isLoading && !response" />
      <p v-else-if="error && !response" class="error-message" role="alert">{{ error }}</p>
      <template v-else-if="response">
        <p v-if="error" class="error-message list-result__refresh-error" role="alert">
          {{ error }}
        </p>
        <header class="result-heading">
          <div>
            <p class="section-kicker">岗位清单</p>
            <h2>
              <strong>{{ response?.page.total ?? 0 }}</strong> 个匹配岗位
            </h2>
          </div>
        </header>
        <ul v-if="response?.items.length" class="job-list">
          <li v-for="job in response.items" :key="job.id">
            <RouterLink :to="{ name: 'job-detail', params: { id: job.id } }" class="job-card">
              <div class="job-card__topline">
                <div class="job-card__title-block">
                  <p class="job-card__locations">{{ job.locations.join(' · ') || '地点未说明' }}</p>
                  <h3>{{ job.title }}</h3>
                </div>
                <div class="job-card__states">
                  <span :class="['status-pill', `status-pill--${job.activity_state}`]">{{
                    formatActivityState(job.activity_state)
                  }}</span>
                  <span v-if="job.is_urgent" class="urgent-pill">紧急</span>
                </div>
              </div>
              <dl class="job-card__facts">
                <div>
                  <dt>主岗位族</dt>
                  <dd class="main-family">{{ job.main_job_family?.label ?? '未分类' }}</dd>
                </div>
                <div>
                  <dt>学历要求</dt>
                  <dd>{{ formatEducation(job.education) }}</dd>
                </div>
                <div>
                  <dt>工作年限</dt>
                  <dd>{{ formatExperience(job.experience) }}</dd>
                </div>
                <div>
                  <dt>首次发现</dt>
                  <dd>{{ formatFirstDiscoveredDate(job.first_discovered_date) }}</dd>
                </div>
              </dl>
              <div v-if="job.other_job_families.length" class="job-card__tag-row">
                <span>其他命中岗位族</span>
                <div>
                  <em
                    v-for="family in job.other_job_families"
                    :key="family.value"
                    class="tag tag--family"
                    >{{ family.label }}</em
                  >
                </div>
              </div>
              <div v-if="job.technology_keywords.length" class="job-card__tag-row">
                <span>技术关键词</span>
                <div>
                  <em
                    v-for="technology in job.technology_keywords"
                    :key="technology.value"
                    class="tag"
                    >{{ technology.label }}</em
                  >
                </div>
              </div>
              <div v-if="job.projects.length" class="job-card__tag-row">
                <span>项目</span>
                <div>
                  <em
                    v-for="project in job.projects"
                    :key="project.value"
                    class="tag tag--project"
                    >{{ project.label }}</em
                  >
                </div>
              </div>
            </RouterLink>
          </li>
        </ul>
        <div v-else class="empty-state">
          <span aria-hidden="true">⌁</span>
          <p>没有符合当前条件的岗位。</p>
          <NButton tertiary @click="resetFilters">清除筛选条件</NButton>
        </div>
        <div
          v-if="response && response.page.total > response.page.page_size"
          class="pagination"
          aria-label="列表分页"
        >
          <NPagination
            :item-count="response.page.total"
            :page="response.page.number"
            :page-size="response.page.page_size"
            :page-sizes="pageSizes"
            :page-slot="7"
            show-quick-jumper
            show-size-picker
            @update:page="changePage"
            @update:pageSize="changePageSize"
          />
        </div>
      </template>
    </section>
  </main>
</template>

<style scoped>
.job-list-page {
  margin: 0 auto;
  max-width: 1240px;
  padding: 64px 32px 88px;
  position: relative;
}

.tide-mark {
  align-items: center;
  display: flex;
  gap: 6px;
  height: 26px;
  margin-bottom: 22px;
}

.tide-mark span {
  background: #007bab;
  border-radius: 99px;
  display: block;
  height: 3px;
  opacity: 0.85;
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

.page-heading {
  align-items: end;
  display: flex;
  gap: 24px;
  justify-content: space-between;
  margin-bottom: 32px;
}

.eyebrow {
  color: #007bab;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

h1 {
  color: #121c22;
  font-size: clamp(2.25rem, 5vw, 3.6rem);
  letter-spacing: -0.055em;
  line-height: 1;
  margin: 10px 0 12px;
}

.heading-copy,
.heading-note,
.filter-panel__heading > p,
.result-heading > p {
  color: #626a6f;
}

.heading-copy {
  font-size: 1.02rem;
}
.heading-note {
  font-size: 0.86rem;
  margin-bottom: 5px;
  white-space: nowrap;
}

.filter-panel,
.list-result {
  background: #fff;
  border-radius: 16px;
  box-shadow:
    0 5px 12px 4px rgba(11, 37, 50, 0.04),
    0 20px 42px -32px rgba(11, 55, 76, 0.32);
}

.filter-panel {
  margin-bottom: 28px;
  padding: 28px;
}

.filter-panel__heading,
.result-heading {
  align-items: baseline;
  display: flex;
  gap: 20px;
  justify-content: space-between;
}

.section-kicker {
  color: #008287;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.1em;
  margin: 0 0 5px;
  text-transform: uppercase;
}

h2,
h3,
p {
  margin-top: 0;
}

.filter-panel h2,
.result-heading h2 {
  color: #121c22;
  font-size: 1.08rem;
  margin-bottom: 0;
}

.filter-panel__heading > p,
.result-heading > p {
  font-size: 0.83rem;
  margin: 0;
}

.title-search {
  margin: 26px 0 18px;
  max-width: 720px;
}

.title-search label,
.filter-field > span {
  color: #406980;
  display: block;
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.025em;
  margin-bottom: 7px;
}

.filter-layout {
  display: grid;
  gap: 16px;
  grid-template-columns: minmax(0, 1.25fr) minmax(330px, 0.75fr);
}

.filter-grid {
  background: #f8fbfd;
  border: 1px solid #e3ebf0;
  border-radius: 14px;
  display: grid;
  gap: 16px 14px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  padding: 18px;
}

.filter-field {
  min-width: 0;
}

.filter-field input {
  background: #f0f8fd;
  border: 1px solid transparent;
  border-radius: 10px;
  color: #2e373c;
  height: 40px;
  padding: 0 12px;
  width: 100%;
}

.filter-field input:focus {
  background: #fff;
  border-color: #007bab;
  outline: 0;
}

.sort-panel {
  background: linear-gradient(145deg, #edf8fb 0%, #f8fcfe 72%);
  border: 1px solid #cfe4eb;
  border-radius: 14px;
  min-height: 100%;
  padding: 18px;
}

.sort-panel__heading {
  align-items: start;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.sort-panel h3 {
  color: #17323d;
  font-size: 1rem;
  margin: 0;
}

.sort-rule-list {
  display: grid;
  gap: 8px;
  list-style: none;
  margin: 0 0 10px;
  padding: 0;
}

.sort-rule {
  align-items: center;
  display: grid;
  gap: 9px;
  grid-template-columns: 24px minmax(0, 1fr) minmax(0, 1fr);
}

.sort-rule > label,
.sort-rule-add > label {
  color: #53717f;
  display: grid;
  font-size: 0.7rem;
  font-weight: 700;
  gap: 5px;
  letter-spacing: 0.02em;
}

.sort-rule__priority {
  align-items: center;
  background: #007bab;
  border-radius: 50%;
  color: #fff;
  display: inline-flex;
  font-size: 0.74rem;
  font-weight: 700;
  height: 22px;
  justify-content: center;
  width: 22px;
}

.sort-rule__actions,
.sort-rule-add {
  align-items: center;
  display: flex;
  gap: 6px;
}

.sort-rule__actions {
  grid-column: 2 / -1;
  justify-content: flex-end;
}

.sort-rule-add {
  align-items: end;
  border-top: 1px solid #d7e8ee;
  margin-top: 14px;
  padding-top: 14px;
}

.sort-rule-add > label {
  flex: 1;
  min-width: 0;
}

.filter-actions {
  align-items: center;
  display: flex;
  gap: 10px;
  margin-top: 24px;
}

.error-message {
  color: #b44953;
  font-size: 0.88rem;
  margin: 0;
}

.list-result {
  overflow: hidden;
  padding: 0;
}

.result-heading {
  background: linear-gradient(90deg, #f0f8fd 0%, #f8fcfe 100%);
  border-bottom: 1px solid #e3ebf0;
  padding: 20px 28px 18px;
}

.result-heading strong {
  color: #007bab;
  font-size: 1.5em;
  letter-spacing: -0.04em;
}

.job-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.job-list li + li {
  border-top: 1px solid #e3ebf0;
}

.job-card {
  color: #121c22;
  display: block;
  padding: 24px 28px 23px;
  text-decoration: none;
  transition:
    background-color 160ms ease,
    transform 160ms ease;
}

.job-list a:hover,
.job-list a:focus-visible {
  background: #f7fbfd;
  color: #121c22;
  outline: 0;
}

.job-card__topline {
  align-items: flex-start;
  display: flex;
  gap: 20px;
  justify-content: space-between;
}
.job-card__locations {
  color: #626a6f;
  font-size: 0.82rem;
  margin-bottom: 4px;
}
.job-card h3 {
  font-size: 1.15rem;
  letter-spacing: -0.018em;
  line-height: 1.35;
  margin-bottom: 0;
}
.job-card__states {
  align-items: center;
  display: flex;
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

.job-card__facts {
  display: grid;
  gap: 14px;
  grid-template-columns: 1.25fr 1.2fr 1.2fr 1fr;
  margin: 19px 0;
}

.job-card__facts div {
  min-width: 0;
}
.job-card__facts dt {
  color: #72818a;
  font-size: 0.73rem;
  font-weight: 600;
  margin-bottom: 4px;
}
.job-card__facts dd {
  color: #2e373c;
  font-size: 0.88rem;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.job-card__facts .main-family {
  color: #007bab;
  font-weight: 700;
}

.job-card__tag-row {
  align-items: flex-start;
  display: flex;
  gap: 12px;
  margin-top: 8px;
}
.job-card__tag-row > span {
  color: #72818a;
  flex: 0 0 96px;
  font-size: 0.73rem;
  font-weight: 600;
  line-height: 28px;
}
.job-card__tag-row > div {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
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

.empty-state {
  align-items: center;
  color: #626a6f;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 66px 24px;
  text-align: center;
}
.empty-state > span {
  color: #008287;
  font-family: Georgia, serif;
  font-size: 2.4rem;
  line-height: 1;
}
.empty-state p {
  margin: 0;
}

.pagination {
  align-items: center;
  border-top: 1px solid #e3ebf0;
  display: flex;
  justify-content: flex-end;
  padding: 18px 28px;
}

@media (max-width: 1080px) {
  .filter-layout {
    grid-template-columns: 1fr;
  }

  .sort-panel {
    min-height: auto;
  }
}

@media (max-width: 620px) {
  .job-list-page {
    padding: 38px 16px 56px;
  }
  .page-heading,
  .filter-panel__heading,
  .result-heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }
  .heading-note {
    display: none;
  }
  .filter-panel {
    padding: 22px 18px;
  }
  .filter-grid {
    grid-template-columns: 1fr;
    padding: 15px;
  }
  .sort-rule {
    align-items: stretch;
    grid-template-columns: 24px minmax(0, 1fr);
  }
  .sort-rule > label:last-of-type {
    grid-column: 2;
  }
  .sort-rule__actions {
    grid-column: 2;
    flex-wrap: wrap;
  }
  .sort-rule-add {
    align-items: stretch;
    flex-direction: column;
  }
  .result-heading,
  .job-card {
    padding-left: 18px;
    padding-right: 18px;
  }
  .job-card__topline {
    gap: 12px;
  }
  .job-card__states {
    flex-direction: column;
    gap: 5px;
  }
  .job-card__facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .job-card__tag-row {
    flex-direction: column;
    gap: 4px;
  }
  .job-card__tag-row > span {
    flex-basis: auto;
    line-height: 1.3;
  }
  .pagination {
    flex-wrap: wrap;
    justify-content: center;
    padding-left: 18px;
    padding-right: 18px;
  }
}
</style>
