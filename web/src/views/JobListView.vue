<script setup lang="ts">
import { NButton, NInput, NPagination, NSelect, type SelectOption } from 'naive-ui'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  getFilterOptions,
  getJobList,
  jobListPath,
  type FilterOptionsResponse,
  type JobListResponse,
} from '@/api/jd-api'
import FilterDatePicker from '@/components/FilterDatePicker.vue'
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
import { presentJobTitle } from '@/lib/job-title-presentation'

defineOptions({ name: 'JobListView' })

const route = useRoute()
const router = useRouter()
const response = ref<JobListResponse>()
const error = ref('')
const isLoading = ref(false)
const focusedQueryCardId = ref<string>()
const mobileFiltersOpen = ref(false)
let latestListRequest = 0
let latestListQuery: string | undefined
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
const queryConsoleSelectMenuProps = { class: 'query-console-select-menu' }
const minimumExperienceOptions: SelectOption[] = Array.from({ length: 9 }, (_, index) => ({
  label: `${index + 1} 年及以上`,
  value: index + 1,
}))

type NamedSelectOption<T extends string> = SelectOption & { label: string; value: T }

const sortKindOptions: NamedSelectOption<SortKind>[] = [
  { value: 'experience', label: '最低工作年限' },
  { value: 'education', label: '学历要求' },
  { value: 'first_discovered', label: '首次发现日期' },
  { value: 'last_modified', label: '最近修改日期' },
  { value: 'urgent', label: '紧急标记' },
  { value: 'activity_state', label: '活跃状态' },
  { value: 'title', label: '岗位标题' },
]

