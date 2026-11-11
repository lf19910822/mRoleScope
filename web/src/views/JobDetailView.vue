<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { getJobDetail, type JobDetailResponse } from '@/api/jd-api'
import JDDetailReadingLayout from '@/components/JDDetailReadingLayout.vue'
import JobHistoryTimeline from '@/components/JobHistoryTimeline.vue'
import JobDetailSkeleton from '@/components/JobDetailSkeleton.vue'

const route = useRoute()
const router = useRouter()
const response = ref<JobDetailResponse>()
const error = ref('')
const isLoading = ref(false)
let latestDetailRequest = 0

watch(
  () => String(route.params.id ?? ''),
  async (id) => {
    const request = ++latestDetailRequest
    isLoading.value = true
    error.value = ''
    try {
      const nextResponse = await getJobDetail(id)
      if (request !== latestDetailRequest) return
      response.value = nextResponse
    } catch (reason) {
      if (request !== latestDetailRequest) return
      error.value = reason instanceof Error ? reason.message : '请求岗位详情失败'
    } finally {
      if (request === latestDetailRequest) isLoading.value = false
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
    <JobDetailSkeleton v-if="isLoading && !response" />
    <p v-else-if="error && !response" class="error-message" role="alert">{{ error }}</p>
    <template v-else-if="response">
      <p v-if="error" class="error-message job-detail-refresh-error" role="alert">{{ error }}</p>
      <JDDetailReadingLayout :job="response">
        <template #back-action>
          <button
            class="back-button"
            data-testid="job-detail-back"
            type="button"
            @click="returnToList"
          >
            <span aria-hidden="true">←</span>
            <span>返回岗位列表</span>
          </button>
        </template>
      </JDDetailReadingLayout>
      <JobHistoryTimeline :activity-state="response.activity_state" :job-id="response.id" />
    </template>
  </main>
</template>

<style scoped>
.job-detail-page {
  margin: 0 auto;
  max-width: 1040px;
  padding: 52px 32px 88px;
}

.back-button {
  align-items: center;
  background: rgba(255, 255, 255, 0.94);
  border: 1px solid rgba(0, 123, 171, 0.2);
  border-radius: 12px;
  box-shadow: 0 8px 22px rgba(11, 55, 76, 0.12);
  color: #007bab;
  cursor: pointer;
  display: inline-flex;
  font: inherit;
  gap: 7px;
  font-weight: 600;
  padding: 9px 13px;
  width: 100%;
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
  z-index: 20;
}

.back-button:hover {
  background: #ffffff;
  border-color: rgba(0, 123, 171, 0.42);
  box-shadow: 0 10px 26px rgba(11, 55, 76, 0.16);
  transform: translateY(-1px);
}

.back-button:active {
  transform: translateY(0);
}

.error-message {
  color: #b44953;
}

.job-detail-refresh-error {
  margin: 0 32px 12px;
}

@media (max-width: 700px) {
  .job-detail-page {
    padding: 36px 16px 56px;
  }
}
</style>