const sortDirectionOptions: Record<SortKind, NamedSelectOption<SortDirection>[]> = {
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
  last_modified: [
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

interface AppliedQueryCondition {
  id: string
  label: string
  remove: () => void
}

function optionLabel(
  options: ReadonlyArray<{ label: string; value: string }>,
  value: string,
): string {
  return options.find((option) => option.value === value)?.label ?? value
}

function copiedListState(state: JDListState): JDListState {
  return {
    ...state,
    activityStates: [...state.activityStates],
    educationLevels: [...state.educationLevels],
    jobFamilies: [...state.jobFamilies],
    locations: [...state.locations],
    projects: [...state.projects],
    sortRules: [...state.sortRules],
    technologyKeywords: [...state.technologyKeywords],
  }
}

function updateAppliedQuery(update: (state: JDListState) => void): void {
  const nextState = copiedListState(listState.value)
  update(nextState)
  nextState.page = 1
  void router.push({ name: 'jobs', query: listStateToRoute(nextState) })
}

function sortRuleLabel(rule: RequirementSort): string {
  const { direction, kind } = sortRuleParts(rule)
  const kindLabel = optionLabel(sortKindOptions, kind)
  const directionLabel = optionLabel(sortDirectionOptions[kind], direction)
  return `${kindLabel}（${directionLabel}）`
}

const appliedQueryConditions = computed<AppliedQueryCondition[]>(() => {
  const state = listState.value
  const conditions: AppliedQueryCondition[] = []

  if (state.title) {
    conditions.push({
      id: 'title',
      label: `标题：${state.title}`,
      remove: () => updateAppliedQuery((next) => (next.title = '')),
    })
  }
  for (const family of state.jobFamilies) {
    const label = optionLabel(filterOptions.value.job_families, family)
    conditions.push({
      id: `job-family-${family}`,
      label: `岗位族：${label}`,
      remove: () =>
        updateAppliedQuery((next) => {
          next.jobFamilies = next.jobFamilies.filter((value) => value !== family)
        }),
    })
  }
  for (const technology of state.technologyKeywords) {
    const label = optionLabel(filterOptions.value.technology_keywords, technology)
    conditions.push({
      id: `technology-${technology}`,
      label: `技术关键词：${label}`,
      remove: () =>
        updateAppliedQuery((next) => {
          next.technologyKeywords = next.technologyKeywords.filter((value) => value !== technology)
        }),
    })
  }
  for (const education of state.educationLevels) {
    const label = optionLabel(filterOptions.value.education_levels, education)
    conditions.push({
      id: `education-${education}`,
      label: `学历要求：${label}`,
      remove: () =>
        updateAppliedQuery((next) => {
          next.educationLevels = next.educationLevels.filter((value) => value !== education)
        }),
    })
  }
  if (state.minimumExperienceYear !== undefined) {
    conditions.push({
      id: 'minimum-experience-year',
      label: `最低工作年限：${state.minimumExperienceYear} 年及以上`,
      remove: () => updateAppliedQuery((next) => (next.minimumExperienceYear = undefined)),
    })
  }
  for (const project of state.projects) {
    const label = optionLabel(filterOptions.value.projects, project)
    conditions.push({
      id: `project-${project}`,
      label: `项目：${label}`,
      remove: () =>
        updateAppliedQuery((next) => {
          next.projects = next.projects.filter((value) => value !== project)
        }),
    })
  }
  for (const location of state.locations) {
    const label = optionLabel(filterOptions.value.locations, location)
    conditions.push({
      id: `location-${location}`,
      label: `工作地点：${label}`,
      remove: () =>
        updateAppliedQuery((next) => {
          next.locations = next.locations.filter((value) => value !== location)
        }),
    })
  }
  if (state.activityStates.length !== 1 || state.activityStates[0] !== 'effective') {
    const activityLabel =
      state.activityStates.length === 2
        ? '全部'
        : optionLabel(filterOptions.value.activity_states, state.activityStates[0] ?? '')
    conditions.push({
      id: 'activity-state',
      label: `活跃状态：${activityLabel}`,
      remove: () => updateAppliedQuery((next) => (next.activityStates = ['effective'])),
    })
  }
  if (state.isUrgent !== undefined) {
    const urgent = String(state.isUrgent)
    const label = optionLabel(filterOptions.value.urgent_options, urgent)
    conditions.push({
      id: 'urgent',
      label: `紧急标记：${label}`,
      remove: () => updateAppliedQuery((next) => (next.isUrgent = undefined)),
    })
  }
  if (state.firstDiscoveredFrom) {
    conditions.push({
      id: 'first-discovered-from',
      label: `首次发现自：${state.firstDiscoveredFrom}`,
      remove: () => updateAppliedQuery((next) => (next.firstDiscoveredFrom = '')),
    })
  }
  if (state.firstDiscoveredUntil) {
    conditions.push({
      id: 'first-discovered-until',
      label: `首次发现至：${state.firstDiscoveredUntil}`,
      remove: () => updateAppliedQuery((next) => (next.firstDiscoveredUntil = '')),
    })
  }
  for (const rule of state.sortRules) {
    const label = sortRuleLabel(rule)
    conditions.push({
      id: `sort-${rule}`,
      label: `排序：${label}`,
      remove: () =>
        updateAppliedQuery((next) => {
          next.sortRules = next.sortRules.filter((value) => value !== rule)
        }),
    })
  }

  return conditions
})

const hasContextualFilters = computed(() => {
  const state = listState.value
  return (
    state.projects.length > 0 ||
    state.locations.length > 0 ||
    state.activityStates.length !== 1 ||
    state.activityStates[0] !== 'effective' ||
    state.isUrgent !== undefined ||
    Boolean(state.firstDiscoveredFrom) ||
    Boolean(state.firstDiscoveredUntil)
  )
})

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
  mobileFiltersOpen.value = false
  void router.push({ name: 'jobs', query: listStateToRoute(stateFromDraft()) })
}

function resetFilters(): void {
  mobileFiltersOpen.value = false
  void router.push({ name: 'jobs', query: listStateToRoute(createDefaultListState()) })
}

function closeMobileFilters(): void {
  mobileFiltersOpen.value = false
}

function focusQueryCard(event: PointerEvent, id: string): void {
  if (event.pointerType !== 'mouse') {
    return
  }
  focusedQueryCardId.value = id
}

function releaseQueryCard(id: string): void {
  if (focusedQueryCardId.value === id) {
    focusedQueryCardId.value = undefined
  }
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
  () => (route.name === 'jobs' ? jobListPath(listState.value) : undefined),
  async (query) => {
    if (query === undefined || query === latestListQuery) return
    latestListQuery = query
    const state = listState.value
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
      latestListQuery = undefined
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
    <header class="page-heading">
      <div>
        <p class="eyebrow">岗位 / 当前清单</p>
        <h1>岗位清单</h1>
        <p class="heading-copy">浏览、筛选并比较当前岗位。</p>
      </div>
      <button
        class="mobile-filter-trigger"
        type="button"
        aria-controls="mobile-filter-sheet"
        :aria-expanded="mobileFiltersOpen"
        data-testid="open-mobile-filters"
        @click="mobileFiltersOpen = true"
      >
        筛选
      </button>
    </header>

    <div class="query-canvas" data-testid="query-canvas">
      <div
        v-if="mobileFiltersOpen"
        class="mobile-filter-backdrop"
        aria-hidden="true"
        @click="closeMobileFilters"
      ></div>
      <div
        :class="['query-console-host', { 'query-console-host--mobile-open': mobileFiltersOpen }]"
        :data-testid="mobileFiltersOpen ? 'mobile-filter-sheet' : undefined"
        :id="mobileFiltersOpen ? 'mobile-filter-sheet' : undefined"
        :role="mobileFiltersOpen ? 'dialog' : undefined"
        :aria-modal="mobileFiltersOpen ? 'true' : undefined"
        aria-label="岗位筛选"
      >
        <section
          class="filter-panel query-console"
          aria-label="岗位筛选"
          data-testid="query-console"
        >
          <div class="filter-panel__heading">
            <div>
              <p class="section-kicker">查询控制台</p>
              <h2>筛选岗位</h2>
            </div>
            <button
              class="mobile-filter-sheet__close"
              type="button"
              aria-label="关闭筛选"
              data-testid="close-mobile-filters"
              @click="closeMobileFilters"
            >
              ×
            </button>
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
                  :menu-props="queryConsoleSelectMenuProps"
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
              <div class="filter-field-pair" data-testid="filter-pair-qualifications">
                <label class="filter-field">
                  <span>学历要求</span>
                  <NSelect
                    v-model:value="draft.educationLevels"
                    :menu-props="queryConsoleSelectMenuProps"
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
                    :menu-props="queryConsoleSelectMenuProps"
                    :options="minimumExperienceOptions"
                    clearable
                    data-testid="minimum-experience-select"
                    placeholder="不限"
                  />
                </label>
              </div>
              <details
                class="filter-disclosure"
                data-testid="advanced-filter-disclosure"
                :open="hasContextualFilters"
              >
                <summary>更多条件</summary>
                <div class="filter-field-pair" data-testid="filter-pair-context">
                  <label class="filter-field">
                    <span>项目</span>
                    <NSelect
                      v-model:value="draft.projects"
                      :menu-props="queryConsoleSelectMenuProps"
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
                      :menu-props="queryConsoleSelectMenuProps"
                      :options="filterOptions.locations"
                      clearable
                      filterable
                      multiple
                      placeholder="搜索或选择工作地点"
                    />
                  </label>
                </div>
                <div class="filter-field-pair" data-testid="filter-pair-activity">
                  <label class="filter-field">
                    <span>活跃状态</span>
                    <NSelect
                      v-model:value="activityMode"
                      :menu-props="queryConsoleSelectMenuProps"
                      :options="filterOptions.activity_states"
                      placeholder="选择活跃状态"
                    />
                  </label>
                  <label class="filter-field">
                    <span>紧急标记</span>
                    <NSelect
                      v-model:value="urgentValue"
                      :menu-props="queryConsoleSelectMenuProps"
                      :options="filterOptions.urgent_options"
                      clearable
                      placeholder="是否紧急招聘"
                    />
                  </label>
                </div>
                <div class="filter-field-pair" data-testid="filter-pair-discovery-date">
                  <label class="filter-field">
                    <span>首次发现日期（起）</span>
                    <FilterDatePicker
                      v-model="draft.firstDiscoveredFrom"
                      aria-label="选择首次发现日期（起）"
                      data-testid="first-discovered-from-picker"
                      placeholder="选择起始日期"
                    />
                  </label>
                  <label class="filter-field">
                    <span>首次发现日期（止）</span>
                    <FilterDatePicker
                      v-model="draft.firstDiscoveredUntil"
                      aria-label="选择首次发现日期（止）"
                      data-testid="first-discovered-until-picker"
                      placeholder="选择结束日期"
                    />
                  </label>
                </div>
              </details>
            </div>

            <details
              class="sort-disclosure"
              data-testid="sort-disclosure"
              :open="listState.sortRules.length > 0"
            >
              <summary>排序</summary>
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
                          :consistent-menu-width="false"
                          :menu-props="queryConsoleSelectMenuProps"
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
                          :consistent-menu-width="false"
                          :menu-props="queryConsoleSelectMenuProps"
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
                      :consistent-menu-width="false"
                      :menu-props="queryConsoleSelectMenuProps"
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
                      :consistent-menu-width="false"
                      :menu-props="queryConsoleSelectMenuProps"
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
            </details>
          </div>
          <div class="filter-actions">
            <NButton type="primary" @click="applyFilters">应用筛选</NButton>
            <NButton tertiary @click="resetFilters">重置</NButton>
            <p v-if="filterOptionsError" class="error-message" role="alert">
              {{ filterOptionsError }}
            </p>
          </div>
        </section>
      </div>

      <section aria-live="polite" class="list-result">
        <JobListSkeleton v-if="isLoading && !response" />
        <p v-else-if="error && !response" class="error-message" role="alert">{{ error }}</p>
        <template v-else-if="response">
          <p v-if="error" class="error-message list-result__refresh-error" role="alert">
            {{ error }}
          </p>
          <header class="result-heading">
            <p class="section-kicker">岗位清单</p>
            <h2>
              <strong>{{ response?.page.total ?? 0 }}</strong> 个匹配岗位
            </h2>
          </header>
          <div
            v-if="appliedQueryConditions.length"
            class="applied-query-summary"
            data-testid="applied-query-summary"
            aria-label="当前筛选条件"
          >
            <span class="applied-query-summary__label">当前条件</span>
            <ul>
              <li v-for="condition in appliedQueryConditions" :key="condition.id">
                <button
                  type="button"
                  :aria-label="`移除${condition.label}`"
                  @click="condition.remove"
                >
                  <span>{{ condition.label }}</span
                  ><i aria-hidden="true">×</i>
                </button>
              </li>
            </ul>
          </div>
          <ul v-if="response?.items.length" class="job-list">
            <li v-for="job in response.items" :key="job.id">
              <RouterLink
                :to="{ name: 'job-detail', params: { id: job.id } }"
                :class="['job-card', { 'job-card--focused': focusedQueryCardId === job.id }]"
                :data-motion-state="focusedQueryCardId === job.id ? 'focused' : 'resting'"
                :data-testid="`query-canvas-${job.id}`"
                @pointerenter="focusQueryCard($event, job.id)"
                @pointerleave="releaseQueryCard(job.id)"
              >
                <div class="job-card__headline">
                  <div class="job-card__title-group">
                    <span class="job-card__title-icon" aria-hidden="true">
                      <svg viewBox="0 0 24 24" fill="none">
                        <path d="M12 6v5M7 16h10M7 16v2M17 16v2" />
                        <rect x="9" y="3" width="6" height="5" rx="1" />
                        <rect x="4" y="17" width="6" height="4" rx="1" />
                        <rect x="14" y="17" width="6" height="4" rx="1" />
                      </svg>
                    </span>
                    <h3>
                      <template
                        v-for="(fragment, fragmentIndex) in presentJobTitle(
                          job.title,
                          job.projects,
                        )"
                        :key="`${fragment.kind}-${fragmentIndex}-${fragment.text}`"
                      >
                        <mark
                          v-if="fragment.kind === 'project'"
                          class="job-card__project-mention"
                          >{{ fragment.text }}</mark
                        >
                        <template v-else>{{ fragment.text }}</template>
                      </template>
                    </h3>
                    <em class="tag tag--family job-card__headline-family">{{
                      job.main_job_family?.label ?? '未分类'
                    }}</em>
                  </div>
                  <div class="job-card__states">
                    <span :class="['status-pill', `status-pill--${job.activity_state}`]">{{
                      formatActivityState(job.activity_state)
                    }}</span>
                    <span :class="['urgent-pill', { 'urgent-pill--active': job.is_urgent }]">{{
                      job.is_urgent ? '紧急' : '常规'
                    }}</span>
                  </div>
                </div>
                <dl
                  :class="[
                    'job-card__facts',
                    { 'job-card__facts--terminal': !job.technology_keywords.length },
                  ]"
                >
                  <div>
                    <dt>工作地区</dt>
                    <dd>{{ job.locations.join(' · ') || '地点未说明' }}</dd>
                  </div>
                  <div>
                    <dt>工作年限</dt>
                    <dd>{{ formatExperience(job.experience) }}</dd>
                  </div>
                  <div>
                    <dt>学历要求</dt>
                    <dd>{{ formatEducation(job.education) }}</dd>
                  </div>
                  <div>
                    <dt>首次发现</dt>
                    <dd>{{ formatFirstDiscoveredDate(job.first_discovered_date) }}</dd>
                  </div>
                  <div>
                    <dt>最近修改日期</dt>
                    <dd>
                      {{
                        job.last_modified_date
                          ? formatFirstDiscoveredDate(job.last_modified_date)
                          : '-'
                      }}
                    </dd>
                  </div>
                </dl>
                <div v-if="job.technology_keywords.length" class="job-card__technologies">
                  <span class="job-card__row-label">技术关键词</span>
                  <div class="job-card__tag-list">
                    <em
                      v-for="technology in job.technology_keywords"
                      :key="technology.value"
                      class="tag"
                      >{{ technology.label }}</em
                    >
                  </div>
                </div>
                <div
                  v-if="job.other_job_families.length"
                  :class="[
                    'job-card__supplementary',
                    {
                      'job-card__supplementary--after-facts': !job.technology_keywords.length,
                    },
                  ]"
                >
                  <div v-if="job.other_job_families.length" class="job-card__tag-row">
                    <span class="job-card__row-label">副岗位族</span>
                    <div class="job-card__tag-list">
                      <em
                        v-for="family in job.other_job_families"
                        :key="family.value"
                        class="tag tag--family"
                        >{{ family.label }}</em
                      >
                    </div>
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
    </div>
  </main>
</template>

<style scoped>
.job-list-page {
  display: block;
  margin: 0 auto;
  max-width: 1240px;
  overflow: visible;
  padding: 32px 32px 88px;
  position: relative;
}

.page-heading {
  align-items: end;
  display: flex;
  gap: 24px;
  justify-content: space-between;
  margin-bottom: 24px;
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
  font-size: clamp(1.65rem, 3vw, 2.1rem);
  letter-spacing: -0.045em;
  line-height: 1.08;
  margin: 6px 0 5px;
}

.heading-copy,
.heading-note,
.filter-panel__heading > p,
.result-heading > p {
  color: #626a6f;
}

.heading-copy {
  font-size: 0.9rem;
  margin-bottom: 0;
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

.filter-field-pair {
  display: grid;
  gap: 14px;
  grid-column: 1 / -1;
  grid-template-columns: repeat(2, minmax(0, 1fr));
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
  .filter-field-pair {
    grid-template-columns: 1fr;
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

/* Query canvas: the desktop workspace keeps the live controls beside the result collection. */
.query-canvas {
  align-items: start;
  display: grid;
  gap: 26px;
  grid-template-columns: minmax(0, 1fr) minmax(306px, 342px);
}

.query-console-host {
  display: contents;
}

.mobile-filter-trigger,
.mobile-filter-sheet__close {
  display: none;
}

.mobile-filter-backdrop {
  display: none;
}

.query-canvas > .list-result {
  background: transparent;
  box-shadow: none;
  grid-column: 1;
  grid-row: 1;
  height: auto;
  overflow: visible;
}

.query-console {
  background: linear-gradient(160deg, #f9fdfe, #eef8fb);
  border: 1px solid #cfe4eb;
  box-shadow: 0 18px 42px -36px rgba(11, 55, 76, 0.5);
  grid-column: 2;
  grid-row: 1;
  height: auto;
  margin: 0;
  max-height: calc(100dvh - 48px);
  overflow-y: auto;
  overscroll-behavior: contain;
  position: sticky;
  scrollbar-gutter: stable;
  padding: 16px 18px 18px;
  top: 24px;
  z-index: 2;
}

.query-console .filter-panel__heading {
  align-items: center;
  border-bottom: 1px solid #cfe4eb;
  padding-bottom: 11px;
}

.query-console .filter-panel__heading > div {
  flex: 1;
}

.query-console .title-search {
  margin: 12px 0;
}

.query-console .filter-layout {
  display: block;
}

.query-console .filter-grid {
  background: transparent;
  border: 0;
  border-radius: 0;
  gap: 10px;
  grid-template-columns: 1fr;
  padding: 0;
}

.query-console .filter-field-pair {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.query-console .filter-disclosure,
.query-console .sort-disclosure {
  border-top: 1px solid #d8e9ee;
  margin-top: 2px;
  padding-top: 9px;
}

.query-console .filter-disclosure {
  grid-column: 1 / -1;
}

.query-console .filter-disclosure summary,
.query-console .sort-disclosure summary {
  color: #406980;
  cursor: pointer;
  font-size: 0.76rem;
  font-weight: 750;
  letter-spacing: 0.025em;
  list-style: none;
  user-select: none;
}

.query-console .filter-disclosure summary::-webkit-details-marker,
.query-console .sort-disclosure summary::-webkit-details-marker {
  display: none;
}

.query-console .filter-disclosure summary::after,
.query-console .sort-disclosure summary::after {
  color: #5a8a99;
  content: '+';
  float: right;
  font-size: 1rem;
  line-height: 0.8;
}

.query-console .filter-disclosure[open] summary::after,
.query-console .sort-disclosure[open] summary::after {
  content: '−';
}

.query-console .filter-disclosure[open] .filter-field-pair {
  margin-top: 11px;
}

.query-console .filter-disclosure[open] .filter-field-pair + .filter-field-pair {
  margin-top: 10px;
}

.query-console .filter-field > span {
  font-size: 0.72rem;
  margin-bottom: 5px;
}

.query-console .sort-panel {
  margin-top: 12px;
  min-height: 0;
  padding: 12px;
}

.query-console .filter-actions {
  background: linear-gradient(180deg, rgba(249, 253, 254, 0.78), #f4fafc 32%);
  bottom: 0;
  border-top: 1px solid #cfe4eb;
  margin-top: 14px;
  padding: 12px 0 2px;
  position: sticky;
  z-index: 3;
}

.query-console .filter-actions .n-button:first-child {
  flex: 1;
}

:global(.query-console-select-menu .n-virtual-list) {
  overscroll-behavior: contain;
}

.query-canvas .result-heading {
  background: transparent;
  border: 0;
  border-bottom: 1px solid #d7e9ef;
  border-radius: 0;
  gap: 10px;
  justify-content: flex-start;
  margin-bottom: 16px;
  padding: 0 0 12px;
}

.query-canvas .result-heading::before {
  align-self: stretch;
  background: linear-gradient(180deg, #007bab, #39b5a3);
  border-radius: 99px;
  content: '';
  display: block;
  flex: 0 0 3px;
  min-height: 32px;
}

.query-canvas .result-heading .section-kicker {
  margin: 0;
  white-space: nowrap;
}

.query-canvas .result-heading h2 {
  font-size: 1rem;
  margin: 0;
}

.query-canvas .result-heading strong {
  font-size: 1.35em;
}

.query-canvas .applied-query-summary {
  align-items: flex-start;
  border-bottom: 1px solid #d7e9ef;
  display: flex;
  gap: 10px;
  margin: -6px 0 16px;
  padding: 0 0 12px;
}

.applied-query-summary__label {
  color: #5d7d89;
  flex: 0 0 auto;
  font-size: 0.72rem;
  font-weight: 750;
  letter-spacing: 0.05em;
  line-height: 28px;
}

.applied-query-summary ul {
  display: flex;
  flex: 1;
  flex-wrap: wrap;
  gap: 6px;
  list-style: none;
  margin: 0;
  padding: 0;
}

.applied-query-summary button {
  align-items: center;
  background: #edf8fb;
  border: 1px solid #d3eaf0;
  border-radius: 999px;
  color: #245d70;
  cursor: pointer;
  display: inline-flex;
  font: inherit;
  font-size: 0.75rem;
  font-weight: 650;
  gap: 6px;
  line-height: 1.1;
  padding: 6px 8px 6px 10px;
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    color 160ms ease;
}

.applied-query-summary button:hover {
  background: #dff3f7;
  border-color: #99d5e2;
  color: #005d80;
}

.applied-query-summary button i {
  color: #4d8494;
  font-size: 1rem;
  font-style: normal;
  font-weight: 500;
  line-height: 0.7;
}

.query-canvas .job-list {
  display: grid;
  gap: 14px;
  grid-template-columns: 1fr;
}

.query-canvas .job-list li,
.query-canvas .job-list li + li {
  border: 0;
  min-width: 0;
  position: relative;
}

.query-canvas .job-card {
  background: rgba(255, 255, 255, 0.94);
  border: 1px solid #d7e9ef;
  border-radius: 16px;
  box-shadow: 0 12px 28px -27px rgba(0, 84, 117, 0.42);
  min-width: 0;
  padding: 14px 18px;
  position: relative;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    transform 360ms cubic-bezier(0.22, 1, 0.36, 1);
}

.query-canvas .job-card:hover,
.query-canvas .job-card:focus-visible {
  background: #fff;
  border-color: #9cd7e5;
  color: #121c22;
  outline: 0;
}

.query-canvas .job-card--focused {
  box-shadow: 0 32px 55px -30px rgba(0, 84, 117, 0.58);
  text-rendering: geometricPrecision;
  transform: scale(1.15);
  z-index: 10;
}

.query-canvas .job-card__headline,
.query-canvas .job-card__title-group {
  align-items: center;
  display: flex;
}

.query-canvas .job-card__headline {
  gap: 10px;
  justify-content: space-between;
}

.query-canvas .job-card__title-group {
  flex: 1;
  gap: 9px;
  min-width: 0;
  flex-wrap: wrap;
}

.query-canvas .job-card__title-icon {
  align-items: center;
  background: #e7f7f7;
  border-radius: 9px;
  color: #008287;
  display: inline-flex;
  flex: 0 0 auto;
  height: 30px;
  justify-content: center;
  width: 30px;
}

.query-canvas .job-card__title-icon svg {
  height: 17px;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.5;
  width: 17px;
}

.query-canvas .job-card__states {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  gap: 5px;
  justify-content: flex-end;
}

.query-canvas .status-pill,
.query-canvas .urgent-pill {
  border-radius: 6px;
  font-size: 0.7rem;
  padding: 5px 6px;
}

.query-canvas .urgent-pill {
  background: #eef4f7;
  color: #607c8a;
}

.query-canvas .urgent-pill--active {
  background: #fff0e6;
  color: #aa642a;
}

.query-canvas .job-card h3 {
  color: #17323d;
  flex: 0 1 auto;
  font-size: 1.02rem;
  letter-spacing: -0.025em;
  line-height: 1.4;
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}

.query-canvas .job-card__headline-family {
  align-self: center;
  flex: 0 0 auto;
}

.query-canvas .job-card__project-mention {
  background: #fff35c;
  border-radius: 4px;
  box-decoration-break: clone;
  color: #302900;
  font: inherit;
  font-weight: 800;
  padding: 0 0.18em;
  -webkit-box-decoration-break: clone;
}

.query-canvas .job-card__facts {
  border-bottom: 1px solid #edf3f5;
  border-top: 1px solid #e7f0f3;
  display: flex;
  flex-wrap: wrap;
  gap: 7px 14px;
  margin: 12px 0 0;
  padding: 9px 0;
}

.query-canvas .job-card__facts--terminal {
  border-bottom: 0;
  padding-bottom: 0;
}

.query-canvas .job-card__facts > div {
  align-items: baseline;
  display: flex;
  flex: 0 1 auto;
  gap: 5px;
  max-width: 100%;
  min-width: 0;
}

.query-canvas .job-card__facts dt {
  color: #87959c;
  font-size: 0.74rem;
  font-weight: 600;
  margin: 0;
  white-space: nowrap;
}

.query-canvas .job-card__facts dd {
  color: #3f535e;
  font-size: 0.84rem;
  line-height: 1.35;
  margin: 0;
  min-width: 0;
  overflow: visible;
  overflow-wrap: anywhere;
  text-overflow: clip;
  white-space: normal;
}

.query-canvas .job-card__technologies,
.query-canvas .job-card__tag-row {
  gap: 8px;
  margin-top: 9px;
}

.query-canvas .job-card__technologies {
  align-items: flex-start;
  display: flex;
}

.query-canvas .job-card__row-label {
  color: #72818a;
  flex: 0 0 72px;
  font-size: 0.74rem;
  font-weight: 600;
  line-height: 22px;
}

.query-canvas .job-card__tag-list {
  display: flex;
  flex: 1;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.query-canvas .job-card__supplementary {
  border-top: 1px solid #edf3f5;
  margin-top: 10px;
  padding-top: 8px;
}

.query-canvas .job-card__supplementary--after-facts {
  margin-top: 0;
}

.query-canvas .job-card__tag-row {
  align-items: flex-start;
  display: flex;
}

.query-canvas .tag {
  font-size: 0.74rem;
  padding: 4px 6px;
}

.query-canvas .empty-state,
.query-canvas .pagination {
  background: #fff;
  border: 1px solid #d7e9ef;
  border-radius: 15px;
  margin-top: 14px;
}

.query-canvas .pagination {
  border-top-color: #d7e9ef;
}

@media (max-width: 980px) {
  .query-canvas {
    grid-template-columns: minmax(0, 1fr) minmax(286px, 318px);
    gap: 18px;
  }

  .query-console {
    padding-left: 17px;
    padding-right: 17px;
  }
}

@media (max-width: 760px) {
  .job-list-page {
    display: block;
    overflow: visible;
    padding: 38px 16px 56px;
  }

  .query-canvas {
    display: flex;
    flex-direction: column;
  }

  .query-canvas > .list-result {
    width: 100%;
  }

  .query-canvas > .list-result {
    order: 1;
  }

  .query-console {
    display: none;
  }

  .mobile-filter-trigger {
    align-items: center;
    background: #007bab;
    border: 1px solid #00658e;
    border-radius: 10px;
    box-shadow: 0 8px 18px -12px rgba(0, 84, 117, 0.65);
    color: #fff;
    cursor: pointer;
    display: inline-flex;
    font: inherit;
    font-size: 0.82rem;
    font-weight: 750;
    justify-content: center;
    min-height: 38px;
    padding: 0 14px;
  }

  .mobile-filter-backdrop {
    background: rgba(7, 29, 40, 0.38);
    display: block;
    inset: 0;
    position: fixed;
    z-index: 40;
  }

  .query-console-host--mobile-open {
    background: #f6fbfd;
    border-radius: 20px 20px 0 0;
    bottom: 0;
    box-shadow: 0 -22px 52px -28px rgba(5, 42, 58, 0.5);
    display: block;
    left: 0;
    max-height: min(86dvh, 780px);
    overflow: hidden;
    position: fixed;
    right: 0;
    z-index: 41;
  }

  .query-console-host--mobile-open .query-console {
    border: 0;
    border-radius: 0;
    display: block;
    height: auto;
    max-height: min(86dvh, 780px);
    overflow-y: auto;
    padding: 18px 16px calc(22px + env(safe-area-inset-bottom));
    position: static;
    width: 100%;
  }

  .mobile-filter-sheet__close {
    align-items: center;
    background: #e3f1f5;
    border: 0;
    border-radius: 50%;
    color: #276275;
    cursor: pointer;
    display: inline-flex;
    flex: 0 0 auto;
    font-size: 1.35rem;
    height: 32px;
    justify-content: center;
    line-height: 1;
    padding: 0;
    width: 32px;
  }

  .query-console .filter-field-pair {
    grid-template-columns: 1fr;
  }

  .query-canvas > .list-result {
    overflow: visible;
    scrollbar-gutter: auto;
  }

  .query-canvas .job-list {
    grid-template-columns: 1fr;
  }

  .query-canvas .result-heading {
    align-items: center;
    flex-direction: row;
  }

  .query-canvas .job-card__headline {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .query-canvas .job-card__states {
    margin-left: auto;
    order: 1;
  }

  .query-canvas .job-card__title-group {
    flex-basis: 100%;
    order: 2;
  }

  .query-canvas .job-card__technologies,
  .query-canvas .job-card__tag-row {
    flex-direction: column;
    gap: 4px;
  }

  .query-canvas .applied-query-summary {
    align-items: flex-start;
    flex-direction: column;
    gap: 4px;
  }

  .applied-query-summary__label {
    line-height: 1.3;
  }

  .query-canvas .job-card__row-label {
    flex-basis: auto;
    line-height: 1.3;
  }

  .query-canvas .pagination {
    padding-left: 12px;
    padding-right: 12px;
  }

  .query-canvas .pagination :deep(.n-pagination) {
    flex-wrap: wrap;
    justify-content: center;
  }
}

@media (prefers-color-scheme: dark) {
  .query-canvas .job-card {
    background: #17262d;
    border-color: #35515e;
    box-shadow: 0 12px 28px -27px rgba(0, 0, 0, 0.7);
  }

  .query-canvas .job-card:hover,
  .query-canvas .job-card:focus-visible {
    background: #1d3038;
    border-color: #5c97aa;
    color: #edf7fa;
  }

  .query-canvas .job-card h3 {
    color: #edf7fa;
  }

  .query-canvas .job-card__facts {
    border-bottom-color: #29414c;
    border-top-color: #35515e;
  }

  .query-canvas .job-card__facts dt,
  .query-canvas .job-card__row-label {
    color: #9aafb8;
  }

  .query-canvas .job-card__facts dd {
    color: #d5e5ea;
  }

  .query-canvas .job-card__project-mention {
    background: #ffe45c;
    color: #2e2500;
  }

  .query-canvas .job-card__supplementary {
    border-top-color: #29414c;
  }

  .query-canvas .tag {
    background: #24414e;
    color: #c4e4ed;
  }
}

@media (min-width: 1280px) {
  .job-list-page {
    margin-left: 0;
    margin-right: auto;
    max-width: 1280px;
    width: calc(100% - 460px);
  }

  .query-canvas {
    grid-template-columns: minmax(0, 1fr);
  }

  .query-console {
    border-radius: 0;
    height: 100dvh;
    max-height: none;
    position: fixed;
    right: 0;
    top: 0;
    width: 428px;
    z-index: 20;
  }
}

:global(.app-shell[data-color-scheme='dark'] .job-list-page) {
  color: #c6dce1;
}

:global(.app-shell[data-color-scheme='dark'] h1),
:global(.app-shell[data-color-scheme='dark'] .filter-panel h2),
:global(.app-shell[data-color-scheme='dark'] .result-heading h2) {
  color: #ecf6f7;
}

:global(.app-shell[data-color-scheme='dark'] .heading-copy),
:global(.app-shell[data-color-scheme='dark'] .filter-panel__heading > p),
:global(.app-shell[data-color-scheme='dark'] .result-heading > p) {
  color: #9bb8c0;
}

:global(.app-shell[data-color-scheme='dark'] .title-search label),
:global(.app-shell[data-color-scheme='dark'] .filter-field > span),
:global(.app-shell[data-color-scheme='dark'] .query-console .filter-disclosure summary),
:global(.app-shell[data-color-scheme='dark'] .query-console .sort-disclosure summary) {
  color: #b5d7df;
}

:global(.app-shell[data-color-scheme='dark'] .query-console) {
  background: linear-gradient(160deg, #15303a, #11262f);
  border-color: #31505c;
  box-shadow: 0 18px 42px -32px rgba(0, 0, 0, 0.7);
}

:global(.app-shell[data-color-scheme='dark'] .query-console .filter-panel__heading),
:global(.app-shell[data-color-scheme='dark'] .query-console .filter-disclosure),
:global(.app-shell[data-color-scheme='dark'] .query-console .sort-disclosure),
:global(.app-shell[data-color-scheme='dark'] .query-console .filter-actions) {
  border-color: #31505c;
}

:global(.app-shell[data-color-scheme='dark'] .query-console .filter-actions) {
  background: linear-gradient(180deg, rgba(17, 38, 47, 0.76), #11262f 32%);
}

:global(.app-shell[data-color-scheme='dark'] .query-console .filter-disclosure summary::after),
:global(.app-shell[data-color-scheme='dark'] .query-console .sort-disclosure summary::after) {
  color: #73c9dd;
}

:global(.app-shell[data-color-scheme='dark'] .filter-field input) {
  background: #19323c;
  color: #ecf6f7;
}

:global(.app-shell[data-color-scheme='dark'] .filter-field input:focus) {
  background: #203d48;
  border-color: #39b5a3;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .result-heading),
:global(.app-shell[data-color-scheme='dark'] .query-canvas .applied-query-summary) {
  border-bottom-color: #31505c;
}

:global(.app-shell[data-color-scheme='dark'] .applied-query-summary__label) {
  color: #aac8d0;
}

:global(.app-shell[data-color-scheme='dark'] .applied-query-summary button) {
  background: #1a3b47;
  border-color: #315d6d;
  color: #c7e8ee;
}

:global(.app-shell[data-color-scheme='dark'] .applied-query-summary button:hover) {
  background: #214a56;
  border-color: #5aaec0;
  color: #f0fbfc;
}

:global(.app-shell[data-color-scheme='dark'] .applied-query-summary button i) {
  color: #91cbd7;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .job-card),
:global(.app-shell[data-color-scheme='dark'] .query-canvas .empty-state),
:global(.app-shell[data-color-scheme='dark'] .query-canvas .pagination) {
  background: #17303a;
  border-color: #31505c;
  box-shadow: 0 12px 28px -27px rgba(0, 0, 0, 0.7);
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .job-card:hover),
:global(.app-shell[data-color-scheme='dark'] .query-canvas .job-card:focus-visible) {
  background: #1d3944;
  border-color: #5cabbc;
  color: #ecf6f7;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .job-card h3) {
  color: #ecf6f7;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .job-card__title-icon) {
  background: #1a4147;
  color: #6ad0c0;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .job-card__facts),
:global(.app-shell[data-color-scheme='dark'] .query-canvas .job-card__supplementary) {
  border-color: #294752;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .job-card__facts dt),
:global(.app-shell[data-color-scheme='dark'] .query-canvas .job-card__row-label) {
  color: #9bb8c0;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .job-card__facts dd) {
  color: #d5e8eb;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .tag) {
  background: #214550;
  color: #c7e8ee;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .tag--family) {
  background: #1b4b4c;
  color: #a9e5d7;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .urgent-pill) {
  background: #263f4a;
  color: #b9d1d8;
}

:global(.app-shell[data-color-scheme='dark'] .query-canvas .urgent-pill--active) {
  background: #563d2c;
  color: #ffd2a1;
}

:global(.app-shell[data-color-scheme='dark'] .query-console-host--mobile-open) {
  background: #11262f;
}

:global(.app-shell[data-color-scheme='dark'] .mobile-filter-sheet__close) {
  background: #244954;
  color: #c7e8ee;
}
</style>
